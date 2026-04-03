package handler

import (
	"log/slog"
	"time"

	"github.com/gin-gonic/gin"
)

// SSEHandler streams server-sent events to authenticated clients.
type SSEHandler struct {
	hub *SSEHub
}

// NewSSEHandler returns a new SSEHandler backed by the given hub.
func NewSSEHandler(hub *SSEHub) *SSEHandler {
	return &SSEHandler{hub: hub}
}

// Stream handles GET /api/events/stream.
// Authenticated (API Key or Passkey session). Events scoped to the authenticated user.
func (h *SSEHandler) Stream(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	ch := h.hub.Subscribe(userID)
	defer h.hub.Unsubscribe(userID, ch)

	slog.Info("SSE client connected", "userID", userID)

	// Send initial connected comment and flush.
	if _, err := c.Writer.WriteString(": connected\n\n"); err != nil {
		slog.Info("SSE write failed, closing", "userID", userID)
		return
	}
	c.Writer.Flush()

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			slog.Info("SSE client disconnected", "userID", userID)
			return
		case evt, ok := <-ch:
			if !ok {
				slog.Info("SSE channel closed", "userID", userID)
				return
			}
			if _, err := c.Writer.Write(evt.MarshalSSE()); err != nil {
				slog.Info("SSE write failed, closing", "userID", userID)
				return
			}
			c.Writer.Flush()
		case <-heartbeat.C:
			if _, err := c.Writer.WriteString(": heartbeat\n\n"); err != nil {
				slog.Info("SSE write failed, closing", "userID", userID)
				return
			}
			c.Writer.Flush()
		}
	}
}
