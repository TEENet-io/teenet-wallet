// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import (
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestBuildETHTx_AlwaysFreshNonce is a regression test for the
// "NonceManager local-counter drift" class of phantom-transfer bugs.
//
// Before this fix, BuildETHTx would advance an in-memory counter on every
// call, so a sign-failure that never broadcast still "burned" the nonce,
// and any subsequent BuildETHTx call picked up a higher-than-chain-actual
// nonce. This test locks that path shut: two back-to-back BuildETHTx calls
// against a mock RPC that always reports pending=5 must BOTH produce a tx
// with nonce=5.
func TestBuildETHTx_AlwaysFreshNonce(t *testing.T) {
	const pendingNonce = uint64(5)

	var txCountCalls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		method, _ := req["method"].(string)

		resp := map[string]interface{}{"jsonrpc": "2.0", "id": req["id"]}
		switch method {
		case "eth_getTransactionCount":
			atomic.AddInt32(&txCountCalls, 1)
			resp["result"] = "0x5" // always pending=5, never advance
		case "eth_maxPriorityFeePerGas":
			resp["result"] = "0x3b9aca00" // 1 gwei
		case "eth_getBlockByNumber":
			resp["result"] = map[string]interface{}{
				"number":        "0x1",
				"baseFeePerGas": "0x3b9aca00",
				"timestamp":     "0x1",
			}
		case "eth_chainId":
			resp["result"] = "0xa"
		default:
			t.Errorf("unexpected RPC method: %s", method)
			resp["error"] = map[string]interface{}{"code": -32601, "message": "method not found"}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	from := "0x1111111111111111111111111111111111111111"
	to := "0x2222222222222222222222222222222222222222"

	txA, err := BuildETHTx(srv.URL, from, to, big.NewFloat(0.001))
	if err != nil {
		t.Fatalf("first BuildETHTx failed: %v", err)
	}
	txB, err := BuildETHTx(srv.URL, from, to, big.NewFloat(0.001))
	if err != nil {
		t.Fatalf("second BuildETHTx failed: %v", err)
	}

	if txA.Params.Nonce != pendingNonce {
		t.Errorf("first tx nonce = %d, want %d", txA.Params.Nonce, pendingNonce)
	}
	if txB.Params.Nonce != pendingNonce {
		t.Errorf("second tx nonce = %d, want %d (fresh fetch must not advance local counter)",
			txB.Params.Nonce, pendingNonce)
	}
	if calls := atomic.LoadInt32(&txCountCalls); calls != 2 {
		t.Errorf("expected 2 getTransactionCount calls (one per BuildETHTx), got %d", calls)
	}
}

// TestBuildETHTx_NonceReflectsChain verifies that when the mock RPC reports a
// different pending nonce on the second call (simulating a tx landing in
// mempool between calls), BuildETHTx picks up the new value rather than a
// cached-and-advanced one.
func TestBuildETHTx_NonceReflectsChain(t *testing.T) {
	var call int32
	nonces := []string{"0x5", "0x6"}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&req)
		method, _ := req["method"].(string)

		resp := map[string]interface{}{"jsonrpc": "2.0", "id": req["id"]}
		switch method {
		case "eth_getTransactionCount":
			idx := atomic.AddInt32(&call, 1) - 1
			if int(idx) >= len(nonces) {
				idx = int32(len(nonces) - 1)
			}
			resp["result"] = nonces[idx]
		case "eth_maxPriorityFeePerGas":
			resp["result"] = "0x3b9aca00"
		case "eth_getBlockByNumber":
			resp["result"] = map[string]interface{}{
				"number":        "0x1",
				"baseFeePerGas": "0x3b9aca00",
				"timestamp":     "0x1",
			}
		case "eth_chainId":
			resp["result"] = "0xb"
		default:
			t.Errorf("unexpected RPC method: %s", method)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	from := "0x3333333333333333333333333333333333333333"
	to := "0x4444444444444444444444444444444444444444"

	txA, err := BuildETHTx(srv.URL, from, to, big.NewFloat(0.001))
	if err != nil {
		t.Fatalf("first BuildETHTx failed: %v", err)
	}
	txB, err := BuildETHTx(srv.URL, from, to, big.NewFloat(0.001))
	if err != nil {
		t.Fatalf("second BuildETHTx failed: %v", err)
	}

	if txA.Params.Nonce != 5 {
		t.Errorf("first tx nonce = %d, want 5", txA.Params.Nonce)
	}
	if txB.Params.Nonce != 6 {
		t.Errorf("second tx nonce = %d, want 6 (fresh fetch must reflect chain)", txB.Params.Nonce)
	}
}
