package handler

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/model"
)

// respondPendingApproval sends a unified 202 response for any operation
// that requires Passkey approval. All approval-returning endpoints use this
// to ensure a consistent response shape for the plugin/agent.
func respondPendingApproval(c *gin.Context, baseURL string, approvalID uint, message string) {
	approvalURL := fmt.Sprintf("%s/#/approve/%d", requestBaseURL(c, baseURL), approvalID)
	c.JSON(http.StatusAccepted, gin.H{
		"success":      true,
		"status":       "pending_approval",
		"approval_id":  approvalID,
		"approval_url": approvalURL,
		"message":      message,
	})
}

// isPasskeyAuth reports whether the request was authenticated via a passkey session.
func isPasskeyAuth(c *gin.Context) bool {
	authMode, _ := c.Get("authMode")
	return authMode == "passkey"
}

// normalizeEVMAddress lowercases and validates an EVM address string.
// Returns the normalized address or an error describing the problem.
func normalizeEVMAddress(addr string) (string, error) {
	addr = strings.ToLower(strings.TrimSpace(addr))
	if !strings.HasPrefix(addr, "0x") || len(addr) != 42 {
		return "", fmt.Errorf("must be a 20-byte hex address (0x...)")
	}
	if _, err := hex.DecodeString(addr[2:]); err != nil {
		return "", fmt.Errorf("contains invalid hex characters")
	}
	return addr, nil
}

// utcStartOfDay returns midnight of the current UTC day.
func utcStartOfDay() time.Time {
	now := time.Now().UTC()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

// authInfo extracts authMode and apiKeyPrefix from the gin context.
func authInfo(c *gin.Context) (authMode, apiKeyPrefix string) {
	if am, ok := c.Get("authMode"); ok {
		authMode, _ = am.(string)
	}
	if p, ok := c.Get("apiKeyPrefix"); ok {
		apiKeyPrefix, _ = p.(string)
	}
	return
}

// createPendingApproval inserts a pending ApprovalRequest into the DB.
// On success it returns the created record and true.
// On failure it writes a 500 error response and returns nil, false — the caller must return immediately.
func createPendingApproval(db *gorm.DB, c *gin.Context, walletID *string, approvalType string, policyData interface{}, expiry ...time.Duration) (*model.ApprovalRequest, bool) {
	policyJSON, _ := json.Marshal(policyData)
	userID := mustUserID(c)
	if c.IsAborted() {
		return nil, false
	}
	expiryDur := 30 * time.Minute
	if len(expiry) > 0 && expiry[0] > 0 {
		expiryDur = expiry[0]
	}
	authMode, apiKeyPrefix := authInfo(c)
	approval := model.ApprovalRequest{
		WalletID:     walletID,
		UserID:       userID,
		ApprovalType: approvalType,
		PolicyData:   string(policyJSON),
		Status:       "pending",
		AuthMode:     authMode,
		APIKeyPrefix: apiKeyPrefix,
		ExpiresAt:    time.Now().Add(expiryDur),
	}
	if err := db.Create(&approval).Error; err != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "failed to create approval request", gin.H{"stage": "create_approval"})
		return nil, false
	}
	return &approval, true
}
