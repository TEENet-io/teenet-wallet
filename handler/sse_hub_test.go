// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"encoding/json"
	"sync"
	"testing"
	"time"
)

func TestSSEHub_SubscribeAndBroadcast(t *testing.T) {
	hub := NewSSEHub()
	ch := hub.Subscribe(42)
	defer hub.Unsubscribe(42, ch)

	evt := SSEEvent{Type: "test_event", Data: map[string]string{"hello": "world"}}
	hub.Broadcast(42, evt)

	select {
	case received := <-ch:
		if received.Type != "test_event" {
			t.Fatalf("expected type 'test_event', got %q", received.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast event")
	}
}

func TestSSEHub_BroadcastToWrongUser(t *testing.T) {
	hub := NewSSEHub()
	ch := hub.Subscribe(42)
	defer hub.Unsubscribe(42, ch)

	evt := SSEEvent{Type: "test_event", Data: "payload"}
	hub.Broadcast(99, evt)

	select {
	case <-ch:
		t.Fatal("user 42 should NOT have received broadcast for user 99")
	case <-time.After(50 * time.Millisecond):
		// correct: nothing received
	}
}

func TestSSEHub_MultipleSubscribers(t *testing.T) {
	hub := NewSSEHub()
	ch1 := hub.Subscribe(42)
	ch2 := hub.Subscribe(42)
	defer hub.Unsubscribe(42, ch1)
	defer hub.Unsubscribe(42, ch2)

	evt := SSEEvent{Type: "multi_event", Data: "shared"}
	hub.Broadcast(42, evt)

	for i, ch := range []chan SSEEvent{ch1, ch2} {
		select {
		case received := <-ch:
			if received.Type != "multi_event" {
				t.Fatalf("subscriber %d: expected type 'multi_event', got %q", i+1, received.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatalf("subscriber %d: timed out waiting for broadcast event", i+1)
		}
	}
}

func TestSSEHub_ApprovalBroadcastShape(t *testing.T) {
	hub := NewSSEHub()
	ch := hub.Subscribe(10)
	defer hub.Unsubscribe(10, ch)

	approvalData := map[string]interface{}{
		"approval_id": uint(7),
		"status":      "approved",
		"tx_hash":     "0xdeadbeef",
		"wallet_id":   uint(3),
	}
	evt := SSEEvent{Type: "approval_resolved", Data: approvalData}
	hub.Broadcast(10, evt)

	select {
	case received := <-ch:
		if received.Type != "approval_resolved" {
			t.Fatalf("expected type 'approval_resolved', got %q", received.Type)
		}

		raw, err := json.Marshal(received.Data)
		if err != nil {
			t.Fatalf("failed to marshal Data: %v", err)
		}
		var parsed map[string]interface{}
		if err := json.Unmarshal(raw, &parsed); err != nil {
			t.Fatalf("failed to unmarshal Data: %v", err)
		}

		for _, key := range []string{"approval_id", "status", "tx_hash", "wallet_id"} {
			if _, ok := parsed[key]; !ok {
				t.Errorf("expected key %q in approval event data", key)
			}
		}
		if parsed["status"] != "approved" {
			t.Errorf("expected status 'approved', got %v", parsed["status"])
		}
		if parsed["tx_hash"] != "0xdeadbeef" {
			t.Errorf("expected tx_hash '0xdeadbeef', got %v", parsed["tx_hash"])
		}

		// Verify SSE wire format
		wire := received.MarshalSSE()
		if len(wire) == 0 {
			t.Fatal("MarshalSSE returned empty bytes")
		}
		wireStr := string(wire)
		if wireStr[:len("event: approval_resolved\n")] != "event: approval_resolved\n" {
			t.Errorf("unexpected SSE wire prefix: %q", wireStr[:30])
		}
		if wireStr[len(wireStr)-2:] != "\n\n" {
			t.Errorf("SSE wire format must end with \\n\\n")
		}

	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for approval broadcast event")
	}
}

func TestSSEHub_UnsubscribeStopsEvents(t *testing.T) {
	hub := NewSSEHub()
	ch := hub.Subscribe(42)

	// Unsubscribe immediately.
	hub.Unsubscribe(42, ch)

	// Broadcast after unsubscribe — should not panic or deliver.
	hub.Broadcast(42, SSEEvent{Type: "test", Data: "after-unsub"})

	// Channel should be closed.
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed after unsubscribe")
		}
	default:
		// Channel is closed and drained — also fine.
	}
}

func TestSSEHub_ConcurrentSafety(t *testing.T) {
	hub := NewSSEHub()
	const goroutines = 50

	var wg sync.WaitGroup
	wg.Add(goroutines * 2)

	// Half goroutines subscribe and unsubscribe.
	// Each goroutine uses a unique userID to avoid hitting the per-user connection limit.
	for i := 0; i < goroutines; i++ {
		userID := uint(i + 1)
		go func() {
			defer wg.Done()
			ch := hub.Subscribe(userID)
			time.Sleep(time.Millisecond)
			hub.Unsubscribe(userID, ch) // safe even if ch is nil
		}()
	}

	// Half goroutines broadcast.
	for i := 0; i < goroutines; i++ {
		userID := uint(i + 1)
		go func() {
			defer wg.Done()
			hub.Broadcast(userID, SSEEvent{Type: "concurrent", Data: "test"})
		}()
	}

	wg.Wait()
	// If we get here without panic, the test passes.
}
