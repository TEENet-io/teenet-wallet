package handler

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// RateLimiter implements a per-key sliding window rate limiter.
// Each key tracks the timestamps of recent requests; entries older than the
// window are discarded on every check, so the limit is a true sliding window
// (not a fixed-bucket reset).
type RateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
	limit   int
	window  time.Duration
	cancel  context.CancelFunc
}

// NewRateLimiter creates a limiter that allows at most `limit` requests per `window`.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	rl := &RateLimiter{
		windows: make(map[string][]time.Time),
		limit:   limit,
		window:  window,
		cancel:  cancel,
	}
	go rl.cleanup(ctx)
	return rl
}

// Stop cancels the background cleanup goroutine.
func (rl *RateLimiter) Stop() { rl.cancel() }

// Allow returns true if the key is within the rate limit and records the request.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-rl.window)

	// Slide the window: discard timestamps older than the cutoff.
	prev := rl.windows[key]
	valid := prev[:0]
	for _, t := range prev {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= rl.limit {
		rl.windows[key] = valid
		return false
	}
	rl.windows[key] = append(valid, now)
	return true
}

// cleanup runs in the background and evicts keys with no recent requests,
// preventing unbounded memory growth when many unique API keys have been used.
func (rl *RateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-rl.window)
			for key, times := range rl.windows {
				allOld := true
				for _, t := range times {
					if t.After(cutoff) {
						allOld = false
						break
					}
				}
				if allOld {
					delete(rl.windows, key)
				}
			}
			rl.mu.Unlock()
		}
	}
}

// rateLimitEntry tracks the timestamps of recent requests for a single key,
// used by IPRateLimiter.
type rateLimitEntry struct {
	times []time.Time
}

// IPRateLimiter implements a per-IP sliding window rate limiter for unauthenticated
// public endpoints. It is independent of RateLimiter (which is keyed on API keys).
type IPRateLimiter struct {
	mu     sync.Mutex
	limits map[string]*rateLimitEntry
	max    int
	window time.Duration
	cancel context.CancelFunc
}

// NewIPRateLimiter creates a limiter that allows at most `max` requests per `window`
// from a single IP address.
func NewIPRateLimiter(max int, window time.Duration) *IPRateLimiter {
	ctx, cancel := context.WithCancel(context.Background())
	l := &IPRateLimiter{
		limits: make(map[string]*rateLimitEntry),
		max:    max,
		window: window,
		cancel: cancel,
	}
	go l.cleanup(ctx)
	return l
}

// Stop cancels the background cleanup goroutine.
func (l *IPRateLimiter) Stop() { l.cancel() }

// Exceeded returns true if the IP has exceeded the rate limit, and records the
// request if it is still within the limit.
func (l *IPRateLimiter) Exceeded(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-l.window)

	entry, ok := l.limits[ip]
	if !ok {
		entry = &rateLimitEntry{}
		l.limits[ip] = entry
	}

	// Slide the window: discard timestamps older than the cutoff.
	valid := entry.times[:0]
	for _, t := range entry.times {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}

	if len(valid) >= l.max {
		entry.times = valid
		return true
	}
	entry.times = append(valid, now)
	return false
}

// cleanup runs in the background and evicts IP entries with no recent requests,
// preventing unbounded memory growth from many unique source IPs.
func (l *IPRateLimiter) cleanup(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			l.mu.Lock()
			cutoff := time.Now().Add(-l.window)
			for ip, entry := range l.limits {
				allOld := true
				for _, t := range entry.times {
					if t.After(cutoff) {
						allOld = false
						break
					}
				}
				if allOld {
					delete(l.limits, ip)
				}
			}
			l.mu.Unlock()
		}
	}
}

// IPRateLimitMiddleware applies IP-based rate limiting for public (unauthenticated)
// endpoints. On limit exceeded, responds with HTTP 429 and a Retry-After header.
func IPRateLimitMiddleware(limiter *IPRateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := c.ClientIP()
		if limiter.Exceeded(ip) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded — too many requests from this IP, try again later",
			})
			return
		}
		c.Next()
	}
}

// UserRateLimitMiddleware applies rate limiting to all authenticated requests (API key and Passkey).
// Use this for expensive operations like wallet creation (DKG).
func UserRateLimitMiddleware(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := mustUserID(c)
		if c.IsAborted() {
			return
		}
		key := fmt.Sprintf("user:%d", userID)
		if !rl.Allow(key) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded, try again later",
			})
			return
		}
		c.Next()
	}
}

// APIKeyRateLimitMiddleware applies rate limiting to API Key authenticated requests only.
// Passkey sessions (human-operated) are not limited.
// On limit exceeded, responds with HTTP 429 and a Retry-After header.
func APIKeyRateLimitMiddleware(rl *RateLimiter) gin.HandlerFunc {
	return func(c *gin.Context) {
		mode, _ := c.Get("authMode")
		if mode != "apikey" {
			c.Next()
			return
		}

		userID := mustUserID(c)
		if c.IsAborted() {
			return
		}
		key := fmt.Sprintf("apikey:%d", userID)
		if !rl.Allow(key) {
			c.Header("Retry-After", "60")
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"error": "rate limit exceeded — API key requests are limited, try again later",
			})
			return
		}
		c.Next()
	}
}
