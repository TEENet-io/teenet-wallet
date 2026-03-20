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
		WalletID: wallet.ID, ContractAddress: contractAddr, Symbol: "USDC", Decimals: 6,
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
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 (RPC not configured), got %d: %s", w.Code, w.Body.String())
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
	if w.Code != http.StatusBadGateway {
		t.Fatalf("expected 502 (RPC not configured), got %d: %s", w.Code, w.Body.String())
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
