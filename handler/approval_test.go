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
	ah := handler.NewApprovalHandler(db, nil)
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
	a := seedApproval(t, db, wallet.ID, user.ID, "pending", time.Now().Add(-1*time.Second))

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

	// Confirm the DB row was updated to "expired".
	var stored model.ApprovalRequest
	db.First(&stored, a.ID)
	if stored.Status != "expired" {
		t.Errorf("expected DB status=expired, got %s", stored.Status)
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
