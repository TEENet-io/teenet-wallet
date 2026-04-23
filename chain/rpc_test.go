// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestETHCall_Success(t *testing.T) {
	// Raw bytes we expect back after hex decoding.
	want := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01} // uint256 = 1

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"result":  "0x0000000000000000000000000000000000000000000000000000000000000001",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	calldata := []byte{0x18, 0x16, 0x0d, 0xdd} // totalSupply() selector
	got, err := ETHCall(srv.URL, "", "0xContractAddr", calldata)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("result length mismatch: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d mismatch: got 0x%02x, want 0x%02x", i, got[i], want[i])
		}
	}
}

func TestETHCall_RPCError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"id":      1,
			"error": map[string]interface{}{
				"code":    -32000,
				"message": "execution reverted",
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	_, err := ETHCall(srv.URL, "", "0xContractAddr", []byte{0xde, 0xad, 0xbe, 0xef})
	if err == nil {
		t.Fatal("expected error from RPC error response, got nil")
	}
}

func TestETHCall_EmptyRPCURL(t *testing.T) {
	_, err := ETHCall("", "", "0xContractAddr", []byte{0x01})
	if err == nil {
		t.Fatal("expected error for empty RPC URL, got nil")
	}
}

// newCountingRPC returns a test server that counts hits and returns `body` as
// JSON-RPC response. If `status` is non-2xx the body is ignored and that status
// is returned — used to simulate transport-level provider failures.
func newCountingRPC(t *testing.T, status int, body map[string]interface{}) (*httptest.Server, *int) {
	t.Helper()
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if status != 200 {
			w.WriteHeader(status)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func TestRPCFallback_PrimaryTransportFailure_UsesFallback(t *testing.T) {
	ClearRPCFallbacks()
	defer ClearRPCFallbacks()

	primary, primaryHits := newCountingRPC(t, 503, nil)
	fallback, fallbackHits := newCountingRPC(t, 200, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"result": "0x0000000000000000000000000000000000000000000000000000000000000001",
	})
	SetRPCFallback(primary.URL, fallback.URL)

	got, err := ETHCall(primary.URL, "", "0xContractAddr", []byte{0x18, 0x16, 0x0d, 0xdd})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("expected non-empty result from fallback")
	}
	if *primaryHits != 3 {
		t.Errorf("primary hits: got %d, want 3 (retries exhausted)", *primaryHits)
	}
	if *fallbackHits < 1 {
		t.Errorf("fallback hits: got %d, want >=1", *fallbackHits)
	}
}

func TestRPCFallback_AppError_DoesNotUseFallback(t *testing.T) {
	ClearRPCFallbacks()
	defer ClearRPCFallbacks()

	primary, primaryHits := newCountingRPC(t, 200, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"error": map[string]interface{}{"code": -32000, "message": "execution reverted"},
	})
	fallback, fallbackHits := newCountingRPC(t, 200, map[string]interface{}{
		"jsonrpc": "2.0", "id": 1,
		"result": "0x00",
	})
	SetRPCFallback(primary.URL, fallback.URL)

	_, err := ETHCall(primary.URL, "", "0xContractAddr", []byte{0xde, 0xad, 0xbe, 0xef})
	if err == nil {
		t.Fatal("expected application error to propagate, got nil")
	}
	if *primaryHits != 1 {
		t.Errorf("primary hits: got %d, want 1 (app error should not retry)", *primaryHits)
	}
	if *fallbackHits != 0 {
		t.Errorf("fallback hits: got %d, want 0 (app error should not trigger fallback)", *fallbackHits)
	}
}

func TestRPCFallback_NoFallbackRegistered_ErrorPropagates(t *testing.T) {
	ClearRPCFallbacks()
	defer ClearRPCFallbacks()

	primary, primaryHits := newCountingRPC(t, 503, nil)

	_, err := ETHCall(primary.URL, "", "0xContractAddr", []byte{0x01})
	if err == nil {
		t.Fatal("expected error when no fallback is registered and primary fails")
	}
	if *primaryHits != 3 {
		t.Errorf("primary hits: got %d, want 3", *primaryHits)
	}
}

func TestRPCFallback_BothFail_ErrorMentionsBoth(t *testing.T) {
	ClearRPCFallbacks()
	defer ClearRPCFallbacks()

	primary, _ := newCountingRPC(t, 503, nil)
	fallback, _ := newCountingRPC(t, 500, nil)
	SetRPCFallback(primary.URL, fallback.URL)

	_, err := ETHCall(primary.URL, "", "0xContractAddr", []byte{0x01})
	if err == nil {
		t.Fatal("expected error when both primary and fallback fail")
	}
	// Error should mention both primary and fallback (use hostnames, not full URLs).
	msg := err.Error()
	if !strings.Contains(msg, "primary") || !strings.Contains(msg, "fallback") {
		t.Errorf("error message should mention primary + fallback, got: %s", msg)
	}
}

// TestJSONRPC_StripsURLOnTransportFailure guards the chain-layer fix that
// prevents provider tokens (embedded in RPC URL paths, e.g. QuickNode)
// from leaking into err strings. Go's net/http returns *url.Error on
// transport failures, whose Error() embeds the full URL verbatim — we
// unwrap it and keep only the host.
func TestJSONRPC_StripsURLOnTransportFailure(t *testing.T) {
	// Reserve a port, close immediately → next request will hit
	// "connection refused" at the transport layer (produces a *url.Error).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	base := srv.URL
	srv.Close()
	const secretToken = "SECRET_CHAIN_LAYER_TOKEN"
	endpoint := base + "/" + secretToken + "/v1/"

	_, err := jsonRPC(endpoint, map[string]interface{}{"jsonrpc": "2.0", "method": "eth_chainId", "id": 1})
	if err == nil {
		t.Fatal("expected transport error on closed server")
	}
	msg := err.Error()
	if strings.Contains(msg, secretToken) {
		t.Fatalf("jsonRPC leaked provider token in err: %s", msg)
	}
	if strings.Contains(msg, endpoint) {
		t.Fatalf("jsonRPC leaked full URL in err: %s", msg)
	}
	// Host-only form should still be present for diagnostics.
	if !strings.Contains(msg, HostOnly(endpoint)) {
		t.Errorf("expected host-only context %q in err, got: %s", HostOnly(endpoint), msg)
	}
}
