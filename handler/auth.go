// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sdk "github.com/TEENet-io/teenet-sdk/go"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/model"
)

// AuthHandler handles passkey registration, login, and API key management.
type AuthHandler struct {
	db         *gorm.DB
	sdk        *sdk.Client
	sessions   *SessionStore
	baseURL    string
	maxAPIKeys int
	maxUsers   int
}

func NewAuthHandler(db *gorm.DB, sdkClient *sdk.Client, sessions *SessionStore, baseURL string) *AuthHandler {
	return &AuthHandler{db: db, sdk: sdkClient, sessions: sessions, baseURL: baseURL, maxAPIKeys: 10, maxUsers: 0}
}

func (h *AuthHandler) SetMaxAPIKeys(n int) { h.maxAPIKeys = n }
func (h *AuthHandler) SetMaxUsers(n int)   { h.maxUsers = n }

// InviteUser invites a passkey user via the SDK admin bridge.
// POST /api/auth/invite
func (h *AuthHandler) InviteUser(c *gin.Context) {
	var req struct {
		DisplayName      string `json:"display_name" binding:"required"`
		ExpiresInSeconds int    `json:"expires_in_seconds"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.ExpiresInSeconds <= 0 {
		req.ExpiresInSeconds = 86400
	}
	res, err := h.sdk.InvitePasskeyUser(c.Request.Context(), sdk.PasskeyInviteRequest{
		DisplayName:      req.DisplayName,
		ExpiresInSeconds: req.ExpiresInSeconds,
	})
	if err != nil || !res.Success {
		msg := "invite failed"
		if err != nil {
			msg = err.Error()
		} else if res != nil {
			msg = res.Error
		}
		slog.Error("passkey invite failed", "error", msg)
		jsonErrorDetails(c, http.StatusBadGateway, msg, gin.H{"stage": "passkey_invite"})
		return
	}
	registerURL := h.baseURL + "/#/register?token=" + res.InviteToken
	c.JSON(http.StatusOK, gin.H{
		"success":      true,
		"invite_token": res.InviteToken,
		"register_url": registerURL,
		"expires_at":   res.ExpiresAt,
	})
}

// CheckName checks whether a display name is already taken.
// GET /api/auth/check-name?name=...  (public)
func (h *AuthHandler) CheckName(c *gin.Context) {
	name := strings.TrimSpace(c.Query("name"))
	if name == "" {
		jsonError(c, http.StatusBadRequest, "name is required")
		return
	}
	var count int64
	h.db.Model(&model.User{}).Where("username = ?", name).Count(&count)
	c.JSON(http.StatusOK, gin.H{"available": count == 0})
}

// PasskeyOptions returns a WebAuthn login challenge.
// GET /api/auth/passkey/options
func (h *AuthHandler) PasskeyOptions(c *gin.Context) {
	res, err := h.sdk.PasskeyLoginOptions(c.Request.Context())
	if err != nil {
		slog.Error("passkey login options failed", "error", err)
		jsonErrorDetails(c, http.StatusBadGateway, err.Error(), gin.H{"stage": "passkey_login_options"})
		return
	}
	c.JSON(http.StatusOK, res)
}

// PasskeyRegistrationBegin auto-generates an invite and returns WebAuthn registration options.
// Open registration — no pre-existing invite token required.
// POST /api/auth/passkey/register/begin
func (h *AuthHandler) PasskeyRegistrationBegin(c *gin.Context) {
	var body struct {
		DisplayName string `json:"display_name"`
	}
	_ = c.ShouldBindJSON(&body)
	displayName := strings.TrimSpace(body.DisplayName)
	if displayName == "" {
		displayName = "user_" + randomHex(3)
	}

	// Auto-generate a short-lived invite (5 min — just for the ceremony).
	inviteRes, err := h.sdk.InvitePasskeyUser(c.Request.Context(), sdk.PasskeyInviteRequest{
		DisplayName:      displayName,
		ExpiresInSeconds: 300,
	})
	if err != nil || !inviteRes.Success {
		msg := "invite failed"
		if err != nil {
			msg = err.Error()
		} else if inviteRes != nil {
			msg = inviteRes.Error
		}
		slog.Error("passkey registration invite failed", "display_name", displayName, "error", msg)
		jsonErrorDetails(c, http.StatusBadGateway, msg, gin.H{"stage": "passkey_registration_invite"})
		return
	}

	// Fetch WebAuthn registration options using the auto-generated token.
	// The result already contains invite_token + options fields.
	optRes, err := h.sdk.PasskeyRegistrationOptions(c.Request.Context(), inviteRes.InviteToken)
	if err != nil || !optRes.Success {
		msg := "get options failed"
		if err != nil {
			msg = err.Error()
		} else if optRes != nil {
			msg = optRes.Error
		}
		slog.Error("passkey registration options failed", "error", msg)
		jsonErrorDetails(c, http.StatusBadGateway, msg, gin.H{"stage": "passkey_registration_options"})
		return
	}
	c.JSON(http.StatusOK, optRes)
}

// PasskeyRegistrationOptions returns WebAuthn registration options for an invite token.
// GET /api/auth/passkey/register/options?token=...
func (h *AuthHandler) PasskeyRegistrationOptions(c *gin.Context) {
	token := strings.TrimSpace(c.Query("token"))
	if token == "" {
		jsonError(c, http.StatusBadRequest, "token is required")
		return
	}
	res, err := h.sdk.PasskeyRegistrationOptions(c.Request.Context(), token)
	if err != nil {
		slog.Error("passkey registration options failed", "error", err)
		jsonErrorDetails(c, http.StatusBadGateway, err.Error(), gin.H{"stage": "passkey_registration_options"})
		return
	}
	c.JSON(http.StatusOK, res)
}

// PasskeyRegistrationVerify verifies passkey registration and creates the local user.
// POST /api/auth/passkey/register/verify
func (h *AuthHandler) PasskeyRegistrationVerify(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	inviteToken, _ := body["invite_token"].(string)
	if inviteToken == "" {
		jsonError(c, http.StatusBadRequest, "invite_token is required")
		return
	}
	res, err := h.sdk.PasskeyRegistrationVerify(c.Request.Context(), inviteToken, body["credential"])
	if err != nil || !res.Success {
		msg := "registration failed"
		if err != nil {
			msg = err.Error()
		} else if res != nil {
			msg = res.Error
		}
		slog.Error("passkey registration verify failed", "error", msg)
		jsonErrorDetails(c, http.StatusBadGateway, msg, gin.H{"stage": "passkey_registration_verify"})
		return
	}
	// Enforce user limit.
	if h.maxUsers > 0 {
		var userCount int64
		h.db.Model(&model.User{}).Count(&userCount)
		if userCount >= int64(h.maxUsers) {
			jsonError(c, http.StatusConflict, "registration is closed — maximum number of users reached")
			return
		}
	}
	// Use DisplayName from the passkey system as the username.
	username := strings.TrimSpace(res.DisplayName)
	if username == "" {
		username = "user"
	}
	user := model.User{
		Username:      username,
		PasskeyUserID: res.PasskeyUserID,
	}
	if err := h.db.Create(&user).Error; err != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "create user failed", gin.H{"stage": "user_create"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "user_id": user.ID, "username": user.Username})
}

// PasskeyVerify verifies a WebAuthn assertion and creates a passkey session.
// POST /api/auth/passkey/verify
func (h *AuthHandler) PasskeyVerify(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		jsonError(c, http.StatusBadRequest, "invalid json")
		return
	}
	loginIDRaw := body["login_session_id"]
	loginID, ok := toUint64(loginIDRaw)
	if !ok {
		jsonError(c, http.StatusBadRequest, "login_session_id is required")
		return
	}
	credBytes, err := json.Marshal(body["credential"])
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid credential")
		return
	}
	res, err := h.sdk.PasskeyLoginVerify(c.Request.Context(), loginID, credBytes)
	if err != nil || !res.Success {
		msg := "login failed"
		if err != nil {
			msg = err.Error()
		} else if res != nil {
			msg = res.Error
		}
		jsonError(c, http.StatusUnauthorized, msg)
		return
	}
	// Extract UMS token and passkey_user_id from response
	umsToken, _ := res.Data["token"].(string)
	passkeyUserIDRaw := res.Data["passkey_user_id"]
	passkeyUserID, _ := toUint64(passkeyUserIDRaw)

	// Find or auto-create local user by passkey_user_id.
	// UMS is the auth authority — a valid passkey login means the user is legitimate.
	var user model.User
	if err := h.db.Where("passkey_user_id = ?", passkeyUserID).First(&user).Error; err != nil {
		// Enforce user limit before auto-creating.
		if h.maxUsers > 0 {
			var userCount int64
			h.db.Model(&model.User{}).Count(&userCount)
			if userCount >= int64(h.maxUsers) {
				jsonError(c, http.StatusConflict, "registration is closed — maximum number of users reached")
				return
			}
		}
		displayName, _ := res.Data["display_name"].(string)
		username := strings.TrimSpace(displayName)
		if username == "" {
			username = fmt.Sprintf("user_%d", passkeyUserID)
		}
		user = model.User{Username: username, PasskeyUserID: uint(passkeyUserID)}
		if err := h.db.Create(&user).Error; err != nil {
			slog.Error("auto-create user failed", "passkey_user_id", passkeyUserID, "error", err)
			jsonErrorDetails(c, http.StatusInternalServerError, "failed to create user", gin.H{"stage": "user_auto_create"})
			return
		}
		slog.Info("auto-created local user", "user_id", user.ID, "passkey_user_id", passkeyUserID, "username", username)
	}

	// Generate a local session token (ps_ prefix)
	sessionToken := "ps_" + randomHex(24)
	csrfToken := h.sessions.Set(sessionToken, user.ID, 24*time.Hour)

	writeAuditLog(h.db, user.ID, "login", "success", "passkey", c.ClientIP(), nil, nil, "")
	c.JSON(http.StatusOK, gin.H{
		"success":       true,
		"session_token": sessionToken,
		"csrf_token":    csrfToken,
		"user_id":       user.ID,
		"username":      user.Username,
		"ums_token":     umsToken, // passed to frontend for passkey approval flows
	})
}

// GenerateAPIKey generates a new API key for the authenticated passkey user.
// POST /api/auth/apikey/generate
func (h *AuthHandler) GenerateAPIKey(c *gin.Context) {
	var req struct {
		LoginSessionID uint64      `json:"login_session_id"`
		Credential     interface{} `json:"credential"`
		Label          string      `json:"label"`
	}
	_ = c.ShouldBindJSON(&req)
	if !verifyFreshPasskeyParsed(h.sdk, c, req.LoginSessionID, req.Credential, h.db) {
		return
	}
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}

	// Enforce per-user limit.
	var count int64
	if err := h.db.Model(&model.APIKey{}).Where("user_id = ?", userID).Count(&count).Error; err != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "db error", gin.H{"stage": "apikey_count"})
		return
	}
	if count >= int64(h.maxAPIKeys) {
		jsonError(c, http.StatusConflict, fmt.Sprintf("maximum %d API keys per user", h.maxAPIKeys))
		return
	}

	rawKey := "ocw_" + randomHex(32)
	salt := randomHex(16)
	hash := hashAPIKeyWithSalt(rawKey, salt)
	prefix := rawKey[:12] // "ocw_" + 8 chars

	label := strings.TrimSpace(req.Label)

	apiKey := model.APIKey{
		UserID:  userID,
		KeyHash: hash,
		KeySalt: salt,
		Prefix:  prefix,
		Label:   label,
	}
	if err := h.db.Create(&apiKey).Error; err != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "failed to save key", gin.H{"stage": "apikey_save"})
		return
	}
	writeAuditCtx(h.db, c, "apikey_generate", "success", nil, map[string]interface{}{"prefix": prefix, "label": label})
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"api_key":    rawKey, // shown once only
		"api_prefix": prefix,
		"label":      label,
		"note":       "store this key securely — it will not be shown again",
	})
}

// ListAPIKeys lists the current user's API key prefixes.
// GET /api/auth/apikey/list
func (h *AuthHandler) ListAPIKeys(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	var apiKeys []model.APIKey
	if err := h.db.Where("user_id = ?", userID).Order("created_at asc").Find(&apiKeys).Error; err != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "db error", gin.H{"stage": "apikey_list"})
		return
	}
	keys := []gin.H{}
	for _, k := range apiKeys {
		keys = append(keys, gin.H{
			"id":         k.ID,
			"prefix":     k.Prefix,
			"label":      k.Label,
			"created_at": k.CreatedAt,
		})
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "keys": keys})
}

// RenameAPIKey changes the label of an API key.
// PATCH /api/auth/apikey  (Passkey only)
func (h *AuthHandler) RenameAPIKey(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	var req struct {
		Prefix string `json:"prefix" binding:"required"`
		Label  string `json:"label" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "prefix and label are required")
		return
	}

	var apiKey model.APIKey
	if err := h.db.Where("user_id = ? AND prefix = ?", userID, req.Prefix).First(&apiKey).Error; err != nil {
		jsonError(c, http.StatusNotFound, "API key not found")
		return
	}

	oldLabel := apiKey.Label
	if err := h.db.Model(&apiKey).Update("label", req.Label).Error; err != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "rename failed", gin.H{"stage": "apikey_rename"})
		return
	}
	writeAuditCtx(h.db, c, "apikey_rename", "success", nil, map[string]interface{}{
		"prefix":    req.Prefix,
		"old_label": oldLabel,
		"new_label": req.Label,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "label": req.Label})
}

// Logout invalidates the current passkey session immediately.
// DELETE /api/auth/session  (Passkey only)
func (h *AuthHandler) Logout(c *gin.Context) {
	token, _ := c.Get("sessionToken")
	if t, ok := token.(string); ok && t != "" {
		h.sessions.Delete(t)
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DeleteAccount permanently deletes the user, all their wallets, and all TEE keys.
// DELETE /api/auth/account
func (h *AuthHandler) DeleteAccount(c *gin.Context) {
	if !verifyFreshPasskey(h.sdk, c, h.db) {
		return
	}
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}

	// Load all wallets for this user.
	var wallets []model.Wallet
	if err := h.db.Where("user_id = ?", userID).Find(&wallets).Error; err != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "failed to load wallets", gin.H{"stage": "account_delete"})
		return
	}

	// Delete each TEE key (best-effort, log failures).
	if h.sdk != nil {
		for _, w := range wallets {
			if w.KeyName == "" {
				continue
			}
			if _, err := h.sdk.DeletePublicKey(c.Request.Context(), w.KeyName); err != nil {
				slog.Error("DeletePublicKey failed", "user_id", userID, "key", w.KeyName, "error", err)
			}
		}
	}

	// Load user to get PasskeyUserID before deletion.
	var user model.User
	if err := h.db.First(&user, userID).Error; err != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "failed to load user", gin.H{"stage": "account_delete"})
		return
	}

	// Collect wallet IDs for cascade deletion.
	walletIDs := make([]string, 0, len(wallets))
	for _, w := range wallets {
		walletIDs = append(walletIDs, w.ID)
	}

	// Delete all related data, wallets, then user in a transaction.
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if len(walletIDs) > 0 {
			tx.Where("user_id = ?", userID).Delete(&model.AllowedContract{})
			tx.Where("wallet_id IN ?", walletIDs).Delete(&model.ApprovalPolicy{})
			tx.Where("wallet_id IN ?", walletIDs).Delete(&model.ApprovalRequest{})
		}
		tx.Where("user_id = ?", userID).Delete(&model.AddressBookEntry{})
		tx.Where("user_id = ?", userID).Delete(&model.AuditLog{})
		tx.Where("user_id = ?", userID).Delete(&model.APIKey{})
		if err := tx.Where("user_id = ?", userID).Delete(&model.Wallet{}).Error; err != nil {
			return err
		}
		return tx.Delete(&model.User{}, userID).Error
	}); err != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "failed to delete account", gin.H{"stage": "account_delete"})
		return
	}

	// Delete UMS PasskeyUser (best-effort).
	if h.sdk != nil && user.PasskeyUserID > 0 {
		if _, err := h.sdk.DeletePasskeyUser(c.Request.Context(), uint(user.PasskeyUserID)); err != nil {
			slog.Error("DeletePasskeyUser failed", "user_id", userID, "passkey_user_id", user.PasskeyUserID, "error", err)
		}
	}

	// Revoke session.
	token, _ := c.Get("sessionToken")
	if t, ok := token.(string); ok && t != "" {
		h.sessions.Delete(t)
	}

	c.JSON(http.StatusOK, gin.H{"success": true})
}

// RevokeAPIKey revokes the current user's API key.
// DELETE /api/auth/apikey?prefix=ocw_xxxx  (query param preferred)
// Also accepts {"api_key_prefix":"ocw_xxxx"} in the request body for backwards compatibility.
func (h *AuthHandler) RevokeAPIKey(c *gin.Context) {
	if !verifyFreshPasskey(h.sdk, c, h.db) {
		return
	}
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}

	// Accept prefix from query parameter or JSON body.
	prefix := c.Query("prefix")
	if prefix == "" {
		var req struct {
			APIKeyPrefix string `json:"api_key_prefix"`
		}
		_ = c.ShouldBindJSON(&req)
		prefix = req.APIKeyPrefix
	}
	if prefix == "" {
		jsonError(c, http.StatusBadRequest, "prefix is required (query param or body)")
		return
	}

	var apiKey model.APIKey
	if err := h.db.Where("user_id = ? AND prefix = ?", userID, prefix).First(&apiKey).Error; err != nil {
		jsonError(c, http.StatusNotFound, "API key not found")
		return
	}

	if err := h.db.Delete(&apiKey).Error; err != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "revoke failed", gin.H{"stage": "apikey_revoke"})
		return
	}
	writeAuditCtx(h.db, c, "apikey_revoke", "success", nil, map[string]interface{}{"prefix": prefix, "label": apiKey.Label})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// mustUserID returns the authenticated user's ID from the gin context.
// If the userID is missing (should never happen when AuthMiddleware runs first),
// it aborts the request with a 500 error and returns 0.
// Callers must check c.IsAborted() after calling and return early if true.
func mustUserID(c *gin.Context) uint {
	v, ok := c.Get("userID")
	if !ok {
		slog.Error("BUG: mustUserID called without userID in context")
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal auth error"})
		return 0
	}
	id, _ := v.(uint)
	return id
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return hex.EncodeToString(b)
}

func toUint64(v interface{}) (uint64, bool) {
	switch n := v.(type) {
	case float64:
		if n <= 0 {
			return 0, false
		}
		return uint64(n), true
	case json.Number:
		i, err := n.Int64()
		if err != nil || i <= 0 {
			return 0, false
		}
		return uint64(i), true
	case string:
		u, err := strconv.ParseUint(strings.TrimSpace(n), 10, 64)
		if err != nil || u == 0 {
			return 0, false
		}
		return u, true
	case int:
		if n <= 0 {
			return 0, false
		}
		return uint64(n), true
	case uint:
		return uint64(n), n > 0
	case uint64:
		return n, n > 0
	}
	return 0, false
}

