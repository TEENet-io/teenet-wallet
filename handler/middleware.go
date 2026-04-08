// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/model"
)

// sessionEntry holds a passkey session token and its expiry.
type sessionEntry struct {
	userID    uint
	csrfToken string
	expiresAt time.Time
}

// SessionStore is an in-memory store for passkey sessions.
type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]*sessionEntry
	cancel   context.CancelFunc
}

// NewSessionStore creates a new session store with background cleanup.
func NewSessionStore() *SessionStore {
	ctx, cancel := context.WithCancel(context.Background())
	s := &SessionStore{sessions: make(map[string]*sessionEntry), cancel: cancel}
	go s.cleanup(ctx)
	return s
}

// Stop cancels the background cleanup goroutine.
func (s *SessionStore) Stop() { s.cancel() }

func (s *SessionStore) Set(token string, userID uint, ttl time.Duration) string {
	csrfToken := randomHex(32)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = &sessionEntry{userID: userID, csrfToken: csrfToken, expiresAt: time.Now().Add(ttl)}
	return csrfToken
}

func (s *SessionStore) Get(token string) (uint, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.sessions[token]
	if !ok || time.Now().After(entry.expiresAt) {
		return 0, false
	}
	return entry.userID, true
}

// GetCSRF returns the CSRF token associated with a session token.
func (s *SessionStore) GetCSRF(token string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.sessions[token]
	if !ok {
		return ""
	}
	return entry.csrfToken
}

func (s *SessionStore) Delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, token)
}

func (s *SessionStore) cleanup(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := time.Now()
			s.mu.Lock()
			for k, v := range s.sessions {
				if now.After(v.expiresAt) {
					delete(s.sessions, k)
				}
			}
			s.mu.Unlock()
		}
	}
}

// AuthMiddleware authenticates requests via API Key (ocw_...) or Passkey session (ps_...).
// Sets "userID" and "authMode" in the context.
func AuthMiddleware(db *gorm.DB, sessions *SessionStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractBearer(c)
		if token == "" {
			slog.Warn("missing Authorization header", "method", c.Request.Method, "path", c.Request.URL.Path, "ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "authorization required"})
			return
		}

		if strings.HasPrefix(token, "ps_") {
			// Passkey session auth
			userID, ok := sessions.Get(token)
			if !ok {
				slog.Warn("invalid or expired session", "method", c.Request.Method, "path", c.Request.URL.Path, "ip", c.ClientIP())
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired session"})
				return
			}
			c.Set("userID", userID)
			c.Set("authMode", "passkey")
			c.Set("sessionToken", token)
			c.Set("csrfToken", sessions.GetCSRF(token))
			c.Next()
			return
		}

		// API Key auth (ocw_... prefix)
		prefix := ""
		if len(token) >= 12 {
			prefix = token[:12]
		}
		var apiKey model.APIKey
		if prefix == "" || db.Where("prefix = ?", prefix).First(&apiKey).Error != nil {
			safePrefix := "****"
			if len(token) >= 12 {
				safePrefix = token[:4] + "****"
			}
			slog.Warn("invalid API key", "prefix", safePrefix, "method", c.Request.Method, "path", c.Request.URL.Path, "ip", c.ClientIP())
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
			return
		}
		hmacHash := hashAPIKeyWithSalt(token, apiKey.KeySalt)
		if subtle.ConstantTimeCompare([]byte(hmacHash), []byte(apiKey.KeyHash)) != 1 {
			// Fall back to legacy SHA-256 hash for keys created before the HMAC migration.
			legacyHash := hashAPIKeyLegacy(token, apiKey.KeySalt)
			if subtle.ConstantTimeCompare([]byte(legacyHash), []byte(apiKey.KeyHash)) != 1 {
				safePrefix := token[:4] + "****"
				slog.Warn("invalid API key", "prefix", safePrefix, "method", c.Request.Method, "path", c.Request.URL.Path, "ip", c.ClientIP())
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid API key"})
				return
			}
			// Auto-migrate: re-hash with HMAC so future lookups use the new algorithm.
			db.Model(&apiKey).Update("key_hash", hmacHash)
			slog.Info("API key auto-migrated to HMAC", "prefix", apiKey.Prefix)
		}
		c.Set("userID", apiKey.UserID)
		c.Set("authMode", "apikey")
		c.Set("apiKeyPrefix", apiKey.Prefix)
		c.Set("apiKeyLabel", apiKey.Label)
		c.Next()
	}
}

// CSRFMiddleware validates the X-CSRF-Token header for state-changing requests
// authenticated via Passkey sessions. API key auth is exempt (no browser session).
func CSRFMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString("authMode") != "passkey" {
			c.Next()
			return // API key auth doesn't need CSRF
		}
		if c.Request.Method == "GET" || c.Request.Method == "HEAD" || c.Request.Method == "OPTIONS" {
			c.Next()
			return
		}
		token := c.GetHeader("X-CSRF-Token")
		expected := c.GetString("csrfToken")
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(expected)) != 1 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "invalid CSRF token"})
			return
		}
		c.Next()
	}
}

// PasskeyOnlyMiddleware rejects non-passkey requests (used for approval actions).
func PasskeyOnlyMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		mode, _ := c.Get("authMode")
		if mode != "passkey" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "this action requires passkey authentication",
			})
			return
		}
		c.Next()
	}
}

func extractBearer(c *gin.Context) string {
	auth := c.GetHeader("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer "))
	}
	return ""
}

func hashAPIKeyWithSalt(key, salt string) string {
	mac := hmac.New(sha256.New, []byte(salt))
	mac.Write([]byte(key))
	return hex.EncodeToString(mac.Sum(nil))
}

// hashAPIKeyLegacy computes the old SHA-256(salt || key) hash for backward compatibility.
func hashAPIKeyLegacy(key, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(key))
	return hex.EncodeToString(h.Sum(nil))
}

// requestBaseURL returns the base URL of the current request, honoring
// X-Forwarded-Proto / X-Forwarded-Host / X-App-Instance-ID headers set by reverse proxies (e.g. UMS).
// When behind UMS, requests arrive under /instance/{id}/, so the approval link must include that path.
// Falls back to the configured baseURL if headers are absent.
func requestBaseURL(c *gin.Context, fallback string) string {
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" {
		host = c.GetHeader("X-Real-Host")
	}
	if host == "" {
		host = c.Request.Host
	}
	if host == "" {
		return fallback
	}
	scheme := c.GetHeader("X-Forwarded-Proto")
	if scheme == "" {
		if c.Request.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	base := scheme + "://" + strings.TrimRight(host, "/")
	// UMS reverse-proxy sets X-App-Instance-ID; the app is mounted at /instance/{id}/.
	if instanceID := c.GetHeader("X-App-Instance-ID"); instanceID != "" {
		base = base + "/instance/" + instanceID
	}
	return base
}
