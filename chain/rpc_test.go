// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
