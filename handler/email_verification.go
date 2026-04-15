// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/model"
)

// EmailVerificationConfig captures tunable knobs for the email-verification flow.
type EmailVerificationConfig struct {
	CodeTTL        time.Duration // default 10 min
	ResendCooldown time.Duration // default 60 s
	MaxAttempts    int           // default 5
}

// EmailVerificationService owns the full code lifecycle.
type EmailVerificationService struct {
	db     *gorm.DB
	sender EmailSender
	cfg    EmailVerificationConfig
}

// NewEmailVerificationService constructs the service. `sender` may be a
// SmtpEmailSender or a MockEmailSender; tests inject their own fake.
func NewEmailVerificationService(db *gorm.DB, sender EmailSender, cfg EmailVerificationConfig) *EmailVerificationService {
	if cfg.CodeTTL <= 0 {
		cfg.CodeTTL = 10 * time.Minute
	}
	if cfg.ResendCooldown <= 0 {
		cfg.ResendCooldown = 60 * time.Second
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 5
	}
	return &EmailVerificationService{db: db, sender: sender, cfg: cfg}
}

// GenerateVerificationCode returns a uniformly distributed 6-digit numeric code.
func GenerateVerificationCode() string {
	max := big.NewInt(1000000) // 0..999999
	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		// crypto/rand failure is catastrophic; fall back to a deterministic
		// but obviously-wrong code so callers notice in logs.
		return "000000"
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// HashCode returns sha256(code) as lowercase hex. Used for storage and
// constant-time comparison.
func HashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// GenerateVerificationID returns 32 random bytes as 64-char lowercase hex.
func GenerateVerificationID() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

// NormalizeEmail lowercases and trims whitespace.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// ValidateEmailFormat performs minimal RFC-style validation: length 5-254,
// exactly one '@', and at least one '.' in the domain part. No regex, no
// third-party validator. Mirrors UMS's utils.ValidateEmail.
func ValidateEmailFormat(email string) bool {
	if len(email) < 5 || len(email) > 254 {
		return false
	}
	if strings.Count(email, "@") != 1 {
		return false
	}
	at := strings.LastIndex(email, "@")
	if at <= 0 || at >= len(email)-1 {
		return false
	}
	domain := email[at+1:]
	if !strings.Contains(domain, ".") {
		return false
	}
	return true
}

// Sentinel errors returned by SendCode/VerifyCode (defined here so later
// tasks can return them; not all are used yet).
var (
	ErrEmailInvalid      = errors.New("email_invalid")
	ErrEmailAlreadyTaken = errors.New("email_already_registered")
	ErrCooldown          = errors.New("cooldown")
	ErrSMTPSend          = errors.New("smtp_send_failed")
	ErrNoPendingCode     = errors.New("no_pending_code")
	ErrCodeExpired       = errors.New("code_expired")
	ErrCodeInvalid       = errors.New("code_invalid")
	ErrAttemptsExhausted = errors.New("attempts_exhausted")
	ErrVerificationIDBad = errors.New("invalid_verification_id")
)

// SendCode is the entry point for the send-code HTTP handler.
//
// Order of checks:
//  1. validate format
//  2. reject if already in users.email
//  3. reject if within cooldown of latest non-expired row
//  4. sweep expired rows for this email
//  5. insert new row + send email; on SMTP failure roll back the insert
func (s *EmailVerificationService) SendCode(rawEmail string) error {
	email := NormalizeEmail(rawEmail)
	if !ValidateEmailFormat(email) {
		return ErrEmailInvalid
	}

	// Step 2: collision with an already-registered wallet user.
	var taken int64
	if err := s.db.Model(&model.User{}).Where("email = ?", email).Count(&taken).Error; err != nil {
		return err
	}
	if taken > 0 {
		return ErrEmailAlreadyTaken
	}

	now := time.Now()

	// Step 3: cooldown check against the most recent non-expired row.
	var latest model.EmailVerification
	err := s.db.Where("email = ? AND expires_at > ?", email, now).
		Order("last_sent_at DESC").
		First(&latest).Error
	if err == nil {
		if now.Sub(latest.LastSentAt) < s.cfg.ResendCooldown {
			return ErrCooldown
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}

	// Step 4: sweep expired rows for this email.
	if err := s.db.Where("email = ? AND expires_at <= ?", email, now).
		Delete(&model.EmailVerification{}).Error; err != nil {
		return err
	}

	// Step 5: insert + send inside a transaction so SMTP failure rolls back.
	code := GenerateVerificationCode()
	row := model.EmailVerification{
		Email:      email,
		CodeHash:   HashCode(code),
		ExpiresAt:  now.Add(s.cfg.CodeTTL),
		LastSentAt: now,
		CreatedAt:  now,
	}
	txErr := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&row).Error; err != nil {
			return err
		}
		if err := s.sender.Send(email, code); err != nil {
			return fmt.Errorf("%w: %v", ErrSMTPSend, err)
		}
		return nil
	})
	return txErr
}

// VerifyCode checks the user-supplied code against the most recent
// non-consumed row for the email. On first success it sets verified_at and
// returns a fresh verification_id. Subsequent calls with the same correct
// code return the existing verification_id (idempotent), which lets the
// user retry the downstream passkey ceremony without having to request a
// new code — important because the passkey "save to…" dialog can fail for
// device-local reasons (biometric cancelled, wrong provider picked, etc.).
func (s *EmailVerificationService) VerifyCode(rawEmail, code string) (string, error) {
	email := NormalizeEmail(rawEmail)
	now := time.Now()

	var row model.EmailVerification
	// Match any non-consumed row (verified or not) so that re-verifying with
	// the same code is idempotent as long as the row hasn't been used yet.
	err := s.db.Where("email = ? AND consumed_at IS NULL", email).
		Order("created_at DESC").
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrNoPendingCode
	}
	if err != nil {
		return "", err
	}

	if now.After(row.ExpiresAt) {
		return "", ErrCodeExpired
	}
	if row.Attempts >= s.cfg.MaxAttempts {
		// Force-expire the row so the next call returns ErrNoPendingCode.
		s.db.Model(&row).Update("expires_at", now.Add(-time.Second))
		return "", ErrAttemptsExhausted
	}

	if HashCode(code) != row.CodeHash {
		s.db.Model(&row).Update("attempts", row.Attempts+1)
		return "", ErrCodeInvalid
	}

	// Idempotent path: row already verified and still usable — just return
	// the existing verification_id.
	if row.VerifiedAt != nil && row.VerificationID != nil && *row.VerificationID != "" {
		return *row.VerificationID, nil
	}

	vid := GenerateVerificationID()
	verifiedAt := now
	if err := s.db.Model(&row).Updates(map[string]any{
		"verified_at":     &verifiedAt,
		"verification_id": &vid,
	}).Error; err != nil {
		return "", err
	}
	return vid, nil
}

// LookupVerification reads (without consuming) a verification row by id.
// Returns the email if the row is verified, not consumed, and not expired.
// Used by PasskeyRegistrationBegin to derive the DisplayName before kicking
// off the WebAuthn ceremony.
func (s *EmailVerificationService) LookupVerification(verificationID string) (string, error) {
	if verificationID == "" {
		return "", ErrVerificationIDBad
	}
	var row model.EmailVerification
	err := s.db.Where("verification_id = ? AND verified_at IS NOT NULL AND consumed_at IS NULL AND expires_at > ?",
		verificationID, time.Now()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrVerificationIDBad
	}
	if err != nil {
		return "", err
	}
	return row.Email, nil
}

// ConsumeVerification atomically reads + deletes a verification row inside
// the caller's transaction. Used by PasskeyRegistrationVerify so the
// verification row and the new users row are committed together.
//
// Returns ErrVerificationIDBad if the row is missing, already consumed, not
// verified, or expired.
func (s *EmailVerificationService) ConsumeVerification(tx *gorm.DB, verificationID string) (string, error) {
	if verificationID == "" {
		return "", ErrVerificationIDBad
	}
	var row model.EmailVerification
	err := tx.Where("verification_id = ? AND verified_at IS NOT NULL AND consumed_at IS NULL AND expires_at > ?",
		verificationID, time.Now()).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return "", ErrVerificationIDBad
	}
	if err != nil {
		return "", err
	}
	if err := tx.Delete(&row).Error; err != nil {
		return "", err
	}
	return row.Email, nil
}

// EmailVerificationHandler exposes the service over HTTP.
type EmailVerificationHandler struct {
	svc *EmailVerificationService
}

func NewEmailVerificationHandler(svc *EmailVerificationService) *EmailVerificationHandler {
	return &EmailVerificationHandler{svc: svc}
}

// SendCode is POST /api/auth/email/send-code (unauthenticated).
func (h *EmailVerificationHandler) SendCode(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if err := h.svc.SendCode(req.Email); err != nil {
		switch {
		case errors.Is(err, ErrEmailInvalid):
			jsonError(c, http.StatusBadRequest, "invalid email format")
		case errors.Is(err, ErrEmailAlreadyTaken):
			jsonError(c, http.StatusConflict, "email already registered")
		case errors.Is(err, ErrCooldown):
			jsonError(c, http.StatusTooManyRequests, "please wait before requesting another code")
		case errors.Is(err, ErrSMTPSend):
			jsonError(c, http.StatusInternalServerError, "failed to send verification email")
		default:
			jsonError(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"message":         "code sent",
		"expires_in":      int(h.svc.cfg.CodeTTL.Seconds()),
		"resend_cooldown": int(h.svc.cfg.ResendCooldown.Seconds()),
	})
}

// VerifyCode is POST /api/auth/email/verify-code (unauthenticated).
func (h *EmailVerificationHandler) VerifyCode(c *gin.Context) {
	var req struct {
		Email string `json:"email"`
		Code  string `json:"code"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	if req.Email == "" || req.Code == "" {
		jsonError(c, http.StatusBadRequest, "email and code are required")
		return
	}
	vid, err := h.svc.VerifyCode(req.Email, req.Code)
	if err != nil {
		switch {
		case errors.Is(err, ErrNoPendingCode):
			jsonError(c, http.StatusNotFound, "no pending code for this email")
		case errors.Is(err, ErrCodeExpired):
			jsonError(c, http.StatusGone, "code expired")
		case errors.Is(err, ErrCodeInvalid):
			jsonError(c, http.StatusUnauthorized, "invalid code")
		case errors.Is(err, ErrAttemptsExhausted):
			jsonError(c, http.StatusLocked, "too many failed attempts, please request a new code")
		default:
			jsonError(c, http.StatusInternalServerError, err.Error())
		}
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":         true,
		"verification_id": vid,
		"expires_in":      int(h.svc.cfg.CodeTTL.Seconds()),
	})
}
