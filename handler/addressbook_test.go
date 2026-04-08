// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

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

// addressbookRouter returns a test gin.Engine with address book routes mounted
// under a middleware that injects the given userID and passkey auth mode.
func addressbookRouter(db *gorm.DB, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewAddressBookHandler(db, nil, "")

	injectUser := func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", "passkey")
		c.Next()
	}
	r.Use(injectUser)
	r.GET("/addressbook", h.ListEntries)
	r.POST("/addressbook", h.AddEntry)
	r.PUT("/addressbook/:id", h.UpdateEntry)
	r.DELETE("/addressbook/:id", h.DeleteEntry)
	return r
}

// ─── ListEntries ─────────────────────────────────────────────────────────────

func TestListEntries_Empty(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/addressbook", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	entries, _ := resp["entries"].([]interface{})
	if len(entries) != 0 {
		t.Errorf("expected empty list, got %d entries", len(entries))
	}
}

func TestListEntries_WithFilter(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	// Seed entries directly.
	db.Create(&model.AddressBookEntry{
		UserID: user.ID, Nickname: "alice", Chain: "ethereum",
		Address: "0x1234567890123456789012345678901234567890",
	})
	db.Create(&model.AddressBookEntry{
		UserID: user.ID, Nickname: "bob", Chain: "solana",
		Address: "11111111111111111111111111111111",
	})

	r := addressbookRouter(db, user.ID)

	// Filter by chain=ethereum should return 1.
	req := httptest.NewRequest(http.MethodGet, "/addressbook?chain=ethereum", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	entries, _ := resp["entries"].([]interface{})
	if len(entries) != 1 {
		t.Errorf("expected 1 entry for ethereum filter, got %d", len(entries))
	}
}

// ─── AddEntry ────────────────────────────────────────────────────────────────

func TestAddEntry_Success(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouter(db, user.ID)

	body := map[string]interface{}{
		"nickname": "Alice",
		"chain":    "ethereum",
		"address":  "0x1234567890123456789012345678901234567890",
		"memo":     "My friend",
	}
	req := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body))
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
	entry, _ := resp["entry"].(map[string]interface{})
	if entry["nickname"] != "alice" {
		t.Errorf("expected nickname stored as 'alice', got %v", entry["nickname"])
	}

	// Verify stored in DB.
	var count int64
	db.Model(&model.AddressBookEntry{}).Where("user_id = ? AND nickname = ?", user.ID, "alice").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 entry in DB, got %d", count)
	}
}

func TestAddEntry_Duplicate(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouter(db, user.ID)

	body := map[string]interface{}{
		"nickname": "alice",
		"chain":    "ethereum",
		"address":  "0x1234567890123456789012345678901234567890",
	}

	// First add.
	req1 := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(httptest.NewRecorder(), req1)

	// Second add (duplicate).
	req2 := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestAddEntry_CaseInsensitive(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouter(db, user.ID)

	body1 := map[string]interface{}{
		"nickname": "Alice",
		"chain":    "ethereum",
		"address":  "0x1234567890123456789012345678901234567890",
	}
	req1 := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first add failed: %d: %s", w1.Code, w1.Body.String())
	}

	// Same nickname different case, same chain -> 409.
	body2 := map[string]interface{}{
		"nickname": "alice",
		"chain":    "ethereum",
		"address":  "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	req2 := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusConflict {
		t.Fatalf("expected 409 for case-insensitive duplicate, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestAddEntry_SameNicknameDifferentChain(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouter(db, user.ID)

	body1 := map[string]interface{}{
		"nickname": "alice",
		"chain":    "ethereum",
		"address":  "0x1234567890123456789012345678901234567890",
	}
	req1 := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body1))
	req1.Header.Set("Content-Type", "application/json")
	w1 := httptest.NewRecorder()
	r.ServeHTTP(w1, req1)
	if w1.Code != http.StatusOK {
		t.Fatalf("first add failed: %d: %s", w1.Code, w1.Body.String())
	}

	// Same nickname, different chain -> should succeed.
	body2 := map[string]interface{}{
		"nickname": "alice",
		"chain":    "solana",
		"address":  "11111111111111111111111111111111",
	}
	req2 := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("expected 200 for same nickname on different chain, got %d: %s", w2.Code, w2.Body.String())
	}
}

func TestAddEntry_InvalidAddress(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouter(db, user.ID)

	body := map[string]interface{}{
		"nickname": "badaddr",
		"chain":    "ethereum",
		"address":  "0xinvalid",
	}
	req := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid address, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddEntry_InvalidNickname(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouter(db, user.ID)

	body := map[string]interface{}{
		"nickname": "-invalid",
		"chain":    "ethereum",
		"address":  "0x1234567890123456789012345678901234567890",
	}
	req := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid nickname, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── ResolveNickname ─────────────────────────────────────────────────────────

func TestResolveNickname_Found(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	db.Create(&model.AddressBookEntry{
		UserID: user.ID, Nickname: "alice", Chain: "ethereum",
		Address: "0x1234567890123456789012345678901234567890",
	})

	addr, err := handler.ResolveNickname(db, user.ID, "alice", "ethereum")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if addr != "0x1234567890123456789012345678901234567890" {
		t.Errorf("expected stored address, got %q", addr)
	}
}

func TestResolveNickname_NotFound(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	_, err := handler.ResolveNickname(db, user.ID, "nobody", "ethereum")
	if err == nil {
		t.Fatal("expected error for missing nickname, got nil")
	}
}

// ─── LooksLikeAddress ────────────────────────────────────────────────────────

func TestLooksLikeAddress_EVM(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"0x1234567890123456789012345678901234567890", true},
		{"0x12345678901234567890123456789012345678", false},  // too short
		{"1234567890123456789012345678901234567890", false},  // no 0x prefix
		{"alice", false},
	}
	for _, tt := range tests {
		got := handler.LooksLikeAddress(tt.input, "evm")
		if got != tt.want {
			t.Errorf("LooksLikeAddress(%q, evm) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

// ─── UpdateEntry ────────────────────────────────────────────────────────────

func TestUpdateEntry_Success(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	// Create an entry directly.
	entry := model.AddressBookEntry{
		UserID: user.ID, Nickname: "alice", Chain: "ethereum",
		Address: "0x1234567890123456789012345678901234567890", Memo: "old memo",
	}
	db.Create(&entry)

	r := addressbookRouter(db, user.ID)

	body := map[string]interface{}{
		"address": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"memo":    "new memo",
	}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/addressbook/%d", entry.ID), jsonBody(body))
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

	// Verify DB updated.
	var updated model.AddressBookEntry
	db.First(&updated, entry.ID)
	if updated.Address != "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Errorf("expected updated address, got %q", updated.Address)
	}
	if updated.Memo != "new memo" {
		t.Errorf("expected updated memo, got %q", updated.Memo)
	}
}

func TestUpdateEntry_NicknameConflict(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	db.Create(&model.AddressBookEntry{
		UserID: user.ID, Nickname: "alice", Chain: "ethereum",
		Address: "0x1234567890123456789012345678901234567890",
	})
	bob := model.AddressBookEntry{
		UserID: user.ID, Nickname: "bob", Chain: "ethereum",
		Address: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	db.Create(&bob)

	r := addressbookRouter(db, user.ID)

	body := map[string]interface{}{
		"nickname": "alice",
	}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/addressbook/%d", bob.ID), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateEntry_OtherUser(t *testing.T) {
	db := testDB(t)
	user1, _ := seedWallet(t, db)
	user2, _ := seedWallet(t, db)

	entry := model.AddressBookEntry{
		UserID: user1.ID, Nickname: "alice", Chain: "ethereum",
		Address: "0x1234567890123456789012345678901234567890",
	}
	db.Create(&entry)

	// user2 tries to update user1's entry.
	r := addressbookRouter(db, user2.ID)

	body := map[string]interface{}{
		"memo": "hacked",
	}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/addressbook/%d", entry.ID), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── DeleteEntry ────────────────────────────────────────────────────────────

func TestDeleteEntry_Success(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	entry := model.AddressBookEntry{
		UserID: user.ID, Nickname: "alice", Chain: "ethereum",
		Address: "0x1234567890123456789012345678901234567890",
	}
	db.Create(&entry)

	r := addressbookRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/addressbook/%d", entry.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Verify gone from DB.
	var count int64
	db.Model(&model.AddressBookEntry{}).Where("id = ?", entry.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected entry deleted, but found %d rows", count)
	}
}

func TestDeleteEntry_OtherUser(t *testing.T) {
	db := testDB(t)
	user1, _ := seedWallet(t, db)
	user2, _ := seedWallet(t, db)

	entry := model.AddressBookEntry{
		UserID: user1.ID, Nickname: "alice", Chain: "ethereum",
		Address: "0x1234567890123456789012345678901234567890",
	}
	db.Create(&entry)

	// user2 tries to delete user1's entry.
	r := addressbookRouter(db, user2.ID)

	req := httptest.NewRequest(http.MethodDelete, fmt.Sprintf("/addressbook/%d", entry.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}

	// Verify entry still exists.
	var count int64
	db.Model(&model.AddressBookEntry{}).Where("id = ?", entry.ID).Count(&count)
	if count != 1 {
		t.Errorf("expected entry still present, found %d rows", count)
	}
}

// ─── LooksLikeAddress ────────────────────────────────────────────────────────

func TestAddEntry_UnsupportedChain(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouter(db, user.ID)

	body := map[string]interface{}{
		"nickname": "alice",
		"chain":    "nonexistent",
		"address":  "0x1234567890123456789012345678901234567890",
	}
	req := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unsupported chain, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddEntry_MemoTooLong(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouter(db, user.ID)

	longMemo := ""
	for i := 0; i < 260; i++ {
		longMemo += "a"
	}
	body := map[string]interface{}{
		"nickname": "alice",
		"chain":    "ethereum",
		"address":  "0x1234567890123456789012345678901234567890",
		"memo":     longMemo,
	}
	req := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for memo too long, got %d: %s", w.Code, w.Body.String())
	}
}

func TestResolveNickname_CaseInsensitive(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	db.Create(&model.AddressBookEntry{
		UserID: user.ID, Nickname: "alice", Chain: "ethereum",
		Address: "0x1234567890123456789012345678901234567890",
	})

	addr, err := handler.ResolveNickname(db, user.ID, "Alice", "ethereum")
	if err != nil {
		t.Fatalf("expected case-insensitive match, got error: %v", err)
	}
	if addr != "0x1234567890123456789012345678901234567890" {
		t.Errorf("unexpected address: %q", addr)
	}
}

func TestListEntries_NicknameFilter(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	db.Create(&model.AddressBookEntry{UserID: user.ID, Nickname: "alice", Chain: "ethereum", Address: "0x1234567890123456789012345678901234567890"})
	db.Create(&model.AddressBookEntry{UserID: user.ID, Nickname: "bob", Chain: "ethereum", Address: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"})

	r := addressbookRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/addressbook?nickname=alice", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	entries := resp["entries"].([]interface{})
	if len(entries) != 1 {
		t.Errorf("expected 1 entry for nickname filter, got %d", len(entries))
	}
}

func TestListEntries_CrossUserIsolation(t *testing.T) {
	db := testDB(t)
	user1, _ := seedWallet(t, db)
	user2, _ := seedWallet(t, db)

	db.Create(&model.AddressBookEntry{UserID: user1.ID, Nickname: "alice", Chain: "ethereum", Address: "0x1234567890123456789012345678901234567890"})

	r := addressbookRouter(db, user2.ID)

	req := httptest.NewRequest(http.MethodGet, "/addressbook", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	entries := resp["entries"].([]interface{})
	if len(entries) != 0 {
		t.Errorf("user2 should see 0 entries, got %d", len(entries))
	}
}

func TestUpdateEntry_NoFields(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	entry := model.AddressBookEntry{UserID: user.ID, Nickname: "alice", Chain: "ethereum", Address: "0x1234567890123456789012345678901234567890"}
	db.Create(&entry)

	r := addressbookRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/addressbook/%d", entry.ID), jsonBody(map[string]interface{}{}))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for no fields, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateEntry_InvalidAddress(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	entry := model.AddressBookEntry{UserID: user.ID, Nickname: "alice", Chain: "ethereum", Address: "0x1234567890123456789012345678901234567890"}
	db.Create(&entry)

	r := addressbookRouter(db, user.ID)

	body := map[string]interface{}{"address": "0xbad"}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/addressbook/%d", entry.ID), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid address, got %d: %s", w.Code, w.Body.String())
	}
}

func TestDeleteEntry_NotFound(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodDelete, "/addressbook/99999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent entry, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAddEntry_InvalidSolanaAddress(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouter(db, user.ID)

	body := map[string]interface{}{
		"nickname": "bob",
		"chain":    "solana",
		"address":  "not-a-solana-address",
	}
	req := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid solana address, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateEntry_MemoTooLong(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	entry := model.AddressBookEntry{UserID: user.ID, Nickname: "alice", Chain: "ethereum", Address: "0x1234567890123456789012345678901234567890"}
	db.Create(&entry)

	r := addressbookRouter(db, user.ID)

	longMemo := ""
	for i := 0; i < 260; i++ {
		longMemo += "x"
	}
	body := map[string]interface{}{"memo": longMemo}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/addressbook/%d", entry.ID), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for memo too long, got %d: %s", w.Code, w.Body.String())
	}
}

// addressbookRouterAPIKey returns a test router with API key auth mode.
func addressbookRouterAPIKey(db *gorm.DB, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := handler.NewAddressBookHandler(db, nil, "")

	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", "apikey")
		c.Set("apiKeyPrefix", "ocw_test")
		c.Next()
	})
	r.POST("/addressbook", h.AddEntry)
	r.PUT("/addressbook/:id", h.UpdateEntry)
	return r
}

func TestAddEntry_APIKey_Returns202(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := addressbookRouterAPIKey(db, user.ID)

	body := map[string]interface{}{
		"nickname": "alice",
		"chain":    "ethereum",
		"address":  "0x1234567890123456789012345678901234567890",
	}
	req := httptest.NewRequest(http.MethodPost, "/addressbook", jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for API key path, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected status=pending_approval, got %v", resp["status"])
	}
	if resp["approval_id"] == nil {
		t.Error("expected approval_id in response")
	}

	// Entry should NOT exist in DB yet (pending approval).
	var count int64
	db.Model(&model.AddressBookEntry{}).Where("user_id = ? AND nickname = ?", user.ID, "alice").Count(&count)
	if count != 0 {
		t.Errorf("expected 0 entries before approval, got %d", count)
	}
}

func TestUpdateEntry_APIKey_Returns202(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)

	entry := model.AddressBookEntry{UserID: user.ID, Nickname: "alice", Chain: "ethereum", Address: "0x1234567890123456789012345678901234567890"}
	db.Create(&entry)

	r := addressbookRouterAPIKey(db, user.ID)

	body := map[string]interface{}{"memo": "updated"}
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/addressbook/%d", entry.ID), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for API key update, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "pending_approval" {
		t.Errorf("expected status=pending_approval, got %v", resp["status"])
	}

	// Entry should NOT be updated yet.
	var existing model.AddressBookEntry
	db.First(&existing, entry.ID)
	if existing.Memo != "" {
		t.Errorf("expected memo unchanged before approval, got %q", existing.Memo)
	}
}

func TestLooksLikeAddress_Solana(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"11111111111111111111111111111111", true},            // 32 chars
		{"So11111111111111111111111111111111111111112", true},  // 43 chars
		{"short", false},                                       // too short
		{"alice_wallet", false},                                // contains _
		{"bob-wallet-aaaaaaaaaaaaaaaaaaaaaaaaa", false},        // contains -
	}
	for _, tt := range tests {
		got := handler.LooksLikeAddress(tt.input, "solana")
		if got != tt.want {
			t.Errorf("LooksLikeAddress(%q, solana) = %v, want %v", tt.input, got, tt.want)
		}
	}
}
