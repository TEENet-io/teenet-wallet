package handler

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sdk "github.com/TEENet-io/teenet-sdk/go"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/chain"
	"github.com/TEENet-io/teenet-wallet/model"
)

var nicknameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// AddressBookHandler manages per-user address book entries.
type AddressBookHandler struct {
	db             *gorm.DB
	sdk            *sdk.Client
	approvalExpiry time.Duration
}

// NewAddressBookHandler creates a new AddressBookHandler.
func NewAddressBookHandler(db *gorm.DB, sdkClient *sdk.Client, approvalExpiry ...time.Duration) *AddressBookHandler {
	expiry := 30 * time.Minute
	if len(approvalExpiry) > 0 && approvalExpiry[0] > 0 {
		expiry = approvalExpiry[0]
	}
	return &AddressBookHandler{db: db, sdk: sdkClient, approvalExpiry: expiry}
}

// ListEntries returns all address book entries for the authenticated user.
// Optional query params: nickname (lowercase match), chain.
// GET /api/addressbook
func (h *AddressBookHandler) ListEntries(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}

	q := h.db.Where("user_id = ?", userID)
	if nickname := strings.TrimSpace(c.Query("nickname")); nickname != "" {
		q = q.Where("nickname = ?", strings.ToLower(nickname))
	}
	if chainName := strings.TrimSpace(c.Query("chain")); chainName != "" {
		q = q.Where("chain = ?", chainName)
	}

	var entries []model.AddressBookEntry
	if err := q.Order("nickname asc, chain asc").Find(&entries).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "db error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "entries": entries})
}

// AddEntry creates a new address book entry.
// POST /api/addressbook
func (h *AddressBookHandler) AddEntry(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}

	var req struct {
		LoginSessionID uint64      `json:"login_session_id"`
		Credential     interface{} `json:"credential"`
		Nickname       string      `json:"nickname" binding:"required"`
		Chain          string      `json:"chain" binding:"required"`
		Address        string      `json:"address" binding:"required"`
		Memo           string      `json:"memo"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Normalize and validate nickname.
	nickname := strings.ToLower(strings.TrimSpace(req.Nickname))
	if len(nickname) > 100 {
		jsonError(c, http.StatusBadRequest, "nickname must be at most 100 characters")
		return
	}
	if !nicknameRe.MatchString(nickname) {
		jsonError(c, http.StatusBadRequest, "nickname must start with a letter or digit and contain only lowercase letters, digits, hyphens, or underscores")
		return
	}

	// Validate chain exists.
	if _, ok := model.Chains[req.Chain]; !ok {
		jsonError(c, http.StatusBadRequest, "unsupported chain")
		return
	}

	// Validate and normalize address.
	addr, err := validateAddress(req.Address, req.Chain)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "address: "+err.Error())
		return
	}

	// Uniqueness check: (user_id, nickname, chain).
	var count int64
	h.db.Model(&model.AddressBookEntry{}).Where("user_id = ? AND nickname = ? AND chain = ?", userID, nickname, req.Chain).Count(&count)
	if count > 0 {
		jsonError(c, http.StatusConflict, "an entry with this nickname already exists for this chain")
		return
	}

	memo := strings.TrimSpace(req.Memo)
	if len(memo) > 256 {
		jsonError(c, http.StatusBadRequest, "memo must be at most 256 characters")
		return
	}

	proposed := model.AddressBookEntry{
		UserID:   userID,
		Nickname: nickname,
		Chain:    req.Chain,
		Address:  addr,
		Memo:     memo,
	}

	// API key path: create pending approval.
	if !isPasskeyAuth(c) {
		approval, created := createPendingApproval(h.db, c, nil, "addressbook_add", proposed, h.approvalExpiry)
		if !created {
			return
		}
		writeAuditCtx(h.db, c, "addressbook_add", "pending", nil, map[string]interface{}{
			"nickname": nickname, "chain": req.Chain, "address": addr, "approval_id": approval.ID,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"success":     true,
			"pending":     true,
			"approval_id": approval.ID,
			"message":     "Address book entry submitted for approval",
		})
		return
	}

	// Passkey path: require a fresh hardware assertion before applying.
	if !verifyFreshPasskeyParsed(h.sdk, c, req.LoginSessionID, req.Credential, h.db) {
		return
	}

	if err := h.db.Create(&proposed).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE") {
			jsonError(c, http.StatusConflict, "an entry with this nickname already exists for this chain")
			return
		}
		jsonError(c, http.StatusInternalServerError, "db error")
		return
	}

	writeAuditCtx(h.db, c, "addressbook_add", "success", nil, map[string]interface{}{
		"nickname": nickname, "chain": req.Chain, "address": addr,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "entry": proposed})
}

// UpdateEntry modifies an existing address book entry.
// PUT /api/addressbook/:id
func (h *AddressBookHandler) UpdateEntry(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}

	entryID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid entry id")
		return
	}

	var existing model.AddressBookEntry
	if err := h.db.Where("id = ? AND user_id = ?", entryID, userID).First(&existing).Error; err != nil {
		jsonError(c, http.StatusNotFound, "address book entry not found")
		return
	}

	var req struct {
		LoginSessionID uint64      `json:"login_session_id"`
		Credential     interface{} `json:"credential"`
		Nickname       *string     `json:"nickname"`
		Address        *string     `json:"address"`
		Memo           *string     `json:"memo"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Build proposed state by merging changes into existing.
	proposed := existing
	updates := map[string]interface{}{}

	if req.Nickname != nil {
		nickname := strings.ToLower(strings.TrimSpace(*req.Nickname))
		if len(nickname) > 100 {
			jsonError(c, http.StatusBadRequest, "nickname must be at most 100 characters")
			return
		}
		if !nicknameRe.MatchString(nickname) {
			jsonError(c, http.StatusBadRequest, "nickname must start with a letter or digit and contain only lowercase letters, digits, hyphens, or underscores")
			return
		}
		// Uniqueness check excluding self: (user_id, nickname, chain) where id != self.
		var count int64
		h.db.Model(&model.AddressBookEntry{}).
			Where("user_id = ? AND nickname = ? AND chain = ? AND id != ?", userID, nickname, existing.Chain, existing.ID).
			Count(&count)
		if count > 0 {
			jsonError(c, http.StatusConflict, "an entry with this nickname already exists for this chain")
			return
		}
		proposed.Nickname = nickname
		updates["nickname"] = nickname
	}

	if req.Address != nil {
		addr, err := validateAddress(*req.Address, existing.Chain)
		if err != nil {
			jsonError(c, http.StatusBadRequest, "address: "+err.Error())
			return
		}
		proposed.Address = addr
		updates["address"] = addr
	}

	if req.Memo != nil {
		memo := strings.TrimSpace(*req.Memo)
		if len(memo) > 256 {
			jsonError(c, http.StatusBadRequest, "memo must be at most 256 characters")
			return
		}
		proposed.Memo = memo
		updates["memo"] = memo
	}

	if len(updates) == 0 {
		jsonError(c, http.StatusBadRequest, "no fields to update")
		return
	}

	// API key path: create pending approval.
	if !isPasskeyAuth(c) {
		approval, created := createPendingApproval(h.db, c, nil, "addressbook_update", proposed, h.approvalExpiry)
		if !created {
			return
		}
		writeAuditCtx(h.db, c, "addressbook_update", "pending", nil, map[string]interface{}{
			"entry_id": entryID, "nickname": proposed.Nickname, "address": proposed.Address, "approval_id": approval.ID,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"success":     true,
			"pending":     true,
			"approval_id": approval.ID,
			"message":     "Address book update submitted for approval",
		})
		return
	}

	// Passkey path: require a fresh hardware assertion before applying.
	if !verifyFreshPasskeyParsed(h.sdk, c, req.LoginSessionID, req.Credential, h.db) {
		return
	}

	if err := h.db.Model(&existing).Updates(updates).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE") {
			jsonError(c, http.StatusConflict, "an entry with this nickname already exists for this chain")
			return
		}
		jsonError(c, http.StatusInternalServerError, "update failed")
		return
	}

	writeAuditCtx(h.db, c, "addressbook_update", "success", nil, map[string]interface{}{
		"entry_id": entryID, "nickname": proposed.Nickname, "address": proposed.Address,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "entry": proposed})
}

// DeleteEntry removes an address book entry.
// DELETE /api/addressbook/:id (Passkey only)
func (h *AddressBookHandler) DeleteEntry(c *gin.Context) {
	if !verifyFreshPasskey(h.sdk, c, h.db) {
		return
	}

	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}

	entryID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid entry id")
		return
	}

	var entry model.AddressBookEntry
	if err := h.db.Where("id = ? AND user_id = ?", entryID, userID).First(&entry).Error; err != nil {
		jsonError(c, http.StatusNotFound, "address book entry not found")
		return
	}

	if err := h.db.Delete(&entry).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "delete failed")
		return
	}

	writeAuditCtx(h.db, c, "addressbook_delete", "success", nil, map[string]interface{}{
		"entry_id": entryID, "nickname": entry.Nickname, "chain": entry.Chain, "address": entry.Address,
	})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// validateAddress validates and normalizes an address for the given chain.
func validateAddress(addr, chainName string) (string, error) {
	chainCfg, ok := model.Chains[chainName]
	if !ok {
		return "", fmt.Errorf("unsupported chain")
	}

	switch chainCfg.Family {
	case "solana":
		addr = strings.TrimSpace(addr)
		pub, err := chain.Base58Decode(addr)
		if err != nil || len(pub) != 32 {
			return "", fmt.Errorf("invalid Solana address")
		}
		return addr, nil
	default: // evm
		normalized, err := normalizeEVMAddress(addr)
		if err != nil {
			return "", err
		}
		return normalized, nil
	}
}

// ResolveNickname looks up an address book entry by (userID, nickname, chain)
// and returns the stored address. Returns an error if not found.
func ResolveNickname(db *gorm.DB, userID uint, nickname, chainName string) (string, error) {
	var entry model.AddressBookEntry
	err := db.Where("user_id = ? AND nickname = ? AND chain = ?", userID, strings.ToLower(nickname), chainName).First(&entry).Error
	if err != nil {
		return "", fmt.Errorf("address book entry not found for nickname %q on chain %q", nickname, chainName)
	}
	return entry.Address, nil
}

// LooksLikeAddress returns true if s looks like a raw on-chain address
// rather than a nickname, based on the chain family.
func LooksLikeAddress(s, family string) bool {
	switch family {
	case "evm":
		return strings.HasPrefix(s, "0x") && len(s) == 42
	case "solana":
		n := len(s)
		return n >= 32 && n <= 44 && !strings.ContainsAny(s, "_-")
	default:
		return false
	}
}
