// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestJsonErrorDetails_SkipsReservedKeys(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	jsonErrorDetails(c, http.StatusBadRequest, "original error", gin.H{
		"success": true,
		"error":   "attacker override",
		"detail":  "extra info",
	})

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["success"] != false {
		t.Errorf("success = %v, want false", resp["success"])
	}
	if resp["error"] != "original error" {
		t.Errorf("error = %v, want 'original error'", resp["error"])
	}
	if resp["detail"] != "extra info" {
		t.Errorf("detail = %v, want 'extra info'", resp["detail"])
	}
}

func TestJsonError_BasicShape(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	jsonError(c, http.StatusNotFound, "not found")

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["success"] != false {
		t.Errorf("success = %v, want false", resp["success"])
	}
	if resp["error"] != "not found" {
		t.Errorf("error = %v, want 'not found'", resp["error"])
	}
}

func TestMustUserID_WrongType(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Set("userID", int(42)) // wrong type: int instead of uint

	id := mustUserID(c)
	if id != 0 {
		t.Errorf("expected 0 for wrong type, got %d", id)
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestMustUserID_Missing(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	// don't set userID

	id := mustUserID(c)
	if id != 0 {
		t.Errorf("expected 0 for missing userID, got %d", id)
	}
	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestMustUserID_Valid(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
	c.Set("userID", uint(42))

	id := mustUserID(c)
	if id != 42 {
		t.Errorf("expected 42, got %d", id)
	}
}

func TestGetCSRF_ExpiredSession(t *testing.T) {
	store := NewSessionStore()
	defer store.Stop()

	token := "ps_" + randomHex(24)
	store.Set(token, 1, -1*time.Second)

	csrf := store.GetCSRF(token)
	if csrf != "" {
		t.Errorf("expected empty CSRF for expired session, got %q", csrf)
	}
}
