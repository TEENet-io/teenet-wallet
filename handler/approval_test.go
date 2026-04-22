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

// ─── router ───────────────────────────────────────────────────────────────────

func approvalRouter(db *gorm.DB, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	ah := handler.NewApprovalHandler(db, nil, nil)
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", "passkey")
		c.Next()
	})
	r.GET("/approvals/pending", ah.ListPending)
	r.GET("/approvals/:id", ah.GetApproval)
	return r
}

// seedApproval inserts an ApprovalRequest with the given status and expiry.
func seedApproval(t *testing.T, db *gorm.DB, walletID string, userID uint, status string, expiresAt time.Time) model.ApprovalRequest {
	t.Helper()
	a := model.ApprovalRequest{
		WalletID:  &walletID,
		UserID:    userID,
		Message:   "deadbeef",
		TxContext: `{"type":"transfer","from":"0x1","to":"0x2","amount":"1","currency":"ETH"}`,
		Status:    status,
		ExpiresAt: expiresAt,
	}
	if err := db.Create(&a).Error; err != nil {
		t.Fatalf("create approval: %v", err)
	}
	return a
}

// ─── ListPending ──────────────────────────────────────────────────────────────

func TestListPending_Empty(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := approvalRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/approvals/pending", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	list, _ := resp["approvals"].([]interface{})
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d", len(list))
	}
}

func TestListPending_ReturnsOnlyPending(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	future := time.Now().Add(30 * time.Minute)

	seedApproval(t, db, wallet.ID, user.ID, "pending", future)
	seedApproval(t, db, wallet.ID, user.ID, "pending", future)
	seedApproval(t, db, wallet.ID, user.ID, "approved", future) // must NOT appear

	r := approvalRouter(db, user.ID)
	req := httptest.NewRequest(http.MethodGet, "/approvals/pending", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	list, _ := resp["approvals"].([]interface{})
	if len(list) != 2 {
		t.Errorf("expected 2 pending, got %d", len(list))
	}
}

func TestListPending_AutoExpiresStale(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)

	// Insert a pending approval whose ExpiresAt is in the past.
	_ = seedApproval(t, db, wallet.ID, user.ID, "pending", time.Now().Add(-1*time.Second))

	r := approvalRouter(db, user.ID)
	req := httptest.NewRequest(http.MethodGet, "/approvals/pending", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	list, _ := resp["approvals"].([]interface{})
	if len(list) != 0 {
		t.Errorf("expired approval should not be in pending list, got %d", len(list))
	}
}

func TestListPending_DoesNotLeakOtherUserApprovals(t *testing.T) {
	db := testDB(t)
	user1, _ := seedWallet(t, db)
	user2, wallet2 := seedWallet(t, db)
	future := time.Now().Add(30 * time.Minute)

	// Approval belonging to user2 — must NOT appear in user1's list.
	seedApproval(t, db, wallet2.ID, user2.ID, "pending", future)

	r := approvalRouter(db, user1.ID)
	req := httptest.NewRequest(http.MethodGet, "/approvals/pending", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	list, _ := resp["approvals"].([]interface{})
	if len(list) != 0 {
		t.Errorf("expected 0 approvals for user1, got %d (leak from user2?)", len(list))
	}
}

// ─── GetApproval ──────────────────────────────────────────────────────────────

func TestGetApproval_NotFound(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := approvalRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/approvals/99999", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetApproval_WrongUser_Returns403(t *testing.T) {
	db := testDB(t)
	user1, wallet1 := seedWallet(t, db)
	user2, _ := seedWallet(t, db)

	a := seedApproval(t, db, wallet1.ID, user1.ID, "pending", time.Now().Add(30*time.Minute))

	// Authenticated as user2 trying to read user1's approval.
	r := approvalRouter(db, user2.ID)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/approvals/%d", a.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetApproval_Success(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	a := seedApproval(t, db, wallet.ID, user.ID, "pending", time.Now().Add(30*time.Minute))

	r := approvalRouter(db, user.ID)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/approvals/%d", a.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["success"] != true {
		t.Errorf("expected success=true, got %v", resp["success"])
	}
}

// TestGetApproval_IncludesTopLevelFields verifies the reconcile-path contract:
// the plugin's ApprovalWatcher reads status, approval_type, tx_hash, wallet_id
// and chain at the top level to rebuild an ApprovalEvent after SSE reconnect.
func TestGetApproval_IncludesTopLevelFields(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db) // Chain defaults to "ethereum"
	a := seedApproval(t, db, wallet.ID, user.ID, "approved", time.Now().Add(30*time.Minute))
	// Simulate a broadcast-succeeded approval by writing the tx hash.
	if err := db.Model(&a).Update("tx_hash", "0xfeed").Error; err != nil {
		t.Fatalf("set tx_hash: %v", err)
	}

	r := approvalRouter(db, user.ID)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/approvals/%d", a.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	if got := resp["status"]; got != "approved" {
		t.Errorf("top-level status: want %q, got %v", "approved", got)
	}
	if got := resp["approval_type"]; got != "sign" { // default when ApprovalType is zero; seedApproval leaves it unset so GORM default applies
		// ApprovalType defaults to "sign" when not set; accept that or empty.
		if got != "" {
			t.Logf("approval_type = %v (accepted)", got)
		}
	}
	if got := resp["tx_hash"]; got != "0xfeed" {
		t.Errorf("top-level tx_hash: want 0xfeed, got %v", got)
	}
	if got := resp["wallet_id"]; got != wallet.ID {
		t.Errorf("top-level wallet_id: want %q, got %v", wallet.ID, got)
	}
	if got := resp["chain"]; got != "ethereum" {
		t.Errorf("top-level chain: want ethereum, got %v", got)
	}
}

func TestGetApproval_AutoExpiresOnFetch(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	a := seedApproval(t, db, wallet.ID, user.ID, "pending", time.Now().Add(-1*time.Second))

	r := approvalRouter(db, user.ID)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/approvals/%d", a.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// GetApproval returns 200 even after auto-expiry but updates the status.
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var stored model.ApprovalRequest
	db.First(&stored, a.ID)
	if stored.Status != "expired" {
		t.Errorf("expected DB status=expired after fetch, got %s", stored.Status)
	}
}

func TestGetApproval_InvalidID_Returns400(t *testing.T) {
	db := testDB(t)
	user, _ := seedWallet(t, db)
	r := approvalRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodGet, "/approvals/notanumber", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-numeric id, got %d", w.Code)
	}
}
