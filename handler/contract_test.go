package handler_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/TEENet-io/teenet-wallet/handler"
	"github.com/TEENet-io/teenet-wallet/model"
)

// TestMain initialises shared state before any test in this package runs.
// model.Chains must be populated or transfer/policy tests will see "unsupported chain".
func TestMain(m *testing.M) {
	// Use built-in defaults (LoadChains falls back when the path doesn't exist).
	model.LoadChains("")
	os.Exit(m.Run())
}

// ─── helpers ─────────────────────────────────────────────────────────────────

var dbCounter int64

// testDB creates a fresh in-memory SQLite DB isolated per test.
func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	n := atomic.AddInt64(&dbCounter, 1)
	dsn := fmt.Sprintf("file:testdb%d?mode=memory&cache=shared", n)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	if err := db.AutoMigrate(
		&model.User{},
		&model.APIKey{},
		&model.Wallet{},
		&model.AllowedContract{},
		&model.ApprovalPolicy{},
		&model.ApprovalRequest{},
		&model.AuditLog{},
		&model.AddressBookEntry{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedWallet(t *testing.T, db *gorm.DB) (model.User, model.Wallet) {
	t.Helper()
	n := atomic.AddInt64(&dbCounter, 1)
	user := model.User{
		Username:      fmt.Sprintf("u%d", n),
		PasskeyUserID: uint(n), // must be unique per row
	}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	wallet := model.Wallet{
		UserID:  user.ID,
		Chain:   "ethereum",
		KeyName: fmt.Sprintf("k%d", n),
		Status:  "ready",
	}
	if err := db.Create(&wallet).Error; err != nil {
		t.Fatalf("create wallet: %v", err)
	}
	return user, wallet
}

// contractRouter returns a test gin.Engine with contract routes mounted under
// a middleware that injects the given userID.
func contractRouter(db *gorm.DB, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ch := handler.NewContractHandler(db, nil, "")

	injectUser := func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", "passkey")
		c.Next()
	}
	r.Use(injectUser)
	r.GET("/wallets/:id/contracts", ch.ListContracts)
	r.POST("/wallets/:id/contracts", ch.AddContract)
	r.DELETE("/wallets/:id/contracts/:cid", ch.DeleteContract)
	return r
}

func jsonBody(v interface{}) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// ─── AddContract ──────────────────────────────────────────────────────────────

func TestAddContract_Success(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := contractRouter(db, user.ID)

	body := map[string]interface{}{
		"contract_address": "0x1234567890123456789012345678901234567890",
		"symbol":           "USDC",
		"decimals":         6,
		"label":            "Circle USD Coin",
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contracts", wallet.ID), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["success"] != true {
		t.Fatalf("expected success=true, got %v", resp)
	}
	contract, _ := resp["contract"].(map[string]interface{})
	if contract["symbol"] != "USDC" {
		t.Errorf("expected symbol USDC, got %v", contract["symbol"])
	}
	if contract["decimals"] != float64(6) {
		t.Errorf("expected decimals 6, got %v", contract["decimals"])
	}

	// Verify stored in DB.
	var count int64
	db.Model(&model.AllowedContract{}).Where("user_id = ? AND chain = ?", user.ID, wallet.Chain).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 contract in DB, got %d", count)
	}
}

func TestAddContract_NormalizesAddressToLowercase(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := contractRouter(db, user.ID)

	body := map[string]interface{}{
		"contract_address": "0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48", // mixed case USDC
		"symbol":           "USDC",
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contracts", wallet.ID), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var stored model.AllowedContract
	db.Where("user_id = ? AND chain = ?", user.ID, wallet.Chain).First(&stored)
	if stored.ContractAddress != "0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48" {
		t.Errorf("expected lowercase address, got %q", stored.ContractAddress)
	}
}

func TestAddContract_Duplicate(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := contractRouter(db, user.ID)

	body := map[string]interface{}{
		"contract_address": "0x1234567890123456789012345678901234567890",
		"symbol":           "USDC",
	}
	// First add.
	req1 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contracts", wallet.ID), jsonBody(body))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req1)

	// Second add (duplicate).
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contracts", wallet.ID), jsonBody(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestAddContract_InvalidAddress_TooShort(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := contractRouter(db, user.ID)

	body := map[string]interface{}{
		"contract_address": "0x1234",
		"symbol":           "BAD",
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contracts", wallet.ID), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddContract_InvalidAddress_NoPrefix(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := contractRouter(db, user.ID)

	body := map[string]interface{}{
		// valid length but no 0x prefix
		"contract_address": "1234567890123456789012345678901234567890",
		"symbol":           "BAD",
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contracts", wallet.ID), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestAddContract_WrongWallet(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	_, otherWallet := seedWallet(t, db) // belongs to a different user

	r := contractRouter(db, user.ID) // authenticated as user, not otherWallet's owner

	body := map[string]interface{}{
		"contract_address": "0x1234567890123456789012345678901234567890",
		"symbol":           "USDC",
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contracts", otherWallet.ID), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for wrong wallet owner, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── ListContracts ────────────────────────────────────────────────────────────

func TestListContracts_Empty(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := contractRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/contracts", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	contracts, _ := resp["contracts"].([]interface{})
	if len(contracts) != 0 {
		t.Errorf("expected empty list, got %d contracts", len(contracts))
	}
}

func TestListContracts_WithEntries(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)

	// Seed contracts directly.
	contracts := []model.AllowedContract{
		{UserID: user.ID, Chain: wallet.Chain, ContractAddress: "0x1111111111111111111111111111111111111111", Symbol: "USDC", Decimals: 6},
		{UserID: user.ID, Chain: wallet.Chain, ContractAddress: "0x2222222222222222222222222222222222222222", Symbol: "WETH", Decimals: 18},
	}
	for i := range contracts {
		db.Create(&contracts[i])
	}

	r := contractRouter(db, user.ID)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/contracts", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	list, _ := resp["contracts"].([]interface{})
	if len(list) != 2 {
		t.Errorf("expected 2 contracts, got %d", len(list))
	}
}

func TestListContracts_DoesNotLeakOtherWallet(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	_, otherWallet := seedWallet(t, db)

	// Contract on the other user's chain — should NOT appear in user's list.
	db.Create(&model.AllowedContract{
		UserID:          otherWallet.UserID,
		Chain:           otherWallet.Chain,
		ContractAddress: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Symbol:          "OTHER",
	})

	r := contractRouter(db, user.ID)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/contracts", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	list, _ := resp["contracts"].([]interface{})
	if len(list) != 0 {
		t.Errorf("expected 0 contracts for user's wallet, got %d (leaked from other wallet?)", len(list))
	}
}

// ─── DeleteContract ───────────────────────────────────────────────────────────

func TestDeleteContract_Success(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)

	contract := model.AllowedContract{
		UserID:          user.ID,
		Chain:           wallet.Chain,
		ContractAddress: "0x1234567890123456789012345678901234567890",
		Symbol:          "USDC",
	}
	db.Create(&contract)

	r := contractRouter(db, user.ID)
	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/wallets/%s/contracts/%d", wallet.ID, contract.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify removed from DB.
	var count int64
	db.Model(&model.AllowedContract{}).Where("id = ?", contract.ID).Count(&count)
	if count != 0 {
		t.Error("expected contract to be deleted from DB")
	}
}

func TestDeleteContract_NotFound(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := contractRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/wallets/%s/contracts/99999", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestDeleteContract_WrongWallet(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	_, otherWallet := seedWallet(t, db)

	contract := model.AllowedContract{
		UserID:          otherWallet.UserID,
		Chain:           otherWallet.Chain,
		ContractAddress: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Symbol:          "EVIL",
	}
	db.Create(&contract)

	// Authenticated as user (not otherWallet's owner), trying to delete otherWallet's contract.
	r := contractRouter(db, user.ID)
	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/wallets/%s/contracts/%d", otherWallet.ID, contract.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// 404 because wallet belongs to another user (combined user_id+id query).
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
	var count int64
	db.Model(&model.AllowedContract{}).Where("id = ?", contract.ID).Count(&count)
	if count != 1 {
		t.Error("contract should NOT have been deleted")
	}
}

func TestDeleteContract_InvalidID(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := contractRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodDelete,
		fmt.Sprintf("/wallets/%s/contracts/notanumber", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

// ─── Per-chain whitelist sharing ────────────────────────────────────────────

// seedSecondWallet creates a second wallet on the given chain for an existing user.
func seedSecondWallet(t *testing.T, db *gorm.DB, user model.User, chain string) model.Wallet {
	t.Helper()
	n := atomic.AddInt64(&dbCounter, 1)
	w := model.Wallet{
		UserID:  user.ID,
		Chain:   chain,
		KeyName: fmt.Sprintf("k-extra-%d", n),
		Label:   fmt.Sprintf("extra-%d", n),
		Status:  "ready",
	}
	if err := db.Create(&w).Error; err != nil {
		t.Fatalf("create second wallet: %v", err)
	}
	return w
}

func TestListContracts_SharedAcrossSameChainWallets(t *testing.T) {
	db := testDB(t)
	user, wallet1 := seedWallet(t, db) // ethereum
	wallet2 := seedSecondWallet(t, db, user, "ethereum")

	// Add a contract via wallet1.
	db.Create(&model.AllowedContract{
		UserID: user.ID, Chain: "ethereum",
		ContractAddress: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		Symbol: "USDC",
	})

	r := contractRouter(db, user.ID)

	// Query via wallet1 — should see the contract.
	req1 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/contracts", wallet1.ID), nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	var resp1 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	list1, _ := resp1["contracts"].([]interface{})
	if len(list1) != 1 {
		t.Errorf("wallet1: expected 1 contract, got %d", len(list1))
	}

	// Query via wallet2 (same user, same chain) — should also see the contract.
	req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/contracts", wallet2.ID), nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var resp2 map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	list2, _ := resp2["contracts"].([]interface{})
	if len(list2) != 1 {
		t.Errorf("wallet2: expected 1 shared contract, got %d", len(list2))
	}
}

func TestListContracts_NotSharedAcrossDifferentChains(t *testing.T) {
	db := testDB(t)
	user, ethWallet := seedWallet(t, db) // ethereum
	solWallet := seedSecondWallet(t, db, user, "solana")

	// Add a contract for ethereum only.
	db.Create(&model.AllowedContract{
		UserID: user.ID, Chain: "ethereum",
		ContractAddress: "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		Symbol: "WETH",
	})

	r := contractRouter(db, user.ID)

	// Ethereum wallet — should see it.
	req1 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/contracts", ethWallet.ID), nil)
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	var resp1 map[string]interface{}
	json.Unmarshal(w1.Body.Bytes(), &resp1)
	list1, _ := resp1["contracts"].([]interface{})
	if len(list1) != 1 {
		t.Errorf("eth wallet: expected 1 contract, got %d", len(list1))
	}

	// Solana wallet — should NOT see it.
	req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/contracts", solWallet.ID), nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var resp2 map[string]interface{}
	json.Unmarshal(w2.Body.Bytes(), &resp2)
	list2, _ := resp2["contracts"].([]interface{})
	if len(list2) != 0 {
		t.Errorf("sol wallet: expected 0 contracts, got %d (leaked across chains?)", len(list2))
	}
}

func TestAddContract_DuplicateAcrossWalletsSameChain(t *testing.T) {
	db := testDB(t)
	user, wallet1 := seedWallet(t, db) // ethereum
	wallet2 := seedSecondWallet(t, db, user, "ethereum")

	r := contractRouter(db, user.ID)
	body := map[string]interface{}{
		"contract_address": "0xcccccccccccccccccccccccccccccccccccccccc",
		"symbol":           "USDC",
	}

	// Add via wallet1 — should succeed.
	req1 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contracts", wallet1.ID), jsonBody(body))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("wallet1 add: expected 200, got %d: %s", w1.Code, w1.Body.String())
	}

	// Add same contract via wallet2 (same user, same chain) — should be 409 duplicate.
	req2 := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/wallets/%s/contracts", wallet2.ID), jsonBody(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusConflict {
		t.Fatalf("wallet2 add: expected 409 Conflict, got %d: %s", w2.Code, w2.Body.String())
	}
}

