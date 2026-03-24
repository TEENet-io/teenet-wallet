package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
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
	if cfg, ok := model.Chains["ethereum"]; ok {
		cfg.RPCURL = rpc.URL
		model.Chains["ethereum"] = cfg
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
	if cfg, ok := model.Chains["ethereum"]; ok {
		cfg.RPCURL = rpc.URL
		model.Chains["ethereum"] = cfg
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

// ─── Contract Call + amount_usd ───────────────────────────────────────────────

func TestContractCall_AmountUSD_AboveThreshold_PendingApproval(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(2000, 100)

	db.Create(&model.AllowedContract{
		WalletID:        wallet.ID,
		ContractAddress: "0x0000000000000000000000000000000000000abc",
		AutoApprove:     true,
	})
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "50", Enabled: true,
	})

	r := contractCallUSDRouter(t, db, user.ID, ps, "apikey")

	// amount_usd=100 > threshold $50 → approval
	body := jsonBody(map[string]interface{}{
		"contract":   "0x0000000000000000000000000000000000000abc",
		"func_sig":   "swap(uint256)",
		"args":       []interface{}{"1000000"},
		"amount_usd": "100",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
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
}

func TestContractCall_AmountUSD_DailyLimitExceeded(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(2000, 100)

	db.Create(&model.AllowedContract{
		WalletID:        wallet.ID,
		ContractAddress: "0x0000000000000000000000000000000000000abc",
		AutoApprove:     true,
	})
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "10000", DailyLimitUSD: "50", Enabled: true,
	})

	r := contractCallUSDRouter(t, db, user.ID, ps, "apikey")

	// amount_usd=100 > daily limit $50 → hard block
	body := jsonBody(map[string]interface{}{
		"contract":   "0x0000000000000000000000000000000000000abc",
		"func_sig":   "swap(uint256)",
		"args":       []interface{}{"1000000"},
		"amount_usd": "100",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestContractCall_NativeValueVsAmountUSD_UsesLarger(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(2000, 100) // ETH = $2000

	db.Create(&model.AllowedContract{
		WalletID:        wallet.ID,
		ContractAddress: "0x0000000000000000000000000000000000000abc",
		AutoApprove:     true,
	})
	// Threshold $250
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "250", Enabled: true,
	})

	r := contractCallUSDRouter(t, db, user.ID, ps, "apikey")

	// value=0.1 ETH = $200, amount_usd=300 → max(200,300)=300 > $250 → approval
	body := jsonBody(map[string]interface{}{
		"contract":   "0x0000000000000000000000000000000000000abc",
		"func_sig":   "deposit(uint256)",
		"args":       []interface{}{"1000000"},
		"value":      "0.1",
		"amount_usd": "300",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 (max of value and amount_usd exceeds threshold), got %d: %s", w.Code, w.Body.String())
	}
}

func TestContractCall_Solana_AmountUSD_AboveThreshold(t *testing.T) {
	db := testDB(t)
	n := atomic.AddInt64(&dbCounter, 1)
	user := model.User{Username: fmt.Sprintf("sol-usd-%d", n), PasskeyUserID: uint(n)}
	db.Create(&user)
	wallet := model.Wallet{
		UserID:  user.ID,
		Chain:   "solana",
		KeyName: fmt.Sprintf("k-sol-usd-%d", n),
		Address: "11111111111111111111111111111111",
		Status:  "ready",
	}
	db.Create(&wallet)

	programID := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	db.Create(&model.AllowedContract{
		WalletID:        wallet.ID,
		ContractAddress: programID,
		AutoApprove:     true,
	})
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "50", Enabled: true,
	})

	solRPC := mockSOLRPCServer(t)
	if cfg, ok := model.Chains["solana"]; ok {
		cfg.RPCURL = solRPC.URL
		model.Chains["solana"] = cfg
	}

	ps := makePS(2000, 100)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ch := handler.NewContractCallHandler(db, nil, "http://localhost")
	ch.SetPriceService(ps)
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("authMode", "apikey")
		c.Next()
	})
	r.POST("/wallets/:id/contract-call", ch.ContractCall)

	// amount_usd=100 > threshold $50 → approval
	body := jsonBody(map[string]interface{}{
		"contract": programID,
		"accounts": []map[string]interface{}{
			{"pubkey": "11111111111111111111111111111111", "is_signer": true, "is_writable": true},
		},
		"data":       "0300000000000000",
		"amount_usd": "100",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for Solana amount_usd > threshold, got %d: %s", w.Code, w.Body.String())
	}
	resp := respJSON(w)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected pending_approval, got %v", resp["status"])
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
	if resp["pending"] != true {
		t.Errorf("expected pending=true, got %v", resp["pending"])
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

func TestSign_USDThreshold_AboveThreshold_PendingApproval(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(2000, 100) // ETH = $2000

	// threshold $100
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "100", Enabled: true,
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, nil, "http://localhost")
	wh.SetPriceService(ps)
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("authMode", "passkey")
		c.Next()
	})
	r.POST("/wallets/:id/sign", wh.Sign)

	// Sign with tx_context indicating 1 ETH = $2000 > $100
	body := jsonBody(map[string]interface{}{
		"message":  "deadbeef",
		"encoding": "hex",
		"tx_context": map[string]interface{}{
			"amount":   "1",
			"currency": "ETH",
		},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/sign", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for Sign with amount > threshold, got %d: %s", w.Code, w.Body.String())
	}
	resp := respJSON(w)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected pending_approval, got %v", resp["status"])
	}
}

func TestSign_USDThreshold_BelowThreshold_DirectSign(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(2000, 100) // ETH = $2000

	// threshold $5000 → 1 ETH = $2000 < $5000 → direct path
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdUSD: "5000", Enabled: true,
	})

	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, nil, "http://localhost")
	wh.SetPriceService(ps)
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Set("authMode", "passkey")
		c.Next()
	})
	r.POST("/wallets/:id/sign", wh.Sign)

	body := jsonBody(map[string]interface{}{
		"message":  "deadbeef",
		"encoding": "hex",
		"tx_context": map[string]interface{}{
			"amount":   "1",
			"currency": "ETH",
		},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/sign", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := respJSON(w)
	// Should NOT be pending_approval (sdk=nil → 502 at signing, but not approval)
	if resp["status"] == "pending_approval" {
		t.Error("should not trigger approval for $2000 < $5000 threshold")
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

	// Should have failed at signing (502)
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 signing failure, got %d: %s", w.Code, w.Body.String())
	}

	// Key assertion: daily_spent_usd should be rolled back to 0
	var pol model.ApprovalPolicy
	db.Where("wallet_id = ?", wallet.ID).First(&pol)
	if pol.DailySpentUSD != "0" && pol.DailySpentUSD != "0.00" {
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
	if pol.DailySpentUSD != "0" && pol.DailySpentUSD != "0.00" {
		t.Errorf("expected daily_spent_usd=0 (rolled back for approval path), got %s", pol.DailySpentUSD)
	}
}

func TestContractCall_NoAmountUSD_NoPolicy_PassesThrough(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWalletWithAddress(t, db)
	ps := makePS(2000, 100)

	db.Create(&model.AllowedContract{
		WalletID:        wallet.ID,
		ContractAddress: "0x0000000000000000000000000000000000000abc",
		AutoApprove:     true,
	})

	r := contractCallUSDRouter(t, db, user.ID, ps, "apikey")

	// No amount_usd, no policy → passes to signing (fails at sdk=nil → 502)
	body := jsonBody(map[string]interface{}{
		"contract": "0x0000000000000000000000000000000000000abc",
		"func_sig": "doSomething()",
		"args":     []interface{}{},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	resp := respJSON(w)
	if resp["status"] == "pending_approval" {
		t.Error("should not trigger approval when no amount_usd and no policy")
	}
}
