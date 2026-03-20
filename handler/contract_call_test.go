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

// mockETHRPCServer returns a test HTTP server that responds to ETH JSON-RPC calls
// with plausible but fake values. This lets contract-call tests exercise the full
// handler flow (including BuildETHContractCallTx) without hitting a live node.
func mockETHRPCServer(t *testing.T) *httptest.Server {
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
			result = "0x1" // nonce = 1
		case "eth_gasPrice":
			result = "0x3B9ACA00" // 1 Gwei
		case "eth_maxPriorityFeePerGas":
			result = "0x3B9ACA00" // 1 Gwei
		case "eth_getBlockByNumber":
			result = map[string]interface{}{
				"baseFeePerGas": "0x3B9ACA00",
				"gasLimit":      "0x1C9C380",
				"number":        "0x1",
			}
		case "eth_chainId":
			result = "0x1" // mainnet
		case "eth_estimateGas":
			result = "0xEA60" // 60000 gas
		default:
			result = nil
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// contractCallRouter wires a minimal gin router for ContractCall tests.
// rpcURL overrides the "ethereum" chain's RPC endpoint for the duration of the test.
func contractCallRouter(db *gorm.DB, userID uint, authMode string, rpcURL string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	if rpcURL != "" {
		// Patch the in-memory chain registry so the handler uses our mock RPC.
		if cfg, ok := model.Chains["ethereum"]; ok {
			cfg.RPCURL = rpcURL
			model.Chains["ethereum"] = cfg
		}
	}
	r := gin.New()
	h := handler.NewContractCallHandler(db, nil, "http://localhost")
	injectUser := func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", authMode)
		c.Next()
	}
	r.Use(injectUser)
	r.POST("/wallets/:id/contract-call", h.ContractCall)
	r.POST("/wallets/:id/approve-token", h.ApproveToken)
	r.POST("/wallets/:id/revoke-approval", h.RevokeApproval)
	return r
}

// seedWalletWithContract creates a user, an ethereum wallet, and a whitelisted contract.
func seedWalletWithContract(t *testing.T, db *gorm.DB, allowedMethods string, autoApprove bool) (model.User, model.Wallet, model.AllowedContract) {
	t.Helper()
	user, wallet := seedWallet(t, db)
	contract := model.AllowedContract{
		WalletID:        wallet.ID,
		ContractAddress: "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		Symbol:          "USDC",
		Decimals:        6,
		AllowedMethods:  allowedMethods,
		AutoApprove:     autoApprove,
	}
	if err := db.Create(&contract).Error; err != nil {
		t.Fatalf("seed contract: %v", err)
	}
	return user, wallet, contract
}

// ─── TestContractCall_NotWhitelisted ─────────────────────────────────────────

func TestContractCall_NotWhitelisted(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	// No AllowedContract record created — contract is NOT whitelisted.

	r := contractCallRouter(db, user.ID, "passkey", "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"func_sig": "transfer(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-whitelisted contract, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── TestContractCall_MethodNotAllowed ────────────────────────────────────────

func TestContractCall_MethodNotAllowed(t *testing.T) {
	db := testDB(t)
	// Contract is whitelisted but only allows "transfer".
	user, wallet, _ := seedWalletWithContract(t, db, "transfer", true)

	r := contractCallRouter(db, user.ID, "passkey", "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "approve(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for disallowed method, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] == nil {
		t.Error("expected error message in response")
	}
}

// ─── TestContractCall_HighRiskForceApproval_APIKey ────────────────────────────

func TestContractCall_HighRiskForceApproval_APIKey(t *testing.T) {
	db := testDB(t)
	// AutoApprove=true but method is high-risk → must still require approval via API Key.
	user, wallet, _ := seedWalletWithContract(t, db, "", true /* autoApprove */)

	// Start a mock RPC so BuildETHContractCallTx can complete.
	rpc := mockETHRPCServer(t)

	r := contractCallRouter(db, user.ID, "apikey", rpc.URL) // API key auth
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "approve(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 pending approval for high-risk method via API Key, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected status=pending_approval, got %v", resp["status"])
	}
	if resp["approval_id"] == nil {
		t.Error("expected approval_id in response")
	}

	// Verify an ApprovalRequest was created in the DB with ETHTxParams-compatible TxParams.
	var count int64
	db.Model(&model.ApprovalRequest{}).Where("wallet_id = ? AND approval_type = ?", wallet.ID, "contract_call").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 approval request in DB, got %d", count)
	}

	// Verify TxParams stores ETHTxParams (has "to" field, not "contract" field).
	var ar model.ApprovalRequest
	db.Where("wallet_id = ? AND approval_type = ?", wallet.ID, "contract_call").First(&ar)
	var ethParams map[string]interface{}
	if err := json.Unmarshal([]byte(ar.TxParams), &ethParams); err != nil {
		t.Fatalf("TxParams is not valid JSON: %v", err)
	}
	if ethParams["to"] == nil {
		t.Error("TxParams missing 'to' field — not ETHTxParams format")
	}
	if ethParams["contract"] != nil {
		t.Error("TxParams has 'contract' field — still using old custom format")
	}
	if ar.Message == "" {
		t.Error("approval Message should be the signing hash hex (non-empty)")
	}
}

// ─── TestContractCall_ApprovalStoresETHTxParams ───────────────────────────────

func TestContractCall_ApprovalStoresETHTxParams(t *testing.T) {
	// Verify that the approval stores TxParams in ETHTxParams format
	// (with "to", "gas_price", "nonce" fields), not a custom format.
	// This ensures approval.go can rebuild the tx on approve.
	db := testDB(t)
	// AutoApprove=false so every API Key call triggers approval path.
	user, wallet, _ := seedWalletWithContract(t, db, "", false /* autoApprove */)

	rpc := mockETHRPCServer(t)

	r := contractCallRouter(db, user.ID, "apikey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "transfer(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "500"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 pending approval, got %d: %s", w.Code, w.Body.String())
	}

	// Load the ApprovalRequest from DB.
	var ar model.ApprovalRequest
	if err := db.Where("wallet_id = ? AND approval_type = ?", wallet.ID, "contract_call").First(&ar).Error; err != nil {
		t.Fatalf("no approval request found in DB: %v", err)
	}

	// Verify approval_type == "contract_call".
	if ar.ApprovalType != "contract_call" {
		t.Errorf("expected ApprovalType='contract_call', got %q", ar.ApprovalType)
	}

	// Verify TxParams contains "to" field (ETHTxParams format).
	var ethParams map[string]interface{}
	if err := json.Unmarshal([]byte(ar.TxParams), &ethParams); err != nil {
		t.Fatalf("TxParams is not valid JSON: %v", err)
	}
	if ethParams["to"] == nil {
		t.Error("TxParams missing 'to' field — not ETHTxParams format")
	}

	// Verify Message is non-empty (signing hash hex).
	if ar.Message == "" {
		t.Error("approval Message should be the signing hash hex (non-empty)")
	}

	// Verify TxParams contains "data" field (calldata).
	if ethParams["data"] == nil {
		t.Error("TxParams missing 'data' field — calldata not stored")
	}
}

// ─── TestContractCall_AutoApproveFalse_APIKey ─────────────────────────────────

func TestContractCall_AutoApproveFalse_APIKey(t *testing.T) {
	db := testDB(t)
	// Non-high-risk method but AutoApprove=false → API Key should get 202.
	user, wallet, _ := seedWalletWithContract(t, db, "", false /* autoApprove */)

	rpc := mockETHRPCServer(t)

	r := contractCallRouter(db, user.ID, "apikey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "transfer(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 pending approval when AutoApprove=false via API Key, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected status=pending_approval, got %v", resp["status"])
	}
}

// ─── TestContractCall_SolanaNotSupported ──────────────────────────────────────

func TestContractCall_SolanaNotSupported(t *testing.T) {
	db := testDB(t)
	n := dbCounter // use current counter for a unique key name
	_ = n
	user := model.User{Username: "soluser-cc"}
	db.Create(&user)
	wallet := model.Wallet{
		UserID:  user.ID,
		Chain:   "solana",
		KeyName: fmt.Sprintf("k-sol-cc-%d", n),
		Status:  "ready",
	}
	db.Create(&wallet)

	r := contractCallRouter(db, user.ID, "passkey", "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "transfer(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for Solana chain, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── TestContractCall_WalletNotReady ──────────────────────────────────────────

func TestContractCall_WalletNotReady(t *testing.T) {
	db := testDB(t)
	user := model.User{Username: "notready-cc"}
	db.Create(&user)
	wallet := model.Wallet{
		UserID:  user.ID,
		Chain:   "ethereum",
		KeyName: "k-notready-cc",
		Status:  "creating",
	}
	db.Create(&wallet)

	r := contractCallRouter(db, user.ID, "passkey", "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "transfer(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-ready wallet, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── TestApproveToken_NotWhitelisted ──────────────────────────────────────────

func TestApproveToken_NotWhitelisted(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	// No AllowedContract record created — contract is NOT whitelisted.

	r := contractCallRouter(db, user.ID, "passkey", "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"spender":  "0x1234567890123456789012345678901234567890",
		"amount":   "100",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/approve-token", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-whitelisted contract, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── TestApproveToken_APIKey_PendingApproval ───────────────────────────────────

func TestApproveToken_APIKey_PendingApproval(t *testing.T) {
	db := testDB(t)
	// AutoApprove=true but approve is always high-risk → must still require approval via API Key.
	user, wallet, _ := seedWalletWithContract(t, db, "", true /* autoApprove */)

	rpc := mockETHRPCServer(t)

	r := contractCallRouter(db, user.ID, "apikey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"spender":  "0x1234567890123456789012345678901234567890",
		"amount":   "100",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/approve-token", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 pending approval for approve-token via API Key, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected status=pending_approval, got %v", resp["status"])
	}
	if resp["approval_id"] == nil {
		t.Error("expected approval_id in response")
	}

	// Verify an ApprovalRequest was created in the DB with type="contract_call".
	var count int64
	db.Model(&model.ApprovalRequest{}).Where("wallet_id = ? AND approval_type = ?", wallet.ID, "contract_call").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 approval request in DB, got %d", count)
	}
}

// ─── TestApproveToken_MissingFields ───────────────────────────────────────────

func TestApproveToken_MissingFields(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)

	r := contractCallRouter(db, user.ID, "passkey", "")
	// Missing spender field.
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"amount":   "100",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/approve-token", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing spender, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── TestRevokeApproval_APIKey_PendingApproval ─────────────────────────────────

func TestRevokeApproval_APIKey_PendingApproval(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db, "", true /* autoApprove */)

	rpc := mockETHRPCServer(t)

	r := contractCallRouter(db, user.ID, "apikey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"spender":  "0x1234567890123456789012345678901234567890",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/revoke-approval", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 pending approval for revoke-approval via API Key, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected status=pending_approval, got %v", resp["status"])
	}
	if resp["approval_id"] == nil {
		t.Error("expected approval_id in response")
	}

	// Verify an ApprovalRequest was created in the DB with type="contract_call".
	var count int64
	db.Model(&model.ApprovalRequest{}).Where("wallet_id = ? AND approval_type = ?", wallet.ID, "contract_call").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 approval request in DB, got %d", count)
	}
}

// ─── TestRevokeApproval_MissingFields ─────────────────────────────────────────

func TestRevokeApproval_MissingFields(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)

	r := contractCallRouter(db, user.ID, "passkey", "")
	// Missing contract field.
	body := jsonBody(map[string]interface{}{
		"spender": "0x1234567890123456789012345678901234567890",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/revoke-approval", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing contract, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── TestContractCall_InvalidFuncSig ─────────────────────────────────────────

func TestContractCall_InvalidFuncSig(t *testing.T) {
	db := testDB(t)
	// Whitelist the contract first so we pass layer 1.
	user, wallet, _ := seedWalletWithContract(t, db, "", true)

	r := contractCallRouter(db, user.ID, "passkey", "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "notavalidsignature", // missing parentheses
		"args":     []interface{}{},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%d/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid func_sig, got %d: %s", w.Code, w.Body.String())
	}
}
