package handler

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/model"
)

// AuditHandler serves the audit log API.
type AuditHandler struct {
	db *gorm.DB
}

func NewAuditHandler(db *gorm.DB) *AuditHandler {
	return &AuditHandler{db: db}
}

// ListLogs returns a paginated list of the current user's audit logs.
// GET /api/audit/logs?page=1&limit=20&action=transfer&wallet_id=5
func (h *AuditHandler) ListLogs(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "30"))
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 30
	}
	offset := (page - 1) * limit

	q := h.db.Model(&model.AuditLog{}).Where("user_id = ?", userID)
	if action := c.Query("action"); action != "" {
		q = q.Where("action = ?", action)
	}
	if wid := c.Query("wallet_id"); wid != "" {
		q = q.Where("wallet_id = ?", wid)
	}

	var total int64
	q.Count(&total)

	var logs []model.AuditLog
	if err := q.Order("created_at desc").Limit(limit).Offset(offset).Find(&logs).Error; err != nil {
		slog.Error("list audit logs failed", "user_id", userID, "error", err)
		jsonErrorDetails(c, http.StatusInternalServerError, "db error", gin.H{"stage": "list_audit_logs", "user_id": userID})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"logs":    logs,
		"total":   total,
		"page":    page,
		"limit":   limit,
	})
}

// ─── Audit helpers ─────────────────────────────────────────────────────────────

// writeAuditLog writes a best-effort audit entry. Errors are logged to stdout but not returned.
// The optional approvalID parameter sets the ApprovalID column for indexed lookup.
func writeAuditLog(db *gorm.DB, userID uint, action, status, authMode, ip string, walletID *string, details interface{}, apiKeyPrefix string, approvalID ...uint) {
	entry := model.AuditLog{
		UserID:       userID,
		Action:       action,
		Status:       status,
		AuthMode:     authMode,
		IP:           ip,
		WalletID:     walletID,
		APIKeyPrefix: apiKeyPrefix,
		CreatedAt:   time.Now(),
	}
	if len(approvalID) > 0 && approvalID[0] > 0 {
		entry.ApprovalID = &approvalID[0]
	}
	if details != nil {
		b, _ := json.Marshal(details)
		entry.Details = string(b)
	}
	if err := db.Create(&entry).Error; err != nil {
		slog.Error("audit write failed", "user_id", userID, "action", action, "error", err)
	}
}

// updateAuditByApprovalID finds the existing pending audit log for the given approval ID
// and updates it in-place with the new status, approval timestamp, and merged details.
// This avoids creating two separate records for the same approval flow.
func updateAuditByApprovalID(db *gorm.DB, approvalID uint, newStatus string, extraDetails map[string]interface{}) {
	var log model.AuditLog
	if err := db.Where("approval_id = ? AND status = ?", approvalID, "pending").First(&log).Error; err != nil {
		slog.Error("audit update: pending log not found", "approval_id", approvalID, "error", err)
		return
	}

	// Merge extra details into existing details JSON.
	var existing map[string]interface{}
	if log.Details != "" {
		_ = json.Unmarshal([]byte(log.Details), &existing)
	}
	if existing == nil {
		existing = make(map[string]interface{})
	}
	for k, v := range extraDetails {
		existing[k] = v
	}
	merged, _ := json.Marshal(existing)

	now := time.Now()
	if err := db.Model(&log).Updates(map[string]interface{}{
		"status":      newStatus,
		"approved_at": now,
		"details":     string(merged),
	}).Error; err != nil {
		slog.Error("audit update failed", "approval_id", approvalID, "error", err)
	}
}

// writeAuditCtx is a convenience wrapper that extracts userID/authMode/IP from the gin context.
// The optional approvalID parameter sets the ApprovalID column for indexed lookup.
func writeAuditCtx(db *gorm.DB, c *gin.Context, action, status string, walletID *string, details interface{}, approvalID ...uint) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	authMode, apiKeyPrefix := authInfo(c)
	writeAuditLog(db, userID, action, status, authMode, c.ClientIP(), walletID, details, apiKeyPrefix, approvalID...)
}
