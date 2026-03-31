package handler_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	sdk "github.com/TEENet-io/teenet-sdk/go"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/handler"
	"github.com/TEENet-io/teenet-wallet/model"
)

// solTransferRouter wires a minimal gin router for Solana transfer tests.
func solTransferRouter(db *gorm.DB, userID uint, sdkClient *sdk.Client, solRPC string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, sdkClient, "http://localhost")
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", "passkey")
		c.Next()
	})
	r.POST("/wallets/:id/transfer", wh.Transfer)
	r.POST("/wallets/:id/wrap-sol", wh.WrapSOL)
	r.POST("/wallets/:id/unwrap-sol", wh.UnwrapSOL)
	return r
}

func seedSolWallet(t *testing.T, db *gorm.DB) (model.User, model.Wallet) {
	t.Helper()
	user, _ := seedWallet(t, db) // creates an ethereum wallet we don't use
	wallet := model.Wallet{
		UserID:   user.ID,
		Chain:    "solana-devnet",
		KeyName:  fmt.Sprintf("sol-k%d", user.ID),
		Address:  "11111111111111111111111111111111",
		Curve:    "ed25519",
		Protocol: "schnorr",
		Status:   "ready",
	}
	if err := db.Create(&wallet).Error; err != nil {
		t.Fatalf("create sol wallet: %v", err)
	}
	return user, wallet
}

// ─── SPL Token Transfer: whitelist gate ─────────────────────────────────────

func TestTransfer_SPL_NotWhitelisted(t *testing.T) {
	db := testDB(t)
	user, wallet := seedSolWallet(t, db)

	r := solTransferRouter(db, user.ID, nil, "")

	body := jsonBody(map[string]interface{}{
		"to":     "11111111111111111111111111111112",
		"amount": "10",
		"token": map[string]interface{}{
			"contract": "So11111111111111111111111111111111111111112",
			"symbol":   "wSOL",
			"decimals": 9,
		},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for unwhitelisted SPL token, got %d: %s", w.Code, w.Body.String())
	}
}

func TestTransfer_SPL_Whitelisted_FailsAtRPC(t *testing.T) {
	db := testDB(t)
	user, wallet := seedSolWallet(t, db)

	// Whitelist the token
	mintAddr := "So11111111111111111111111111111111111111112"
	db.Create(&model.AllowedContract{
		UserID: user.ID, Chain: wallet.Chain, ContractAddress: mintAddr, Symbol: "wSOL", Decimals: 9,
	})

	// Use mock SOL RPC that returns a blockhash
	solRPC := mockSOLRPCServer(t)
	origCfg := model.Chains["solana-devnet"]
	model.Chains["solana-devnet"] = model.ChainConfig{
		Name: "solana-devnet", Label: "Solana Devnet", Family: "solana",
		Currency: "SOL", Curve: "ed25519", Protocol: "schnorr",
		RPCURL: solRPC.URL,
	}
	t.Cleanup(func() { model.Chains["solana-devnet"] = origCfg })

	r := solTransferRouter(db, user.ID, nil, "")

	body := jsonBody(map[string]interface{}{
		"to":     "11111111111111111111111111111112",
		"amount": "10",
		"token": map[string]interface{}{
			"contract": mintAddr,
			"symbol":   "wSOL",
			"decimals": 9,
		},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should pass whitelist check but fail at TEE signing (nil sdk)
	// 502 = build tx or signing failure (not 403)
	if w.Code == http.StatusForbidden {
		t.Fatalf("got 403 even though token is whitelisted: %s", w.Body.String())
	}
}

func TestTransfer_SPL_InvalidAddress(t *testing.T) {
	db := testDB(t)
	user, wallet := seedSolWallet(t, db)

	mintAddr := "So11111111111111111111111111111111111111112"
	db.Create(&model.AllowedContract{
		UserID: user.ID, Chain: wallet.Chain, ContractAddress: mintAddr, Symbol: "wSOL", Decimals: 9,
	})

	r := solTransferRouter(db, user.ID, nil, "")

	body := jsonBody(map[string]interface{}{
		"to":     "INVALID!!!",
		"amount": "10",
		"token": map[string]interface{}{
			"contract": mintAddr,
			"symbol":   "wSOL",
			"decimals": 9,
		},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/transfer", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid address, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── Wrap SOL ───────────────────────────────────────────────────────────────

func TestWrapSOL_NonSolanaChain(t *testing.T) {
	db := testDB(t)
	user, ethWallet := seedWallet(t, db) // ethereum wallet

	r := solTransferRouter(db, user.ID, nil, "")

	body := jsonBody(map[string]interface{}{"amount": "0.1"})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/wrap-sol", ethWallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-solana chain, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWrapSOL_InvalidAmount(t *testing.T) {
	db := testDB(t)
	user, wallet := seedSolWallet(t, db)

	r := solTransferRouter(db, user.ID, nil, "")

	body := jsonBody(map[string]interface{}{"amount": "-1"})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/wrap-sol", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for negative amount, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWrapSOL_MissingAmount(t *testing.T) {
	db := testDB(t)
	user, wallet := seedSolWallet(t, db)

	r := solTransferRouter(db, user.ID, nil, "")

	body := jsonBody(map[string]interface{}{})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/wrap-sol", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing amount, got %d: %s", w.Code, w.Body.String())
	}
}

func TestWrapSOL_FailsAtRPC(t *testing.T) {
	db := testDB(t)
	user, wallet := seedSolWallet(t, db)

	solRPC := mockSOLRPCServer(t)
	origCfg := model.Chains["solana-devnet"]
	model.Chains["solana-devnet"] = model.ChainConfig{
		Name: "solana-devnet", Label: "Solana Devnet", Family: "solana",
		Currency: "SOL", Curve: "ed25519", Protocol: "schnorr",
		RPCURL: solRPC.URL,
	}
	t.Cleanup(func() { model.Chains["solana-devnet"] = origCfg })

	r := solTransferRouter(db, user.ID, nil, "")

	body := jsonBody(map[string]interface{}{"amount": "0.1"})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/wrap-sol", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should build tx OK but fail at TEE signing (nil sdk) → 502
	if w.Code == http.StatusBadRequest {
		t.Fatalf("should not get 400 with valid amount: %s", w.Body.String())
	}
}

func TestWrapSOL_WalletNotReady(t *testing.T) {
	db := testDB(t)
	user, wallet := seedSolWallet(t, db)
	db.Model(&wallet).Update("status", "creating")

	r := solTransferRouter(db, user.ID, nil, "")

	body := jsonBody(map[string]interface{}{"amount": "0.1"})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/wrap-sol", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for wallet not ready, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── Unwrap SOL ─────────────────────────────────────────────────────────────

func TestUnwrapSOL_NonSolanaChain(t *testing.T) {
	db := testDB(t)
	user, ethWallet := seedWallet(t, db) // ethereum wallet

	r := solTransferRouter(db, user.ID, nil, "")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/unwrap-sol", ethWallet.ID), jsonBody(map[string]interface{}{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-solana chain, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUnwrapSOL_FailsAtRPC(t *testing.T) {
	db := testDB(t)
	user, wallet := seedSolWallet(t, db)

	solRPC := mockSOLRPCServer(t)
	origCfg := model.Chains["solana-devnet"]
	model.Chains["solana-devnet"] = model.ChainConfig{
		Name: "solana-devnet", Label: "Solana Devnet", Family: "solana",
		Currency: "SOL", Curve: "ed25519", Protocol: "schnorr",
		RPCURL: solRPC.URL,
	}
	t.Cleanup(func() { model.Chains["solana-devnet"] = origCfg })

	r := solTransferRouter(db, user.ID, nil, "")

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/unwrap-sol", wallet.ID), jsonBody(map[string]interface{}{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should build tx OK but fail at TEE signing (nil sdk) → 502
	if w.Code == http.StatusBadRequest {
		t.Fatalf("should not get 400 with valid request: %s", w.Body.String())
	}
}
