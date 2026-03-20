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

// ─── router ───────────────────────────────────────────────────────────────────

func policyRouter(db *gorm.DB, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, nil, "http://localhost")
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", "passkey")
		c.Next()
	})
	r.PUT("/wallets/:id/policy", wh.SetPolicy)
	r.GET("/wallets/:id/policy", wh.GetPolicy)
	return r
}

// ─── SetPolicy ────────────────────────────────────────────────────────────────

func TestSetPolicy_Success(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := policyRouter(db, user.ID)

	body := jsonBody(map[string]interface{}{
		"threshold_amount": "0.5",
		"currency":         "eth",
		"enabled":          true,
	})
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/wallets/%s/policy", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var stored model.ApprovalPolicy
	if err := db.Where("wallet_id = ?", wallet.ID).First(&stored).Error; err != nil {
		t.Fatal("policy not persisted to DB")
	}
	if stored.ThresholdAmount != "0.5" {
		t.Errorf("threshold: got %s, want 0.5", stored.ThresholdAmount)
	}
	if stored.Currency != "ETH" { // SetPolicy normalises to uppercase
		t.Errorf("currency: got %s, want ETH", stored.Currency)
	}
}

func TestSetPolicy_UpdateExisting(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)

	// Seed an initial policy.
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdAmount: "1.0", Currency: "ETH", Enabled: true,
	})

	r := policyRouter(db, user.ID)
	body := jsonBody(map[string]interface{}{
		"threshold_amount": "2.0",
		"currency":         "ETH",
	})
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/wallets/%s/policy", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var policies []model.ApprovalPolicy
	db.Where("wallet_id = ?", wallet.ID).Find(&policies)
	if len(policies) != 1 {
		t.Errorf("expected 1 policy (upsert), got %d", len(policies))
	}
	if policies[0].ThresholdAmount != "2.0" {
		t.Errorf("expected threshold 2.0, got %s", policies[0].ThresholdAmount)
	}
}

func TestSetPolicy_MissingFields_Returns400(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := policyRouter(db, user.ID)

	// No threshold_amount field.
	body := jsonBody(map[string]interface{}{"currency": "ETH"})
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/wallets/%s/policy", wallet.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing fields, got %d: %s", w.Code, w.Body.String())
	}
}

func TestSetPolicy_WrongWallet_Returns403(t *testing.T) {
	db := testDB(t)
	user1, _ := seedWallet(t, db)
	_, wallet2 := seedWallet(t, db)

	// Authenticate as user1 but target wallet2 (owned by another user).
	r := policyRouter(db, user1.ID)
	body := jsonBody(map[string]interface{}{
		"threshold_amount": "0.1",
		"currency":         "ETH",
	})
	req := httptest.NewRequest(http.MethodPut, fmt.Sprintf("/wallets/%s/policy", wallet2.ID), body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// ─── GetPolicy ────────────────────────────────────────────────────────────────

func TestGetPolicy_NoPolicy_ReturnsNull(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	r := policyRouter(db, user.ID)

	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/policy", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["policy"] != nil {
		t.Errorf("expected policy=null when none exists, got %v", resp["policy"])
	}
}

func TestGetPolicy_WithPolicy(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)
	db.Create(&model.ApprovalPolicy{
		WalletID: wallet.ID, ThresholdAmount: "1.5", Currency: "SOL", Enabled: true,
	})

	r := policyRouter(db, user.ID)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/policy", wallet.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)
	policies, ok := resp["policies"].([]interface{})
	if !ok || len(policies) == 0 {
		t.Fatalf("expected non-empty policies array, got %v", resp["policies"])
	}
	policy, ok := policies[0].(map[string]interface{})
	if !ok || policy == nil {
		t.Fatal("expected non-null policy object")
	}
	if policy["threshold_amount"] != "1.5" {
		t.Errorf("expected threshold_amount=1.5, got %v", policy["threshold_amount"])
	}
	if policy["currency"] != "SOL" {
		t.Errorf("expected currency=SOL, got %v", policy["currency"])
	}
}

func TestGetPolicy_WrongWallet_Returns403(t *testing.T) {
	db := testDB(t)
	user1, _ := seedWallet(t, db)
	_, wallet2 := seedWallet(t, db)

	r := policyRouter(db, user1.ID)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/wallets/%s/policy", wallet2.ID), nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for another user's wallet, got %d", w.Code)
	}
}
