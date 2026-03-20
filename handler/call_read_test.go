package handler_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/handler"
	"github.com/TEENet-io/teenet-wallet/model"
)

// callReadRouter wires a minimal gin router for CallRead tests.
func callReadRouter(db *gorm.DB, userID uint, rpcURL string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	if rpcURL != "" {
		if cfg, ok := model.Chains["ethereum"]; ok {
			cfg.RPCURL = rpcURL
			model.Chains["ethereum"] = cfg
		}
	}
	r := gin.New()
	h := handler.NewContractCallHandler(db, nil, "http://localhost")
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", "apikey")
		c.Next()
	})
	r.POST("/wallets/:id/call-read", h.CallRead)
	return r
}

func TestCallRead_Success(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)

	// Mock RPC that returns a uint256 value for eth_call.
	returnHex := "0x00000000000000000000000000000000000000000000000000000000000f4240"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0", "id": 1, "result": returnHex,
		})
	}))
	defer srv.Close()

	router := callReadRouter(db, user.ID, srv.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "balanceOf(address)",
		"args":     []interface{}{wallet.Address},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/call-read", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp["success"])
	}
	if resp["result"] == nil || resp["result"] == "" {
		t.Error("expected non-empty result")
	}
	if resp["method"] != "balanceOf" {
		t.Errorf("expected method=balanceOf, got %v", resp["method"])
	}
}

func TestCallRead_SolanaNotSupported(t *testing.T) {
	db := testDB(t)
	user := model.User{Username: "sol-cr"}
	db.Create(&user)
	wallet := model.Wallet{UserID: user.ID, Chain: "solana", KeyName: "k-sol-cr", Status: "ready"}
	db.Create(&wallet)

	router := callReadRouter(db, user.ID, "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "balanceOf(address)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/call-read", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for Solana, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCallRead_InvalidFuncSig(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": "0x"})
	}))
	defer srv.Close()

	router := callReadRouter(db, user.ID, srv.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "not valid",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/call-read", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid func_sig, got %d: %s", w.Code, w.Body.String())
	}
}
