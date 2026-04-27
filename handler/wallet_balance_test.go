// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"github.com/TEENet-io/teenet-wallet/handler"
	"github.com/TEENet-io/teenet-wallet/model"
)

// mockETHRPCServerSmallBalance behaves like mockETHRPCServer except
// eth_getBalance returns a small value (default 1000 wei). Used to
// drive the pre-flight balance check into its rejection branch.
func mockETHRPCServerSmallBalance(t *testing.T, balanceHex string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		method, _ := req["method"].(string)
		var result interface{}
		switch method {
		case "eth_getTransactionCount":
			result = "0x1"
		case "eth_gasPrice":
			result = "0x3B9ACA00"
		case "eth_maxPriorityFeePerGas":
			result = "0x3B9ACA00"
		case "eth_getBlockByNumber":
			result = map[string]interface{}{
				"baseFeePerGas": "0x3B9ACA00",
				"gasLimit":      "0x1C9C380",
				"number":        "0x1",
			}
		case "eth_chainId":
			result = "0x1"
		case "eth_estimateGas":
			result = "0xEA60"
		case "eth_getBalance":
			result = balanceHex
		default:
			result = nil
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestTransfer_PreflightBalance_RejectsInsufficient verifies the new
// pre-flight check refuses native transfers that the chain would
// reject for "insufficient funds" anyway, but does so synchronously
// (HTTP 400) instead of:
//   - creating a useless pending approval that resolves to a broadcast
//     failure when approved (API-key path), or
//   - leaking a cryptic chain-level error after a passkey assertion.
func TestTransfer_PreflightBalance_RejectsInsufficient(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	rpc := mockETHRPCServerSmallBalance(t, "0x3e8") // 1000 wei — way under any real transfer

	if cfg, ok := model.GetChain("ethereum"); ok {
		cfg.RPCURL = rpc.URL
		model.SetChain("ethereum", cfg)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, nil, "http://localhost")
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("authMode", "passkey")
		c.Next()
	})
	r.POST("/wallets/:id/transfer", wh.Transfer)

	body := jsonBody(map[string]interface{}{
		"to":     "0x0000000000000000000000000000000000000001",
		"amount": "0.1",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 from pre-flight balance check, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "insufficient balance") {
		t.Errorf("expected error to mention 'insufficient balance', got: %s", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "value + gas") {
		t.Errorf("expected error to identify value+gas as the failing component, got: %s", w.Body.String())
	}
}

// TestTransfer_PreflightBalance_AllowsSufficient confirms the check
// doesn't fire when balance covers value + gas. (Downstream signing
// fails with sdk=nil, but we should reach that point — the request
// must pass the pre-flight check first.)
func TestTransfer_PreflightBalance_AllowsSufficient(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	rpc := mockETHRPCServerSmallBalance(t, "0x3635c9adc5dea00000") // 1000 ETH

	if cfg, ok := model.GetChain("ethereum"); ok {
		cfg.RPCURL = rpc.URL
		model.SetChain("ethereum", cfg)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, nil, "http://localhost")
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("authMode", "passkey")
		c.Next()
	})
	r.POST("/wallets/:id/transfer", wh.Transfer)

	body := jsonBody(map[string]interface{}{
		"to":     "0x0000000000000000000000000000000000000001",
		"amount": "0.1",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Anything other than 400-for-insufficient-balance counts as "the
	// pre-flight check let it through". Downstream may still error
	// (sdk=nil, broadcast failure, etc.) — that's fine here.
	if w.Code == http.StatusBadRequest && strings.Contains(w.Body.String(), "insufficient balance") {
		t.Fatalf("balance pre-check incorrectly rejected a sufficient balance: %s", w.Body.String())
	}
}
