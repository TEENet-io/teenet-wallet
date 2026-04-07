package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
)

// SSEEvent is a single Server-Sent Event to be pushed to a client.
type SSEEvent struct {
	Type string
	Data interface{}
}

// MarshalSSE formats the event as an SSE wire-protocol message:
//
//	event: <type>\ndata: <json>\n\n
func (e SSEEvent) MarshalSSE() []byte {
	dataBytes, err := json.Marshal(e.Data)
	if err != nil {
		dataBytes = []byte(`{}`)
	}
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", e.Type, dataBytes))
}

// SSEHub manages per-user subscriber channels for real-time event broadcasting.
type SSEHub struct {
	mu          sync.RWMutex
	subscribers map[uint][]chan SSEEvent
}

// NewSSEHub returns a new, ready-to-use SSEHub.
func NewSSEHub() *SSEHub {
	return &SSEHub{
		subscribers: make(map[uint][]chan SSEEvent),
	}
}

const maxSSEConnsPerUser = 5

// Subscribe registers a new subscriber for userID and returns a buffered channel.
// Returns nil if the user has reached the maximum number of SSE connections.
func (h *SSEHub) Subscribe(userID uint) chan SSEEvent {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.subscribers[userID]) >= maxSSEConnsPerUser {
		slog.Warn("SSE connection limit reached", "userID", userID, "max", maxSSEConnsPerUser)
		return nil
	}
	ch := make(chan SSEEvent, 16)
	h.subscribers[userID] = append(h.subscribers[userID], ch)
	return ch
}

// Unsubscribe removes ch from userID's subscriber list and closes the channel.
// Safe to call with a nil channel (e.g. when Subscribe returned nil due to limit).
func (h *SSEHub) Unsubscribe(userID uint, ch chan SSEEvent) {
	if ch == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()

	channels := h.subscribers[userID]
	updated := make([]chan SSEEvent, 0, len(channels))
	for _, c := range channels {
		if c != ch {
			updated = append(updated, c)
		}
	}
	if len(updated) == 0 {
		delete(h.subscribers, userID)
	} else {
		h.subscribers[userID] = updated
	}
	close(ch)
}

// Stop closes all subscriber channels.
func (h *SSEHub) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for userID, channels := range h.subscribers {
		for _, ch := range channels {
			close(ch)
		}
		delete(h.subscribers, userID)
	}
}

// Broadcast sends evt to all subscribers for userID. The send is non-blocking;
// channels whose buffers are full are skipped rather than blocking the caller.
func (h *SSEHub) Broadcast(userID uint, evt SSEEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, ch := range h.subscribers[userID] {
		select {
		case ch <- evt:
		default:
			slog.Warn("SSE event dropped: subscriber buffer full", "userID", userID, "eventType", evt.Type)
		}
	}
}
