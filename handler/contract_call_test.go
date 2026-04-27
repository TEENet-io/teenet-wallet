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
		case "eth_getBalance":
			// 1000 ETH — large enough to satisfy any pre-flight balance check
			// the wallet handler does before approval/policy logic kicks in.
			result = "0x3635c9adc5dea00000"
		default:
			result = nil
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	t.Cleanup(srv.Close)
	return srv
}

func mockETHRPCServerEstimateRevert(t *testing.T, revertMsg string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		method, _ := req["method"].(string)
		w.Header().Set("Content-Type", "application/json")
		switch method {
		case "eth_getTransactionCount":
			json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": "0x1"})
		case "eth_gasPrice":
			json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": "0x3B9ACA00"})
		case "eth_maxPriorityFeePerGas":
			json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": "0x3B9ACA00"})
		case "eth_getBlockByNumber":
			json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": map[string]interface{}{"baseFeePerGas": "0x3B9ACA00", "gasLimit": "0x1C9C380", "number": "0x1"}})
		case "eth_chainId":
			json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": "0x1"})
		case "eth_estimateGas":
			json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "error": map[string]interface{}{"code": 3, "message": "execution reverted: " + revertMsg}})
		case "eth_getBalance":
			json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": "0x3635c9adc5dea00000"}) // 1000 ETH
		default:
			json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": nil})
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// mockSOLRPCServer returns a test HTTP server that responds to Solana JSON-RPC calls
// with plausible but fake values. This lets Solana contract-call tests exercise the
// handler flow (including BuildSOLProgramCallTx) without hitting a live node.
func mockSOLRPCServer(t *testing.T) *httptest.Server {
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
		case "getLatestBlockhash":
			result = map[string]interface{}{
				"context": map[string]interface{}{"slot": 1234},
				"value": map[string]interface{}{
					"blockhash":            "11111111111111111111111111111111",
					"lastValidBlockHeight": 9999,
				},
			}
		case "getBalance":
			// 1000 SOL = 1e12 lamports — sufficient for any pre-flight balance check.
			result = map[string]interface{}{
				"context": map[string]interface{}{"slot": 1234},
				"value":   1000000000000,
			}
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
func contractCallRouter(t *testing.T, db *gorm.DB, userID uint, authMode string, rpcURL string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	if rpcURL != "" {
		// Patch the in-memory chain registry so the handler uses our mock RPC.
		// Save the original config and restore it after the test completes.
		if cfg, ok := model.GetChain("ethereum"); ok {
			original := cfg
			t.Cleanup(func() { model.SetChain("ethereum", original) })
			cfg.RPCURL = rpcURL
			model.SetChain("ethereum", cfg)
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
func seedWalletWithContract(t *testing.T, db *gorm.DB) (model.User, model.Wallet, model.AllowedContract) {
	t.Helper()
	user, wallet := seedWallet(t, db)
	contract := model.AllowedContract{
		UserID:          wallet.UserID,
		Chain:           wallet.Chain,
		ContractAddress: "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		Symbol:          "USDC",
		Decimals:        6,
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

	r := contractCallRouter(t, db, user.ID, "passkey", "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"func_sig": "transfer(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-whitelisted contract, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── TestContractCall_APIKey_RequiresApproval ─────────────────────────────────

func TestContractCall_APIKey_RequiresApproval(t *testing.T) {
	db := testDB(t)
	// All contract operations require approval via API Key.
	user, wallet, _ := seedWalletWithContract(t, db)

	// Start a mock RPC so BuildETHContractCallTx can complete.
	rpc := mockETHRPCServer(t)

	r := contractCallRouter(t, db, user.ID, "apikey", rpc.URL) // API key auth
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "approve(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 pending approval for contract call via API Key, got %d: %s", w.Code, w.Body.String())
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
	// All API Key calls trigger approval path.
	user, wallet, _ := seedWalletWithContract(t, db)

	rpc := mockETHRPCServer(t)

	r := contractCallRouter(t, db, user.ID, "apikey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "transfer(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "500"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
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

// ─── TestContractCall_SolanaInvalidProgramID ──────────────────────────────────

func TestContractCall_SolanaInvalidProgramID(t *testing.T) {
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

	r := contractCallRouter(t, db, user.ID, "passkey", "")
	// Send an EVM address as program ID — should fail base58 validation.
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"accounts": []map[string]interface{}{
			{"pubkey": "11111111111111111111111111111111", "is_signer": true, "is_writable": true},
		},
		"data": "01",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid Solana program ID, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── TestContractCall_SolanaNotWhitelisted ────────────────────────────────────

func TestContractCall_SolanaNotWhitelisted(t *testing.T) {
	db := testDB(t)
	n := dbCounter
	_ = n
	user := model.User{Username: "soluser-cc-nw"}
	db.Create(&user)
	wallet := model.Wallet{
		UserID:  user.ID,
		Chain:   "solana",
		KeyName: fmt.Sprintf("k-sol-cc-nw-%d", n),
		Status:  "ready",
	}
	db.Create(&wallet)

	r := contractCallRouter(t, db, user.ID, "passkey", "")
	// Valid base58 program ID, but not whitelisted.
	body := jsonBody(map[string]interface{}{
		"contract": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
		"accounts": []map[string]interface{}{
			{"pubkey": "11111111111111111111111111111111", "is_signer": true, "is_writable": true},
		},
		"data": "03",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-whitelisted program, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── TestContractCall_SolanaNoAccounts ─────────────────────────────────────────

func TestContractCall_SolanaNoAccounts(t *testing.T) {
	db := testDB(t)
	n := dbCounter
	_ = n
	user := model.User{Username: "soluser-cc-noacc"}
	db.Create(&user)
	wallet := model.Wallet{
		UserID:  user.ID,
		Chain:   "solana",
		KeyName: fmt.Sprintf("k-sol-cc-noacc-%d", n),
		Status:  "ready",
	}
	db.Create(&wallet)

	programID := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	contract := model.AllowedContract{
		UserID:          user.ID,
		Chain:           wallet.Chain,
		ContractAddress: programID,
	}
	db.Create(&contract)

	r := contractCallRouter(t, db, user.ID, "passkey", "")
	body := jsonBody(map[string]interface{}{
		"contract": programID,
		"accounts": []map[string]interface{}{},
		"data":     "03",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for no accounts, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── TestContractCall_Solana_APIKey_RequiresApproval ───────────────────────────

func TestContractCall_Solana_APIKey_RequiresApproval(t *testing.T) {
	db := testDB(t)
	n := dbCounter
	_ = n
	user := model.User{Username: "soluser-cc-hr"}
	db.Create(&user)
	wallet := model.Wallet{
		UserID:  user.ID,
		Chain:   "solana",
		KeyName: fmt.Sprintf("k-sol-cc-hr-%d", n),
		Address: "11111111111111111111111111111111", // valid 32-byte base58 address
		Status:  "ready",
	}
	db.Create(&wallet)

	programID := "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA"
	contract := model.AllowedContract{
		UserID:          user.ID,
		Chain:           wallet.Chain,
		ContractAddress: programID,
	}
	db.Create(&contract)

	// Patch solana chain RPC to our mock so BuildSOLProgramCallTx can complete.
	rpc := mockSOLRPCServer(t)
	if cfg, ok := model.GetChain("solana"); ok {
		cfg.RPCURL = rpc.URL
		model.SetChain("solana", cfg)
	}

	r := contractCallRouter(t, db, user.ID, "apikey", "")
	// Any Solana contract call via API Key requires approval.
	body := jsonBody(map[string]interface{}{
		"contract": programID,
		"accounts": []map[string]interface{}{
			{"pubkey": "11111111111111111111111111111111", "is_signer": true, "is_writable": true},
		},
		"data": "0400000000000000",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 pending approval for Solana contract call via API Key, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected status=pending_approval, got %v", resp["status"])
	}
	if resp["approval_id"] == nil {
		t.Error("expected approval_id in response")
	}

	// Verify an ApprovalRequest was created in the DB.
	var count int64
	db.Model(&model.ApprovalRequest{}).Where("wallet_id = ? AND approval_type = ?", wallet.ID, "contract_call").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 approval request in DB, got %d", count)
	}
}

// ─── TestContractCall_EVMRequiresFuncSig ──────────────────────────────────────

func TestContractCall_EVMRequiresFuncSig(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db)

	r := contractCallRouter(t, db, user.ID, "passkey", "")
	// Missing func_sig — should fail for EVM.
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing func_sig on EVM, got %d: %s", w.Code, w.Body.String())
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

	r := contractCallRouter(t, db, user.ID, "passkey", "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "transfer(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
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

	r := contractCallRouter(t, db, user.ID, "passkey", "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
		"spender":  "0x1234567890123456789012345678901234567890",
		"amount":   "100",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/approve-token", wallet.ID), body)
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
	// All contract operations require approval via API Key.
	user, wallet, _ := seedWalletWithContract(t, db)

	rpc := mockETHRPCServer(t)

	r := contractCallRouter(t, db, user.ID, "apikey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"spender":  "0x1234567890123456789012345678901234567890",
		"amount":   "100",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/approve-token", wallet.ID), body)
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

	r := contractCallRouter(t, db, user.ID, "passkey", "")
	// Missing spender field.
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"amount":   "100",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/approve-token", wallet.ID), body)
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
	user, wallet, _ := seedWalletWithContract(t, db)

	rpc := mockETHRPCServer(t)

	r := contractCallRouter(t, db, user.ID, "apikey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"spender":  "0x1234567890123456789012345678901234567890",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/revoke-approval", wallet.ID), body)
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

	r := contractCallRouter(t, db, user.ID, "passkey", "")
	// Missing contract field.
	body := jsonBody(map[string]interface{}{
		"spender": "0x1234567890123456789012345678901234567890",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/revoke-approval", wallet.ID), body)
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
	user, wallet, _ := seedWalletWithContract(t, db)

	r := contractCallRouter(t, db, user.ID, "passkey", "")
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "notavalidsignature", // missing parentheses
		"args":     []interface{}{},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid func_sig, got %d: %s", w.Code, w.Body.String())
	}
}

// TestContractCall_UnreachableRPC_DoesNotLeakProviderToken guards
// respondContractCallStageError against leaking the RPC URL (and any
// provider token in its path) when eth_estimateGas fails at transport
// level. Pairs with TestTransfer_ERC20_UnreachableRPC_DoesNotLeakProviderToken
// to cover the two main EVM error paths.
func TestContractCall_UnreachableRPC_DoesNotLeakProviderToken(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db)

	// Bind a server just to reserve a port, then close so requests get a
	// transport-level "connection refused" error carrying the full URL.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	baseURL := srv.URL
	srv.Close()
	const secretToken = "SECRET_QN_TOKEN_CC" //nolint:gosec // synthetic test value
	fakeRPC := baseURL + "/" + secretToken + "/v1/"

	r := contractCallRouter(t, db, user.ID, "passkey", fakeRPC)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "approve(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
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
	if strings.Contains(w.Body.String(), secretToken) {
		t.Fatalf("response body leaked provider token %q: %s", secretToken, w.Body.String())
	}
	rpcError, _ := resp["rpc_error"].(string)
	if rpcError == "" {
		t.Fatalf("rpc_error field missing or empty: %s", w.Body.String())
	}
	if !strings.Contains(strings.ToLower(rpcError), "connection refused") &&
		!strings.Contains(strings.ToLower(rpcError), "connect") {
		t.Fatalf("rpc_error lost its diagnostic tail, got: %s", rpcError)
	}
}

func TestContractCall_EstimateGasRevert_PropagatesReason(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db)
	rpc := mockETHRPCServerEstimateRevert(t, "Too little received")
	r := contractCallRouter(t, db, user.ID, "passkey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "approve(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for estimateGas revert, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Too little received") {
		t.Fatalf("expected revert reason in response, got: %s", w.Body.String())
	}
}

// ─── TestContractCall_PasskeyAuth_DirectExecution ─────────────────────────────

func TestContractCall_PasskeyAuth_DirectExecution(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db)
	rpc := mockETHRPCServer(t)
	r := contractCallRouter(t, db, user.ID, "passkey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"func_sig": "approve(address,uint256)",
		"args":     []interface{}{"0x1234567890123456789012345678901234567890", "1000000"},
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contract-call", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusAccepted {
		t.Fatalf("Passkey should skip approval, got 202: %s", w.Body.String())
	}
}

// ─── TestApproveToken_PasskeyAuth_DirectExecution ─────────────────────────────

func TestApproveToken_PasskeyAuth_DirectExecution(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db)
	rpc := mockETHRPCServer(t)
	// Passkey auth — should bypass approval even for approve-token
	r := contractCallRouter(t, db, user.ID, "passkey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"spender":  "0x1234567890123456789012345678901234567890",
		"amount":   "1000000",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/approve-token", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// Passkey goes to signing directly. SDK nil → 502, NOT 202.
	if w.Code == http.StatusAccepted {
		t.Fatalf("Passkey approve-token should skip approval, got 202: %s", w.Body.String())
	}
}

// ─── TestRevokeApproval_PasskeyAuth_DirectExecution ───────────────────────────

func TestRevokeApproval_PasskeyAuth_DirectExecution(t *testing.T) {
	db := testDB(t)
	user, wallet, _ := seedWalletWithContract(t, db)
	rpc := mockETHRPCServer(t)
	r := contractCallRouter(t, db, user.ID, "passkey", rpc.URL)
	body := jsonBody(map[string]interface{}{
		"contract": "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48",
		"spender":  "0x1234567890123456789012345678901234567890",
	})
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/revoke-approval", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code == http.StatusAccepted {
		t.Fatalf("Passkey revoke-approval should skip approval, got 202: %s", w.Body.String())
	}
}
