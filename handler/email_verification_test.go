// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/handler"
	"github.com/TEENet-io/teenet-wallet/model"
)

func TestGenerateVerificationCode_Format(t *testing.T) {
	for i := 0; i < 100; i++ {
		code := handler.GenerateVerificationCode()
		if len(code) != 6 {
			t.Fatalf("code length = %d, want 6 (got %q)", len(code), code)
		}
		for _, r := range code {
			if r < '0' || r > '9' {
				t.Fatalf("code contains non-digit: %q", code)
			}
		}
	}
}

func TestHashCode_Deterministic(t *testing.T) {
	a := handler.HashCode("123456")
	b := handler.HashCode("123456")
	if a != b {
		t.Fatalf("hashes differ: %s vs %s", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("sha256 hex length = %d, want 64", len(a))
	}
}

func TestHashCode_DifferentInputs(t *testing.T) {
	if handler.HashCode("123456") == handler.HashCode("123457") {
		t.Fatal("different inputs produced same hash")
	}
}

func TestNormalizeEmail(t *testing.T) {
	cases := map[string]string{
		"Alice@Foo.COM":    "alice@foo.com",
		"  bob@bar.io  ":   "bob@bar.io",
		"already@lower.io": "already@lower.io",
	}
	for in, want := range cases {
		if got := handler.NormalizeEmail(in); got != want {
			t.Errorf("NormalizeEmail(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateEmailFormat(t *testing.T) {
	good := []string{"a@b.co", "alice@example.com", "x.y+z@sub.example.io"}
	bad := []string{"", "noatsign", "@nodomain.io", "no@dot", "a@b", strings.Repeat("a", 250) + "@b.co"}
	for _, e := range good {
		if !handler.ValidateEmailFormat(e) {
			t.Errorf("expected %q to be valid", e)
		}
	}
	for _, e := range bad {
		if handler.ValidateEmailFormat(e) {
			t.Errorf("expected %q to be invalid", e)
		}
	}
}

func TestGenerateVerificationID_HexLength(t *testing.T) {
	id := handler.GenerateVerificationID()
	if len(id) != 64 {
		t.Fatalf("verification id length = %d, want 64 (got %q)", len(id), id)
	}
	for _, r := range id {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			t.Fatalf("verification id contains non-hex: %q", id)
		}
	}
}

// fakeSender records the last code without sending email.
type fakeSender struct {
	lastTo   string
	lastCode string
	failNext bool
}

func (f *fakeSender) Mode() string { return "fake" }
func (f *fakeSender) Send(to, code string) error {
	if f.failNext {
		f.failNext = false
		return fmt.Errorf("fake send failure")
	}
	f.lastTo = to
	f.lastCode = code
	return nil
}

func newServiceForTest(t *testing.T) (*handler.EmailVerificationService, *gorm.DB, *fakeSender) {
	t.Helper()
	db := testDB(t)
	fs := &fakeSender{}
	svc := handler.NewEmailVerificationService(db, fs, handler.EmailVerificationConfig{
		CodeTTL:        10 * time.Minute,
		ResendCooldown: 60 * time.Second,
		MaxAttempts:    5,
	})
	return svc, db, fs
}

func TestSendCode_HappyPath(t *testing.T) {
	svc, db, fs := newServiceForTest(t)
	if err := svc.SendCode("Alice@Foo.com"); err != nil {
		t.Fatalf("SendCode: %v", err)
	}
	if fs.lastTo != "alice@foo.com" {
		t.Errorf("recipient = %q, want lowercased", fs.lastTo)
	}
	if len(fs.lastCode) != 6 {
		t.Errorf("code length = %d", len(fs.lastCode))
	}
	var count int64
	db.Model(&model.EmailVerification{}).Where("email = ?", "alice@foo.com").Count(&count)
	if count != 1 {
		t.Errorf("expected 1 row in email_verifications, got %d", count)
	}
}

func TestSendCode_RejectsInvalidEmail(t *testing.T) {
	svc, _, _ := newServiceForTest(t)
	if err := svc.SendCode("not-an-email"); !errors.Is(err, handler.ErrEmailInvalid) {
		t.Errorf("err = %v, want ErrEmailInvalid", err)
	}
}

func TestSendCode_RejectsAlreadyRegistered(t *testing.T) {
	svc, db, _ := newServiceForTest(t)
	existing := "taken@foo.com"
	db.Create(&model.User{Username: "u1", PasskeyUserID: 999, Email: &existing})

	if err := svc.SendCode("Taken@foo.com"); !errors.Is(err, handler.ErrEmailAlreadyTaken) {
		t.Errorf("err = %v, want ErrEmailAlreadyTaken", err)
	}
}

func TestSendCode_DifferentEmailsCoexist(t *testing.T) {
	// Regression: two unverified rows previously collided on the unique
	// verification_id index because both stored "" instead of NULL.
	svc, db, _ := newServiceForTest(t)
	if err := svc.SendCode("alice@foo.com"); err != nil {
		t.Fatalf("alice: %v", err)
	}
	if err := svc.SendCode("bob@foo.com"); err != nil {
		t.Fatalf("bob: %v", err)
	}
	var count int64
	db.Model(&model.EmailVerification{}).Count(&count)
	if count != 2 {
		t.Errorf("expected 2 rows, got %d", count)
	}
}

func TestSendCode_CooldownBlocksRapidResend(t *testing.T) {
	svc, _, _ := newServiceForTest(t)
	if err := svc.SendCode("alice@foo.com"); err != nil {
		t.Fatalf("first send: %v", err)
	}
	if err := svc.SendCode("alice@foo.com"); !errors.Is(err, handler.ErrCooldown) {
		t.Errorf("second send err = %v, want ErrCooldown", err)
	}
}

func TestSendCode_RollsBackOnSMTPFailure(t *testing.T) {
	svc, db, fs := newServiceForTest(t)
	fs.failNext = true
	if err := svc.SendCode("alice@foo.com"); err == nil {
		t.Fatal("expected SMTP failure, got nil")
	}
	var count int64
	db.Model(&model.EmailVerification{}).Where("email = ?", "alice@foo.com").Count(&count)
	if count != 0 {
		t.Errorf("expected 0 rows after rollback, got %d", count)
	}
}

func TestVerifyCode_HappyPath(t *testing.T) {
	svc, _, fs := newServiceForTest(t)
	if err := svc.SendCode("alice@foo.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	vid, err := svc.VerifyCode("alice@foo.com", fs.lastCode)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(vid) != 64 {
		t.Errorf("verification_id length = %d", len(vid))
	}
}

func TestVerifyCode_NoPending(t *testing.T) {
	svc, _, _ := newServiceForTest(t)
	_, err := svc.VerifyCode("alice@foo.com", "123456")
	if !errors.Is(err, handler.ErrNoPendingCode) {
		t.Errorf("err = %v, want ErrNoPendingCode", err)
	}
}

func TestVerifyCode_WrongCodeIncrementsAttempts(t *testing.T) {
	svc, db, _ := newServiceForTest(t)
	if err := svc.SendCode("alice@foo.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	for i := 0; i < 4; i++ {
		_, err := svc.VerifyCode("alice@foo.com", "000000")
		if !errors.Is(err, handler.ErrCodeInvalid) {
			t.Fatalf("attempt %d: err = %v, want ErrCodeInvalid", i, err)
		}
	}
	var row model.EmailVerification
	db.Where("email = ?", "alice@foo.com").First(&row)
	if row.Attempts != 4 {
		t.Errorf("attempts = %d, want 4", row.Attempts)
	}
}

func TestVerifyCode_AttemptsExhausted(t *testing.T) {
	svc, _, _ := newServiceForTest(t)
	if err := svc.SendCode("alice@foo.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	for i := 0; i < 5; i++ {
		_, _ = svc.VerifyCode("alice@foo.com", "000000")
	}
	_, err := svc.VerifyCode("alice@foo.com", "000000")
	if !errors.Is(err, handler.ErrAttemptsExhausted) && !errors.Is(err, handler.ErrNoPendingCode) {
		t.Errorf("err = %v, want ErrAttemptsExhausted or ErrNoPendingCode", err)
	}
}

func TestVerifyCode_Expired(t *testing.T) {
	db := testDB(t)
	fs := &fakeSender{}
	svc := handler.NewEmailVerificationService(db, fs, handler.EmailVerificationConfig{
		CodeTTL:        1 * time.Millisecond,
		ResendCooldown: 0,
		MaxAttempts:    5,
	})
	if err := svc.SendCode("alice@foo.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	_, err := svc.VerifyCode("alice@foo.com", fs.lastCode)
	if !errors.Is(err, handler.ErrCodeExpired) && !errors.Is(err, handler.ErrNoPendingCode) {
		t.Errorf("err = %v, want ErrCodeExpired or ErrNoPendingCode", err)
	}
}

func TestVerifyCode_NormalizesEmail(t *testing.T) {
	svc, _, fs := newServiceForTest(t)
	if err := svc.SendCode("alice@foo.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	vid, err := svc.VerifyCode("ALICE@foo.COM", fs.lastCode)
	if err != nil {
		t.Fatalf("verify normalized: %v", err)
	}
	if vid == "" {
		t.Fatal("expected non-empty vid")
	}
}

func TestLookupVerification_Valid(t *testing.T) {
	svc, _, fs := newServiceForTest(t)
	if err := svc.SendCode("alice@foo.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	vid, err := svc.VerifyCode("alice@foo.com", fs.lastCode)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	email, err := svc.LookupVerification(vid)
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if email != "alice@foo.com" {
		t.Errorf("email = %q, want alice@foo.com", email)
	}
}

func TestLookupVerification_BadID(t *testing.T) {
	svc, _, _ := newServiceForTest(t)
	_, err := svc.LookupVerification("deadbeef")
	if !errors.Is(err, handler.ErrVerificationIDBad) {
		t.Errorf("err = %v, want ErrVerificationIDBad", err)
	}
}

func TestConsumeVerification_DeletesRow(t *testing.T) {
	svc, db, fs := newServiceForTest(t)
	if err := svc.SendCode("alice@foo.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	vid, err := svc.VerifyCode("alice@foo.com", fs.lastCode)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	err = db.Transaction(func(tx *gorm.DB) error {
		email, err := svc.ConsumeVerification(tx, vid)
		if err != nil {
			return err
		}
		if email != "alice@foo.com" {
			t.Errorf("consumed email = %q", email)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("consume tx: %v", err)
	}
	var count int64
	db.Model(&model.EmailVerification{}).Where("verification_id = ?", vid).Count(&count)
	if count != 0 {
		t.Errorf("expected row deleted, got %d remaining", count)
	}
}

func TestConsumeVerification_DoubleConsumeFails(t *testing.T) {
	svc, db, fs := newServiceForTest(t)
	if err := svc.SendCode("alice@foo.com"); err != nil {
		t.Fatalf("send: %v", err)
	}
	vid, err := svc.VerifyCode("alice@foo.com", fs.lastCode)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	_ = db.Transaction(func(tx *gorm.DB) error {
		_, err := svc.ConsumeVerification(tx, vid)
		return err
	})
	err = db.Transaction(func(tx *gorm.DB) error {
		_, err := svc.ConsumeVerification(tx, vid)
		return err
	})
	if !errors.Is(err, handler.ErrVerificationIDBad) {
		t.Errorf("second consume err = %v, want ErrVerificationIDBad", err)
	}
}

func newEmailRouterForTest(t *testing.T, db *gorm.DB, sender handler.EmailSender) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := handler.NewEmailVerificationService(db, sender, handler.EmailVerificationConfig{
		CodeTTL:        10 * time.Minute,
		ResendCooldown: 60 * time.Second,
		MaxAttempts:    5,
	})
	h := handler.NewEmailVerificationHandler(svc)
	r.POST("/api/auth/email/send-code", h.SendCode)
	r.POST("/api/auth/email/verify-code", h.VerifyCode)
	return r
}

func TestSendCodeHandler_Returns200(t *testing.T) {
	db := testDB(t)
	fs := &fakeSender{}
	r := newEmailRouterForTest(t, db, fs)

	body, _ := json.Marshal(map[string]string{"email": "alice@foo.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/send-code", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", w.Code, w.Body.String())
	}
	if fs.lastCode == "" {
		t.Error("expected fakeSender to have received a code")
	}
}

func TestSendCodeHandler_Rejects409WhenEmailTaken(t *testing.T) {
	db := testDB(t)
	taken := "taken@foo.com"
	db.Create(&model.User{Username: "u1", PasskeyUserID: 1, Email: &taken})
	r := newEmailRouterForTest(t, db, &fakeSender{})

	body, _ := json.Marshal(map[string]string{"email": "Taken@foo.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/send-code", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409, body = %s", w.Code, w.Body.String())
	}
}

func TestVerifyCodeHandler_HappyPath(t *testing.T) {
	db := testDB(t)
	fs := &fakeSender{}
	r := newEmailRouterForTest(t, db, fs)

	// First send a code to populate fakeSender.
	body, _ := json.Marshal(map[string]string{"email": "alice@foo.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/send-code", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("send-code failed: %d %s", w.Code, w.Body.String())
	}

	// Now verify it.
	body2, _ := json.Marshal(map[string]string{"email": "alice@foo.com", "code": fs.lastCode})
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/email/verify-code", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("verify-code failed: %d %s", w2.Code, w2.Body.String())
	}
	var resp map[string]any
	_ = json.Unmarshal(w2.Body.Bytes(), &resp)
	if vid, _ := resp["verification_id"].(string); len(vid) != 64 {
		t.Errorf("verification_id = %v", resp["verification_id"])
	}
}

func TestVerifyCodeHandler_WrongCodeReturns401(t *testing.T) {
	db := testDB(t)
	fs := &fakeSender{}
	r := newEmailRouterForTest(t, db, fs)

	body, _ := json.Marshal(map[string]string{"email": "alice@foo.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/email/send-code", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	body2, _ := json.Marshal(map[string]string{"email": "alice@foo.com", "code": "000000"})
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/email/verify-code", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	if w2.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401, body = %s", w2.Code, w2.Body.String())
	}
}
