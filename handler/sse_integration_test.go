// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestSSEStream_ReceivesApprovalEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := NewSSEHub()
	sseH := NewSSEHandler(hub)

	r := gin.New()
	// Fake auth middleware that sets userID=42.
	r.GET("/api/events/stream", func(c *gin.Context) {
		c.Set("userID", uint(42))
		c.Next()
	}, sseH.Stream)

	srv := httptest.NewServer(r)
	defer srv.Close()

	// Connect as SSE client.
	resp, err := http.Get(srv.URL + "/api/events/stream")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %s", ct)
	}

	scanner := bufio.NewScanner(resp.Body)

	// Read the initial ": connected" comment.
	if scanner.Scan() {
		line := scanner.Text()
		if !strings.Contains(line, "connected") {
			t.Errorf("expected connected comment, got: %s", line)
		}
	}

	// Broadcast an event after a small delay.
	go func() {
		time.Sleep(100 * time.Millisecond)
		hub.Broadcast(42, SSEEvent{
			Type: "approval_resolved",
			Data: map[string]interface{}{
				"approval_id": 99,
				"status":      "approved",
				"tx_hash":     "0xdeadbeef",
			},
		})
	}()

	// Read the event lines.
	var eventType, data string
	deadline := time.After(3 * time.Second)
	done := false
	for !done {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for SSE event")
			return
		default:
		}
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
			done = true
		}
	}

	if eventType != "approval_resolved" {
		t.Errorf("expected event type approval_resolved, got %s", eventType)
	}
	if !strings.Contains(data, "0xdeadbeef") {
		t.Errorf("expected tx_hash in data, got %s", data)
	}
	if !strings.Contains(data, `"status":"approved"`) {
		t.Errorf("expected approved status in data, got %s", data)
	}
}

func TestSSEStream_ReceivesRejectedEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := NewSSEHub()
	sseH := NewSSEHandler(hub)

	r := gin.New()
	r.GET("/api/events/stream", func(c *gin.Context) {
		c.Set("userID", uint(42))
		c.Next()
	}, sseH.Stream)

	srv := httptest.NewServer(r)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/events/stream")
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	// Skip ": connected" line.
	scanner.Scan()

	go func() {
		time.Sleep(100 * time.Millisecond)
		hub.Broadcast(42, SSEEvent{
			Type: "approval_resolved",
			Data: map[string]interface{}{
				"approval_id":   55,
				"status":        "rejected",
				"approval_type": "transfer",
			},
		})
	}()

	var eventType, data string
	deadline := time.After(3 * time.Second)
	done := false
	for !done {
		select {
		case <-deadline:
			t.Fatal("timeout waiting for SSE event")
			return
		default:
		}
		if !scanner.Scan() {
			break
		}
		line := scanner.Text()
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		}
		if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
			done = true
		}
	}

	if eventType != "approval_resolved" {
		t.Errorf("expected approval_resolved, got %s", eventType)
	}
	if !strings.Contains(data, `"rejected"`) {
		t.Errorf("expected rejected status in data, got %s", data)
	}
	if !strings.Contains(data, `"transfer"`) {
		t.Errorf("expected transfer type in data, got %s", data)
	}
}

func TestSSEStream_UserIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)
	hub := NewSSEHub()
	sseH := NewSSEHandler(hub)

	r := gin.New()
	r.GET("/api/events/user42", func(c *gin.Context) {
		c.Set("userID", uint(42))
		c.Next()
	}, sseH.Stream)
	r.GET("/api/events/user99", func(c *gin.Context) {
		c.Set("userID", uint(99))
		c.Next()
	}, sseH.Stream)

	srv := httptest.NewServer(r)
	defer srv.Close()

	// Connect as user 42.
	resp42, err := http.Get(srv.URL + "/api/events/user42")
	if err != nil {
		t.Fatalf("connect user42: %v", err)
	}
	defer resp42.Body.Close()
	scanner42 := bufio.NewScanner(resp42.Body)
	scanner42.Scan() // skip ": connected"

	// Broadcast to user 99 only.
	go func() {
		time.Sleep(100 * time.Millisecond)
		hub.Broadcast(99, SSEEvent{
			Type: "approval_resolved",
			Data: map[string]interface{}{"approval_id": 1, "status": "approved"},
		})
		// Then broadcast to user 42.
		time.Sleep(50 * time.Millisecond)
		hub.Broadcast(42, SSEEvent{
			Type: "approval_resolved",
			Data: map[string]interface{}{"approval_id": 2, "status": "approved"},
		})
	}()

	// User 42 should receive approval_id=2 (not 1).
	var data string
	deadline := time.After(3 * time.Second)
	for {
		select {
		case <-deadline:
			t.Fatal("timeout")
			return
		default:
		}
		if !scanner42.Scan() {
			break
		}
		line := scanner42.Text()
		if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
			break
		}
	}

	if !strings.Contains(data, `"approval_id":2`) {
		// Try without space too
		if !strings.Contains(data, `"approval_id": 2`) {
			t.Errorf("user 42 should receive approval_id 2, got: %s", data)
		}
	}
}
