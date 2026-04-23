// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
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

// TestSanitizeErrString_RedactsProviderToken guards the main concern that
// motivated this sanitizer: Go's net/http errors embed the full RPC URL —
// including any QuickNode token in the path — and that must never reach
// a client response.
func TestSanitizeErrString_RedactsProviderToken(t *testing.T) {
	raw := fmt.Errorf(`Post "https://wispy-wiser-road.base-sepolia.quiknode.pro/abc123secret/": dial tcp: lookup host: no such host`)
	got := sanitizeErrString(raw)
	if strings.Contains(got, "abc123secret") {
		t.Fatalf("sanitizer leaked token: %q", got)
	}
	if strings.Contains(got, "quiknode.pro") {
		t.Fatalf("sanitizer left hostname in place: %q", got)
	}
	if !strings.Contains(got, "<url>") {
		t.Fatalf("sanitizer did not insert <url> placeholder: %q", got)
	}
	if !strings.Contains(got, "no such host") {
		t.Fatalf("sanitizer stripped the useful diagnostic tail: %q", got)
	}

	// Nil input is tolerated and yields empty string.
	if s := sanitizeErrString(nil); s != "" {
		t.Fatalf("nil err should yield empty string, got %q", s)
	}
}

// TestRespondInternalError_OmitsRawError verifies the 500 path never leaks
// DB/internal error text and always returns a correlation ID instead.
func TestRespondInternalError_OmitsRawError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

	dbErr := errors.New("UNIQUE constraint failed: api_keys.prefix")
	respondInternalError(c, "db error", dbErr, gin.H{"stage": "apikey_save"})

	body := w.Body.String()
	if strings.Contains(body, "UNIQUE constraint failed") {
		t.Fatalf("500 response leaked raw DB error: %s", body)
	}
	if strings.Contains(body, "api_keys.prefix") {
		t.Fatalf("500 response leaked schema details: %s", body)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["error"] != "db error" {
		t.Errorf("error field = %v, want 'db error'", resp["error"])
	}
	if _, ok := resp["request_id"].(string); !ok || resp["request_id"] == "" {
		t.Errorf("request_id missing or not a string: %v", resp["request_id"])
	}
	if resp["stage"] != "apikey_save" {
		t.Errorf("stage lost: %v", resp["stage"])
	}
	if _, leaked := resp["reason"]; leaked {
		t.Errorf("reason field must not appear in 500 responses, got: %v", resp["reason"])
	}
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

// TestCategorizeSigningError maps common TEE/SDK error strings to safe
// category labels; the raw error text must not be exposed elsewhere.
func TestCategorizeSigningError(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("context deadline exceeded"), "timeout"},
		{errors.New("rpc error: connection refused"), "tee_unavailable"},
		{errors.New("threshold not reached: got 2, need 3"), "threshold_not_reached"},
		{errors.New("operation was cancelled"), "cancelled"},
		{errors.New("unknown internal failure"), "sdk_error"},
		{nil, ""},
	}
	for _, tc := range cases {
		if got := categorizeSigningError(tc.err); got != tc.want {
			t.Errorf("categorizeSigningError(%v) = %q, want %q", tc.err, got, tc.want)
		}
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
