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
	sdk "github.com/TEENet-io/teenet-sdk/go"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/handler"
	"github.com/TEENet-io/teenet-wallet/model"
)

// ethTransferRouter wires a minimal gin router for Transfer tests.
// sdkClient may be nil for tests that never reach TEE signing.
func ethTransferRouter(db *gorm.DB, userID uint, sdkClient *sdk.Client, ethRPC string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, sdkClient, "http://localhost")
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", "passkey")
		c.Next()
	})
	r.POST("/wallets/:id/transfer", wh.Transfer)
	return r
}

// ethTransferRouterWithRPC is like ethTransferRouter but patches the "ethereum"
// chain's RPC endpoint for the duration of the test (mirrors contractCallRouter).
func ethTransferRouterWithRPC(t *testing.T, db *gorm.DB, userID uint, sdkClient *sdk.Client, rpcURL string) *gin.Engine {
	t.Helper()
	if rpcURL != "" {
		if cfg, ok := model.GetChain("ethereum"); ok {
			original := cfg
			t.Cleanup(func() { model.SetChain("ethereum", original) })
			cfg.RPCURL = rpcURL
			model.SetChain("ethereum", cfg)
		}
	}
	return ethTransferRouter(db, userID, sdkClient, "")
}

// ─── ERC-20 whitelist gate ────────────────────────────────────────────────────

func TestTransfer_ERC20_NotWhitelisted(t *testing.T) {
	db := testDB(t)
	const uid uint = 10
	db.Create(&model.User{ID: uid, Username: "erc20user"})
	wallet := model.Wallet{UserID: uid, Chain: "ethereum", KeyName: "k-erc20-nwl", Status: "ready"}
	db.Create(&wallet)

	r := ethTransferRouter(db, uid, nil, "")

	body := jsonBody(map[string]interface{}{
		"to":     "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"amount": "100",
		"token": map[string]interface{}{
			"contract": "0x1234567890123456789012345678901234567890",
			"symbol":   "USDC",
			"decimals": 6,
		},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unwhitelisted contract, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTransfer_ERC20_Whitelisted_FailsAtRPC(t *testing.T) {
	// Contract is whitelisted → 403 must NOT occur.
	// With empty ethRPC, BuildETHContractCallTx returns "ETH_RPC_URL is not configured" → 502.
	db := testDB(t)
	const uid uint = 11
	db.Create(&model.User{ID: uid, Username: "erc20user2"})
	wallet := model.Wallet{UserID: uid, Chain: "ethereum", KeyName: "k-erc20-wl", Status: "ready"}
	db.Create(&wallet)

	contractAddr := "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
	db.Create(&model.AllowedContract{
		UserID: uid, Chain: wallet.Chain, ContractAddress: contractAddr, Symbol: "USDC", Decimals: 6,
	})

	r := ethTransferRouter(db, uid, nil, "") // empty ethRPC

	body := jsonBody(map[string]interface{}{
		"to":     "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"amount": "100",
		"token":  map[string]interface{}{"contract": contractAddr, "symbol": "USDC", "decimals": 6},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Fatal("got 403 for a whitelisted contract — whitelist check should have passed")
	}
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 (build tx failure), got %d: %s", w.Code, w.Body.String())
	}
}

func TestTransfer_NativeETH_SkipsContractCheck(t *testing.T) {
	// Native ETH (no token field) must never consult the contract whitelist.
	db := testDB(t)
	const uid uint = 12
	db.Create(&model.User{ID: uid, Username: "nativeuser"})
	wallet := model.Wallet{UserID: uid, Chain: "ethereum", KeyName: "k-native", Status: "ready"}
	db.Create(&wallet)

	r := ethTransferRouter(db, uid, nil, "")

	body := jsonBody(map[string]interface{}{
		"to":     "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"amount": "0.001",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code == http.StatusForbidden {
		t.Fatal("native transfer should not trigger contract whitelist check")
	}
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 (build tx failure), got %d: %s", w.Code, w.Body.String())
	}
}

func TestTransfer_InvalidAmount(t *testing.T) {
	db := testDB(t)
	const uid uint = 13
	db.Create(&model.User{ID: uid, Username: "badamount"})
	wallet := model.Wallet{UserID: uid, Chain: "ethereum", KeyName: "k-badamt", Status: "ready"}
	db.Create(&wallet)

	r := ethTransferRouter(db, uid, nil, "")

	body := jsonBody(map[string]interface{}{
		"to":     "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"amount": "not-a-number",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid amount, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTransfer_WalletNotReady(t *testing.T) {
	db := testDB(t)
	const uid uint = 14
	db.Create(&model.User{ID: uid, Username: "notreadyuser"})
	wallet := model.Wallet{UserID: uid, Chain: "ethereum", KeyName: "k-notready", Status: "creating"}
	db.Create(&wallet)

	r := ethTransferRouter(db, uid, nil, "")

	body := jsonBody(map[string]interface{}{
		"to":     "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"amount": "0.1",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-ready wallet, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── ERC-20 uint256 overflow ──────────────────────────────────────────────────

func TestTransfer_ERC20_Uint256Overflow(t *testing.T) {
	db := testDB(t)
	const uid uint = 15
	db.Create(&model.User{ID: uid, Username: "overflowuser"})
	wallet := model.Wallet{UserID: uid, Chain: "ethereum", KeyName: "k-overflow", Status: "ready"}
	db.Create(&wallet)

	contractAddr := "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb49"
	db.Create(&model.AllowedContract{
		UserID: uid, Chain: wallet.Chain, ContractAddress: contractAddr, Symbol: "USDC", Decimals: 6,
	})

	r := ethTransferRouter(db, uid, nil, "")

	// Amount larger than 2^256 — will produce a tokenUnits value whose byte
	// representation exceeds 32 bytes and must be rejected.
	body := jsonBody(map[string]interface{}{
		"to":     "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"amount": "999999999999999999999999999999999999999999999999999999999999999999999999999999999",
		"token":  map[string]interface{}{"contract": contractAddr, "symbol": "USDC", "decimals": 6},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for uint256 overflow, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── ERC-20 revert-reason propagation ─────────────────────────────────────────

// TestTransfer_ERC20_UnreachableRPC_DoesNotLeakProviderToken guards against
// leaking the full RPC URL (and any provider token embedded in its path,
// such as a QuickNode token per main.go's override) on transport failures.
// Go's net/http wraps transport errors in url.Error, which embeds the URL
// verbatim; respondBuildTxFailed must route that through sanitizeErrString
// before putting it in the 422 response body.
func TestTransfer_ERC20_UnreachableRPC_DoesNotLeakProviderToken(t *testing.T) {
	db := testDB(t)
	const uid uint = 43
	db.Create(&model.User{ID: uid, Username: "leakguard"})
	wallet := model.Wallet{UserID: uid, Chain: "ethereum", KeyName: "k-leakguard", Status: "ready", Address: "0x2222222222222222222222222222222222222222"}
	db.Create(&wallet)

	contractAddr := "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
	db.Create(&model.AllowedContract{
		UserID: uid, Chain: wallet.Chain, ContractAddress: contractAddr, Symbol: "USDC", Decimals: 6,
	})

	// Stand up a server just to reserve a real port, then close it so any
	// request to the URL yields a transport error ("connection refused").
	// Embed a synthetic provider token in the path — the test checks this
	// token never appears in the response body.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	baseURL := srv.URL
	srv.Close()
	const secretToken = "SECRET_QN_TOKEN_ZZZ" //nolint:gosec // synthetic test value
	fakeRPC := baseURL + "/" + secretToken + "/v1/"

	r := ethTransferRouterWithRPC(t, db, uid, nil, fakeRPC)

	body := jsonBody(map[string]interface{}{
		"to":     "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"amount": "1",
		"token":  map[string]interface{}{"contract": contractAddr, "symbol": "USDC", "decimals": 6},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 on unreachable RPC, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response is not JSON: %v\n%s", err, w.Body.String())
	}
	// Primary security contract: the provider token must not appear anywhere
	// in the response body — not in rpc_error, not in the error message, not
	// in any other field.
	if strings.Contains(w.Body.String(), secretToken) {
		t.Fatalf("response body leaked provider token %q: %s", secretToken, w.Body.String())
	}
	// rpc_error should still carry a meaningful diagnostic so the caller
	// knows *what* failed (connection-level symptom) — just without the URL.
	rpcError, _ := resp["rpc_error"].(string)
	if rpcError == "" {
		t.Fatalf("rpc_error field missing or empty: %s", w.Body.String())
	}
	if !strings.Contains(strings.ToLower(rpcError), "connection refused") &&
		!strings.Contains(strings.ToLower(rpcError), "connect") {
		t.Fatalf("rpc_error lost its diagnostic tail, got: %s", rpcError)
	}
}

// TestTransfer_ERC20_EstimateGasRevert_PropagatesReason ensures that when
// eth_estimateGas reverts with a Solidity Error(string), the /transfer
// response surfaces the decoded revert_reason and the raw rpc_error so the
// caller can tell why the tx failed (e.g. "ERC20: transfer amount exceeds
// balance") instead of getting a bare "failed to build transaction".
func TestTransfer_ERC20_EstimateGasRevert_PropagatesReason(t *testing.T) {
	db := testDB(t)
	const uid uint = 42
	db.Create(&model.User{ID: uid, Username: "revertuser"})
	wallet := model.Wallet{UserID: uid, Chain: "ethereum", KeyName: "k-revert", Status: "ready", Address: "0x1111111111111111111111111111111111111111"}
	db.Create(&wallet)

	contractAddr := "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48"
	db.Create(&model.AllowedContract{
		UserID: uid, Chain: wallet.Chain, ContractAddress: contractAddr, Symbol: "USDC", Decimals: 6,
	})

	rpc := mockETHRPCServerEstimateRevert(t, "ERC20: transfer amount exceeds balance")
	r := ethTransferRouterWithRPC(t, db, uid, nil, rpc.URL)

	body := jsonBody(map[string]interface{}{
		"to":     "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"amount": "100",
		"token":  map[string]interface{}{"contract": contractAddr, "symbol": "USDC", "decimals": 6},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for estimateGas revert, got %d: %s", w.Code, w.Body.String())
	}
	bodyStr := w.Body.String()
	if !strings.Contains(bodyStr, "ERC20: transfer amount exceeds balance") {
		t.Fatalf("expected revert reason in response, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"revert_reason"`) {
		t.Fatalf("expected revert_reason field in response, got: %s", bodyStr)
	}
	if !strings.Contains(bodyStr, `"rpc_error"`) {
		t.Fatalf("expected rpc_error field in response, got: %s", bodyStr)
	}
}
