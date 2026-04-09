// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/handler"
	"github.com/TEENet-io/teenet-wallet/model"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// policyUSDRouter wires wallet routes with an optional PriceService and a mock RPC
// so that transaction building (nonce, gas) succeeds without hitting a live node.
func policyUSDRouter(t *testing.T, db *gorm.DB, userID uint, ps *handler.PriceService) *gin.Engine {
	t.Helper()
	rpc := mockETHRPCServer(t)
	// Patch chain registry to use mock RPC.
	if cfg, ok := model.GetChain("ethereum"); ok {
		cfg.RPCURL = rpc.URL
		model.SetChain("ethereum", cfg)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, nil, "http://localhost")
	if ps != nil {
		wh.SetPriceService(ps)
	}
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", "passkey")
		c.Next()
	})
	r.PUT("/wallets/:id/policy", wh.SetPolicy)
	r.GET("/wallets/:id/policy", wh.GetPolicy)
	r.DELETE("/wallets/:id/policy", wh.DeletePolicy)
	r.POST("/wallets/:id/transfer", wh.Transfer)
	return r
}

func contractCallUSDRouter(t *testing.T, db *gorm.DB, userID uint, ps *handler.PriceService, authMode string) *gin.Engine {
	t.Helper()
	rpc := mockETHRPCServer(t)
	if cfg, ok := model.GetChain("ethereum"); ok {
		cfg.RPCURL = rpc.URL
		model.SetChain("ethereum", cfg)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	ch := handler.NewContractCallHandler(db, nil, "http://localhost")
	if ps != nil {
		ch.SetPriceService(ps)
	}
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", authMode)
		c.Next()
	})
	r.POST("/wallets/:id/contract-call", ch.ContractCall)
	return r
}

func makePS(ethPrice, solPrice float64) *handler.PriceService {
	srv := fakeCoinGeckoServer(ethPrice, solPrice)
	return handler.NewPriceServiceWithBaseURL(60*time.Second, srv.URL)
}

func respJSON(w *httptest.ResponseRecorder) map[string]interface{} {
	var out map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &out)
	return out
}

// seedWalletWithAddress creates a wallet with a valid-looking ETH address for RPC tests.
func seedWalletWithAddress(t *testing.T, db *gorm.DB) (model.User, model.Wallet) {
	t.Helper()
	user, wallet := seedWallet(t, db)
	wallet.Address = "0x0000000000000000000000000000000000001234"
	db.Save(&wallet)
	return user, wallet
}

// ─── Transfer: USD threshold ──────────────────────────────────────────────────

func TestTransfer_USDThreshold_BelowThreshold_NoApproval(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(2000, 100) // ETH = $2000

	// threshold $500 → 0.1 ETH = $200 < $500 → direct path
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "500", Enabled: true,
	})

	r := policyUSDRouter(t, db, user.ID, ps)
	body := jsonBody(map[string]interface{}{
		"to": "0x0000000000000000000000000000000000000001", "amount": "0.1",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := respJSON(w)
	// Should NOT be pending_approval (sdk=nil → fails at signing → 502, which is fine)
	if resp["status"] == "pending_approval" {
		t.Errorf("expected direct path (not pending_approval) for $200 < $500 threshold")
	}
}

func TestTransfer_USDThreshold_AboveThreshold_PendingApproval(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(2000, 100) // ETH = $2000

	// threshold $100 → 0.1 ETH = $200 > $100 → approval
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "100", Enabled: true,
	})

	r := policyUSDRouter(t, db, user.ID, ps)
	body := jsonBody(map[string]interface{}{
		"to": "0x0000000000000000000000000000000000000001", "amount": "0.1",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}
	resp := respJSON(w)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected pending_approval, got %v", resp["status"])
	}
	if resp["approval_id"] == nil {
		t.Error("expected approval_id in response")
	}
}

// ─── Transfer: Daily Limit USD ────────────────────────────────────────────────

func TestTransfer_DailyLimitUSD_Exceeded(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(2000, 100) // ETH = $2000

	// threshold $10000 (high), daily limit $300 → 0.2 ETH = $400 > $300
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "10000", DailyLimitUSD: "300", Enabled: true,
	})

	r := policyUSDRouter(t, db, user.ID, ps)
	body := jsonBody(map[string]interface{}{
		"to": "0x0000000000000000000000000000000000000001", "amount": "0.2",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for daily limit exceeded, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDailyLimitUSD_Accumulates(t *testing.T) {
	db := testDB(t)
	_, wallet := seedWalletWithAddress(t, db)
	ps := makePS(1000, 100) // ETH=$1000

	// Pre-seed daily_spent=$200 to simulate a prior successful transfer.
	// (With rollback-on-failure, we can't accumulate via failed signing attempts.)
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "10000", DailyLimitUSD: "500",
		DailySpentUSD: "200.00", Enabled: true, DailyResetAt: time.Now().UTC(),
	})

	r := policyUSDRouter(t, db, wallet.UserID, ps)

	// Transfer 0.4 ETH = $400, already spent $200 → total $600 > $500 → blocked
	body := jsonBody(map[string]interface{}{
		"to": "0x0000000000000000000000000000000000000001", "amount": "0.4",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for cumulative daily limit exceeded, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── Transfer: No policy → skip threshold ─────────────────────────────────────

func TestTransfer_NoPolicy_SkipsThreshold(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(2000, 100)

	r := policyUSDRouter(t, db, user.ID, ps)
	body := jsonBody(map[string]interface{}{
		"to": "0x0000000000000000000000000000000000000001", "amount": "100",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := respJSON(w)
	if resp["status"] == "pending_approval" {
		t.Error("should not trigger approval when no policy exists")
	}
}

// ─── DeletePolicy ─────────────────────────────────────────────────────────────

func TestDeletePolicy_Success(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "100", Enabled: true,
	})

	r := policyUSDRouter(t, db, user.ID, nil)
	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/wallets/%s/policy", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var count int64
	db.Model(&model.ApprovalPolicy{}).Where("wallet_id = ?", wallet.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 policies after delete, got %d", count)
	}
}

func TestDeletePolicy_NotFound(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := policyUSDRouter(t, db, user.ID, nil)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/wallets/%s/policy", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestExceedsUSDThreshold_EqualDoesNotTrigger(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(1000, 100) // ETH = $1000

	// threshold $100, transfer 0.1 ETH = exactly $100 → should NOT trigger (strict >)
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "100", Enabled: true,
	})

	r := policyUSDRouter(t, db, user.ID, ps)
	body := jsonBody(map[string]interface{}{
		"to": "0x0000000000000000000000000000000000000001", "amount": "0.1",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := respJSON(w)
	if resp["status"] == "pending_approval" {
		t.Error("equal-to-threshold should NOT trigger approval (strict greater-than)")
	}
}

func TestSetPolicy_APIKey_CreatesPendingApproval(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, nil, "http://localhost")
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("authMode", "apikey") // API key, not passkey
		c.Next()
	})
	r.PUT("/wallets/:id/policy", wh.SetPolicy)

	body := jsonBody(map[string]interface{}{
		"threshold_usd": "500",
	})
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/wallets/%s/policy", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for API key policy change, got %d: %s", w.Code, w.Body.String())
	}
	resp := respJSON(w)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected status=pending_approval, got %v", resp["status"])
	}
	if resp["approval_id"] == nil {
		t.Error("expected approval_id")
	}

	// Should NOT have been applied directly
	var count int64
	db.Model(&model.ApprovalPolicy{}).Where("wallet_id = ?", wallet.ID).Count(&count)
	if count != 0 {
		t.Errorf("policy should not be applied directly via API key, got %d rows", count)
	}
}



func TestTransfer_DailySpent_RollbackOnSigningFailure(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(1000, 100) // ETH=$1000

	// threshold $10000 (won't trigger approval), daily limit $500
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "10000", DailyLimitUSD: "500", Enabled: true,
	})

	r := policyUSDRouter(t, db, user.ID, ps)

	// Transfer 0.1 ETH = $100 — will pre-deduct $100 then fail at signing (sdk=nil)
	body := jsonBody(map[string]interface{}{
		"to": "0x0000000000000000000000000000000000000001", "amount": "0.1",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should have failed at signing (422)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 signing failure, got %d: %s", w.Code, w.Body.String())
	}

	// Key assertion: daily_spent_usd should be rolled back to 0
	var pol model.ApprovalPolicy
	db.Where("wallet_id = ?", wallet.ID).First(&pol)
	if pol.DailySpentUSD != "0" && pol.DailySpentUSD != "0.00" && pol.DailySpentUSD != "0.000000" {
		t.Errorf("expected daily_spent_usd to be rolled back to 0, got %s", pol.DailySpentUSD)
	}
}

func TestTransfer_DailySpent_RollbackOnApprovalPath(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(1000, 100) // ETH=$1000

	// threshold $50, daily limit $500
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "50", DailyLimitUSD: "500", Enabled: true,
	})

	r := policyUSDRouter(t, db, user.ID, ps)

	// Transfer 0.1 ETH = $100 > $50 threshold → approval path
	body := jsonBody(map[string]interface{}{
		"to": "0x0000000000000000000000000000000000000001", "amount": "0.1",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d: %s", w.Code, w.Body.String())
	}

	// Key assertion: daily_spent_usd should NOT have been deducted (rolled back for approval path)
	var pol model.ApprovalPolicy
	db.Where("wallet_id = ?", wallet.ID).First(&pol)
	if pol.DailySpentUSD != "0" && pol.DailySpentUSD != "0.00" && pol.DailySpentUSD != "0.000000" {
		t.Errorf("expected daily_spent_usd=0 (rolled back for approval path), got %s", pol.DailySpentUSD)
	}
}

func policyUSDRouterWithAuth(t *testing.T, db *gorm.DB, userID uint, ps *handler.PriceService, authMode string) *gin.Engine {
	t.Helper()
	rpc := mockETHRPCServer(t)
	if cfg, ok := model.GetChain("ethereum"); ok {
		cfg.RPCURL = rpc.URL
		model.SetChain("ethereum", cfg)
	}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, nil, "http://localhost")
	if ps != nil {
		wh.SetPriceService(ps)
	}
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", authMode)
		c.Next()
	})
	r.POST("/wallets/:id/transfer", wh.Transfer)
	r.GET("/wallets/:id/daily-spent", wh.DailySpent)
	return r
}

func TestDailySpent_WithPolicy(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	db.Create(&model.ApprovalPolicy{
		WalletID:      wallet.ID,
		ThresholdUSD:  "100",
		DailyLimitUSD: "1000",
		DailySpentUSD: "235.50",
		DailyResetAt:  time.Now().UTC().Truncate(24 * time.Hour),
		Enabled:       true,
	})

	r := policyUSDRouterWithAuth(t, db, user.ID, nil, "apikey")
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/daily-spent", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["daily_spent_usd"] != "235.50" {
		t.Errorf("expected daily_spent_usd=235.50, got %v", resp["daily_spent_usd"])
	}
	if resp["daily_limit_usd"] != "1000" {
		t.Errorf("expected daily_limit_usd=1000, got %v", resp["daily_limit_usd"])
	}
	if resp["remaining_usd"] == nil || resp["remaining_usd"] == "" {
		t.Error("expected remaining_usd to be set")
	}
}

func TestDailySpent_NoPolicy(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)

	r := policyUSDRouterWithAuth(t, db, user.ID, nil, "apikey")
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/daily-spent", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["daily_spent_usd"] != "0" {
		t.Errorf("expected daily_spent_usd=0, got %v", resp["daily_spent_usd"])
	}
}

func TestTransfer_UnknownTokenPrice_RequiresApproval(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	db.Create(&model.AllowedContract{
		UserID:          user.ID,
		Chain:           wallet.Chain,
		ContractAddress: "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		Symbol:          "UNKNOWN",
		Decimals:        18,
	})
	db.Create(&model.ApprovalPolicy{WalletID: wallet.ID, ThresholdUSD: "100", Enabled: true})

	ps := makePS(3500, 150)
	r := policyUSDRouterWithAuth(t, db, user.ID, ps, "apikey")
	body := jsonBody(map[string]interface{}{
		"to":     "0x0000000000000000000000000000000000005678",
		"amount": "10",
		"token":  map[string]interface{}{"contract": "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef", "symbol": "UNKNOWN", "decimals": 18},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for unknown token price, got %d: %s", w.Code, w.Body.String())
	}
}

func TestContractCall_NoPolicy_APIKey_StillRequiresApproval(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(2000, 100)

	db.Create(&model.AllowedContract{
		UserID:          user.ID,
		Chain:           wallet.Chain,
		ContractAddress: "0x0000000000000000000000000000000000000abc",
	})

	r := contractCallUSDRouter(t, db, user.ID, ps, "apikey")

	// All contract operations via API Key require approval regardless of policy.
	body := jsonBody(map[string]interface{}{
		"contract": "0x0000000000000000000000000000000000000abc",
		"func_sig": "doSomething()",
		"args":     []interface{}{},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for API Key contract call, got %d: %s", w.Code, w.Body.String())
	}
	resp := respJSON(w)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected pending_approval, got %v", resp["status"])
	}
}

// ─── Transfer: token priced by contract address ───────────────────────────────

func TestTransfer_TokenPricedByContractAddress_BelowThreshold(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	// Whitelist UNI token — not a stablecoin, not in coinGeckoIDs
	db.Create(&model.AllowedContract{
		UserID:          user.ID,
		Chain:           wallet.Chain,
		ContractAddress: "0x1f9840a85d5af5bf1d1762f925bdaddc4201f984",
		Symbol:          "UNI",
		Decimals:        18,
	})
	db.Create(&model.ApprovalPolicy{WalletID: wallet.ID, ThresholdUSD: "500", Enabled: true})

	// Token server knows UNI = $10
	priceSrv := fakeCoinGeckoTokenServer(map[string]float64{
		"0x1f9840a85d5af5bf1d1762f925bdaddc4201f984": 10.0,
	})
	defer priceSrv.Close()
	ps := handler.NewPriceServiceWithBaseURL(60*time.Second, priceSrv.URL)

	r := policyUSDRouterWithAuth(t, db, user.ID, ps, "apikey")
	// Transfer 10 UNI = $100 < threshold $500
	body := jsonBody(map[string]interface{}{
		"to":     "0x0000000000000000000000000000000000005678",
		"amount": "10",
		"token":  map[string]interface{}{"contract": "0x1f9840a85d5af5bf1d1762f925bdaddc4201f984", "symbol": "UNI", "decimals": 18},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// $100 < $500 threshold → should NOT be 202 (bypasses approval)
	// SDK is nil so expect 502 (signing failed), NOT 202
	if w.Code == http.StatusAccepted {
		t.Fatalf("expected approval bypass for token priced by contract address, got 202: %s", w.Body.String())
	}
}

// ─── Daily-spent: cross-day reset ────────────────────────────────────────────

func TestDailySpent_CrossDayReset(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	// Policy with spent from yesterday
	yesterday := time.Now().UTC().Truncate(24*time.Hour).Add(-24 * time.Hour)
	db.Create(&model.ApprovalPolicy{
		WalletID:      wallet.ID,
		ThresholdUSD:  "100",
		DailyLimitUSD: "1000",
		DailySpentUSD: "500.00",
		DailyResetAt:  yesterday,
		Enabled:       true,
	})

	r := policyUSDRouterWithAuth(t, db, user.ID, nil, "apikey")
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/daily-spent", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	// Yesterday's spend should be reset to 0
	if resp["daily_spent_usd"] != "0" {
		t.Errorf("expected daily_spent_usd=0 after day reset, got %v", resp["daily_spent_usd"])
	}
	if resp["remaining_usd"] != "1000.000000" {
		t.Errorf("expected remaining_usd=1000.000000, got %v", resp["remaining_usd"])
	}
}
