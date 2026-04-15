// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	sdk "github.com/TEENet-io/teenet-sdk/go"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/chain"
	"github.com/TEENet-io/teenet-wallet/model"
)

// walletShards provides per-wallet mutexes using a fixed shard array to prevent
// unbounded memory growth. Uses FNV hash to distribute wallet IDs across shards.
const walletShardCount = 256

var walletShards [walletShardCount]sync.Mutex

func getWalletMutex(walletID string) *sync.Mutex {
	h := fnv.New32a()
	h.Write([]byte(walletID))
	return &walletShards[h.Sum32()%walletShardCount]
}

// WalletHandler handles wallet CRUD, signing, and on-chain transfers.
type WalletHandler struct {
	db             *gorm.DB
	sdk            *sdk.Client
	baseURL        string // used to build approval_url
	approvalExpiry time.Duration
	maxWallets     int
	idempotency    *IdempotencyStore
	prices         *PriceService
}

func NewWalletHandler(db *gorm.DB, sdkClient *sdk.Client, baseURL string, approvalExpiry ...time.Duration) *WalletHandler {
	expiry := 30 * time.Minute
	if len(approvalExpiry) > 0 && approvalExpiry[0] > 0 {
		expiry = approvalExpiry[0]
	}
	return &WalletHandler{db: db, sdk: sdkClient, baseURL: baseURL, approvalExpiry: expiry, maxWallets: 10}
}

// SetMaxWallets sets the maximum number of wallets a user can create.
func (h *WalletHandler) SetMaxWallets(n int) { h.maxWallets = n }

// SetPriceService sets the USD price service used for threshold conversion.
func (h *WalletHandler) SetPriceService(ps *PriceService) { h.prices = ps }

// StartReaper runs a background goroutine that marks wallets stuck in "creating"
// status for more than 10 minutes as "error". It stops when ctx is cancelled.
func (h *WalletHandler) StartReaper(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cutoff := time.Now().Add(-10 * time.Minute)
				result := h.db.Model(&model.Wallet{}).
					Where("status = ? AND created_at < ?", "creating", cutoff).
					Update("status", "error")
				if result.RowsAffected > 0 {
					slog.Warn("reaped zombie wallets", "count", result.RowsAffected)
				}
			}
		}
	}()
}

// CreateWallet creates a new TEE-backed wallet on the specified chain.
// POST /api/wallets
func (h *WalletHandler) CreateWallet(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	var req struct {
		Chain string `json:"chain" binding:"required"`
		Label string `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	req.Chain = strings.ToLower(strings.TrimSpace(req.Chain))
	chainCfg, ok := model.GetChain(req.Chain)
	if !ok {
		jsonError(c, http.StatusBadRequest, "unsupported chain: "+req.Chain)
		return
	}

	// Enforce per-user wallet limit to prevent TEE DKG abuse.
	// Exclude wallets in "error" status — they are non-functional and should not count.
	var count int64
	h.db.Model(&model.Wallet{}).Where("user_id = ? AND status != ?", userID, "error").Count(&count)
	if count >= int64(h.maxWallets) {
		jsonError(c, http.StatusBadRequest, fmt.Sprintf("wallet limit reached (max %d)", h.maxWallets))
		return
	}

	// Enforce unique label per user.
	label := strings.TrimSpace(req.Label)
	if label != "" {
		var dup int64
		h.db.Model(&model.Wallet{}).Where("user_id = ? AND label = ?", userID, label).Count(&dup)
		if dup > 0 {
			jsonError(c, http.StatusConflict, "a wallet with this label already exists")
			return
		}
	}

	// Create a pending wallet record immediately so the user can see progress.
	// Use a temporary unique KeyName to satisfy the uniqueIndex constraint.
	// It will be replaced with the real TEE key name after DKG completes.
	walletID := uuid.New().String()
	wallet := model.Wallet{
		ID:        walletID,
		UserID:    userID,
		Chain:     req.Chain,
		Label:     label,
		KeyName:   "pending-" + walletID,
		Curve:     chainCfg.Curve,
		Protocol:  chainCfg.Protocol,
		Status:    "creating",
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(&wallet).Error; err != nil {
		slog.Error("wallet create db insert failed", "user_id", userID, "chain", req.Chain, "error", err)
		jsonErrorDetails(c, http.StatusInternalServerError, "db error", gin.H{"stage": "wallet_create", "chain": req.Chain})
		return
	}

	// Generate key via TEE-DAO (may take 1-2 min for ECDSA).
	keyResult, genErr := h.sdk.GenerateKey(c.Request.Context(), chainCfg.Protocol, chainCfg.Curve)
	if genErr != nil || !keyResult.Success {
		msg := "key generation failed"
		if genErr != nil {
			msg = genErr.Error()
		} else if keyResult != nil {
			msg = keyResult.Message
		}
		slog.Error("wallet key generation failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "protocol", chainCfg.Protocol, "error", msg)
		h.db.Model(&wallet).Updates(map[string]interface{}{"status": "error"})
		jsonErrorDetails(c, http.StatusUnprocessableEntity, msg, gin.H{
			"stage": "key_generation", "wallet_id": wallet.ID, "chain": wallet.Chain,
			"protocol": chainCfg.Protocol, "curve": chainCfg.Curve,
		})
		return
	}

	// Derive chain address from public key.
	address, addrErr := chain.DeriveAddress(chainCfg.Family, keyResult.PublicKey.KeyData)
	if addrErr != nil {
		slog.Error("wallet address derivation failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "error", addrErr)
		h.db.Model(&wallet).Updates(map[string]interface{}{"status": "error"})
		jsonErrorDetails(c, http.StatusInternalServerError, "address derivation failed: "+addrErr.Error(), gin.H{"stage": "address_derivation", "wallet_id": wallet.ID, "chain": wallet.Chain})
		return
	}

	// Update wallet with final data.
	if err := h.db.Model(&wallet).Updates(map[string]interface{}{
		"key_name":   keyResult.PublicKey.Name,
		"public_key": keyResult.PublicKey.KeyData,
		"address":    address,
		"status":     "ready",
	}).Error; err != nil {
		slog.Error("wallet db update failed after key generation", "wallet_id", wallet.ID, "chain", wallet.Chain, "error", err)
		jsonErrorDetails(c, http.StatusInternalServerError, "db update failed", gin.H{"stage": "wallet_create_update", "wallet_id": wallet.ID, "chain": wallet.Chain})
		return
	}
	wallet.KeyName = keyResult.PublicKey.Name
	wallet.PublicKey = keyResult.PublicKey.KeyData
	wallet.Address = address
	wallet.Status = "ready"

	writeAuditCtx(h.db, c, "wallet_create", "success", &wallet.ID, map[string]interface{}{
		"chain": wallet.Chain, "address": wallet.Address, "label": wallet.Label,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "wallet": wallet})
}

// ListWallets returns all wallets for the current user.
// GET /api/wallets
func (h *WalletHandler) ListWallets(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	var wallets []model.Wallet
	if err := h.db.Where("user_id = ?", userID).Order("created_at desc").Find(&wallets).Error; err != nil {
		slog.Error("list wallets db query failed", "user_id", userID, "error", err)
		jsonErrorDetails(c, http.StatusInternalServerError, "db error", gin.H{"stage": "list_wallets"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "wallets": wallets})
}

// GetWallet returns details of a single wallet.
// GET /api/wallets/:id
func (h *WalletHandler) GetWallet(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "wallet": wallet})
}

// RenameWallet updates the label of a wallet.
// PATCH /api/wallets/:id
func (h *WalletHandler) RenameWallet(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	var req struct {
		Label string `json:"label" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "label is required")
		return
	}
	label := strings.TrimSpace(req.Label)
	if label == "" {
		jsonError(c, http.StatusBadRequest, "label must not be empty")
		return
	}
	if len(label) > 100 {
		jsonError(c, http.StatusBadRequest, "label must be at most 100 characters")
		return
	}
	// Enforce unique label per user.
	var dup int64
	h.db.Model(&model.Wallet{}).Where("user_id = ? AND label = ? AND id != ?", wallet.UserID, label, wallet.ID).Count(&dup)
	if dup > 0 {
		jsonError(c, http.StatusConflict, "a wallet with this label already exists")
		return
	}
	if err := h.db.Model(&wallet).Update("label", label).Error; err != nil {
		slog.Error("wallet rename failed", "wallet_id", wallet.ID, "label", label, "error", err)
		jsonErrorDetails(c, http.StatusInternalServerError, "update failed", gin.H{"stage": "wallet_rename", "wallet_id": wallet.ID})
		return
	}
	wallet.Label = label
	c.JSON(http.StatusOK, gin.H{"success": true, "wallet": wallet})
}

// DeleteWallet deletes a wallet record.
// DELETE /api/wallets/:id
func (h *WalletHandler) DeleteWallet(c *gin.Context) {
	if !verifyFreshPasskey(h.sdk, c, h.db) {
		return
	}
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("wallet_id = ?", wallet.ID).Delete(&model.ApprovalPolicy{}).Error; err != nil {
			return err
		}
		if err := tx.Where("wallet_id = ?", wallet.ID).Delete(&model.ApprovalRequest{}).Error; err != nil {
			return err
		}
		if err := tx.Delete(&wallet).Error; err != nil {
			return err
		}
		return nil
	}); err != nil {
		slog.Error("wallet deletion failed", "wallet_id", wallet.ID, "error", err)
		jsonErrorDetails(c, http.StatusInternalServerError, "failed to delete wallet", gin.H{"stage": "delete_wallet", "wallet_id": wallet.ID})
		return
	}
	// Best-effort: delete the TEE key. Log on failure but don't block the response.
	if h.sdk != nil && wallet.KeyName != "" {
		if _, err := h.sdk.DeletePublicKey(c.Request.Context(), wallet.KeyName); err != nil {
			slog.Error("DeletePublicKey failed", "wallet_id", wallet.ID, "key", wallet.KeyName, "error", err)
		}
	}
	writeAuditCtx(h.db, c, "wallet_delete", "success", &wallet.ID, map[string]interface{}{
		"chain": wallet.Chain, "address": wallet.Address,
	})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// GetPubkey returns the raw public key of a wallet.
// GET /api/wallets/:id/pubkey
func (h *WalletHandler) GetPubkey(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"wallet_id":  wallet.ID,
		"public_key": wallet.PublicKey,
		"address":    wallet.Address,
		"chain":      wallet.Chain,
		"curve":      wallet.Curve,
		"protocol":   wallet.Protocol,
	})
}

// SignRequest is used internally by transfer and contract-call approval flows.
type SignRequest struct {
	Message   string                 `json:"message" binding:"required"` // hex or base64
	Encoding  string                 `json:"encoding"`                   // "hex" (default) or "base64"
	TxContext map[string]interface{} `json:"tx_context"`
}

// SetPolicy upserts the USD-denominated approval policy for a wallet.
// PUT /api/wallets/:id/policy
func (h *WalletHandler) SetPolicy(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	var req struct {
		LoginSessionID uint64      `json:"login_session_id"`
		Credential     interface{} `json:"credential"`
		ThresholdUSD   string      `json:"threshold_usd" binding:"required"`
		DailyLimitUSD  string      `json:"daily_limit_usd"`
		Enabled        *bool       `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	t, ok3 := new(big.Float).SetString(req.ThresholdUSD)
	if !ok3 || t.Sign() <= 0 {
		jsonError(c, http.StatusBadRequest, "threshold_usd must be a positive number")
		return
	}
	if req.DailyLimitUSD != "" {
		dl, ok4 := new(big.Float).SetString(req.DailyLimitUSD)
		if !ok4 || dl.Sign() <= 0 {
			jsonError(c, http.StatusBadRequest, "daily_limit_usd must be a positive number")
			return
		}
	}

	// API key requests create a pending approval instead of applying directly.
	if !isPasskeyAuth(c) {
		proposed := model.ApprovalPolicy{
			WalletID:      wallet.ID,
			ThresholdUSD:  req.ThresholdUSD,
			Enabled:       enabled,
			DailyLimitUSD: req.DailyLimitUSD,
		}
		approval, created := createPendingApproval(h.db, c, &wallet.ID, "policy_change", proposed, h.approvalExpiry)
		if !created {
			return
		}
		writeAuditCtx(h.db, c, "policy_update", "pending", &wallet.ID, map[string]interface{}{
			"threshold_usd": req.ThresholdUSD, "approval_id": approval.ID,
		}, approval.ID)
		respondPendingApproval(c, h.baseURL, approval.ID, "Policy change submitted for approval")
		return
	}

	// Passkey path: require a fresh hardware assertion before applying.
	// IMPORTANT: Use verifyFreshPasskeyParsed (not verifyFreshPasskey) because
	// c.ShouldBindJSON already consumed the request body above.
	if !verifyFreshPasskeyParsed(h.sdk, c, req.LoginSessionID, req.Credential, h.db) {
		return
	}

	var policy model.ApprovalPolicy
	if h.db.Where("wallet_id = ?", wallet.ID).First(&policy).Error != nil {
		policy = model.ApprovalPolicy{WalletID: wallet.ID}
	}
	policy.ThresholdUSD = req.ThresholdUSD
	policy.Enabled = enabled
	policy.DailyLimitUSD = req.DailyLimitUSD

	if err := h.db.Save(&policy).Error; err != nil {
		slog.Error("save approval policy failed", "wallet_id", wallet.ID, "error", err)
		jsonErrorDetails(c, http.StatusInternalServerError, "save policy failed", gin.H{"stage": "save_policy", "wallet_id": wallet.ID})
		return
	}
	writeAuditCtx(h.db, c, "policy_update", "success", &wallet.ID, map[string]interface{}{
		"threshold_usd": req.ThresholdUSD, "daily_limit_usd": req.DailyLimitUSD,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "policy": policy})
}

// GetPolicy returns the USD approval policy for a wallet (one per wallet).
// GET /api/wallets/:id/policy
func (h *WalletHandler) GetPolicy(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	var policy model.ApprovalPolicy
	if err := h.db.Where("wallet_id = ?", wallet.ID).First(&policy).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "policy": nil})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "policy": policy})
}

// DeletePolicy deletes the approval policy for a wallet (Passkey only).
// DELETE /api/wallets/:id/policy
func (h *WalletHandler) DeletePolicy(c *gin.Context) {
	if !verifyFreshPasskey(h.sdk, c, h.db) {
		return
	}
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	result := h.db.Where("wallet_id = ?", wallet.ID).Delete(&model.ApprovalPolicy{})
	if result.Error != nil {
		slog.Error("delete approval policy failed", "wallet_id", wallet.ID, "error", result.Error)
		jsonErrorDetails(c, http.StatusInternalServerError, "delete failed", gin.H{"stage": "delete_policy", "wallet_id": wallet.ID})
		return
	}
	if result.RowsAffected == 0 {
		jsonError(c, http.StatusNotFound, "policy not found")
		return
	}
	writeAuditCtx(h.db, c, "policy_update", "success", &wallet.ID, map[string]interface{}{
		"action": "delete",
	})
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// DailySpent returns the current day's USD spend and limit info.
// GET /api/wallets/:id/daily-spent
func (h *WalletHandler) DailySpent(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}

	var policy model.ApprovalPolicy
	if err := h.db.Where("wallet_id = ? AND enabled = ?", wallet.ID, true).First(&policy).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{
			"daily_spent_usd": "0",
			"daily_limit_usd": "",
			"remaining_usd":   "",
			"reset_at":        "",
		})
		return
	}

	startOfDay := utcStartOfDay()
	spent := policy.DailySpentUSD
	if policy.DailyResetAt.Before(startOfDay) {
		spent = "0"
	}

	var remaining string
	if policy.DailyLimitUSD != "" {
		limit, _ := new(big.Float).SetString(policy.DailyLimitUSD)
		spentF, _ := new(big.Float).SetString(spent)
		if limit != nil && spentF != nil {
			rem := new(big.Float).Sub(limit, spentF)
			if rem.Sign() < 0 {
				rem = new(big.Float)
			}
			remaining = rem.Text('f', 6)
		}
	}

	resetAt := ""
	if policy.DailyLimitUSD != "" {
		nextReset := startOfDay.Add(24 * time.Hour)
		resetAt = nextReset.Format(time.RFC3339)
	}

	c.JSON(http.StatusOK, gin.H{
		"daily_spent_usd": spent,
		"daily_limit_usd": policy.DailyLimitUSD,
		"remaining_usd":   remaining,
		"reset_at":        resetAt,
	})
}

// createApprovalRequest creates a pending ApprovalRequest and returns a 202 response.
// txParams is optional: when set (non-empty), the approval handler will broadcast the tx after signing.
func (h *WalletHandler) createApprovalRequest(c *gin.Context, wallet model.Wallet, req SignRequest, msgBytes []byte, policy *model.ApprovalPolicy, txParams string) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}

	txContextJSON, jsonErr := json.Marshal(req.TxContext)
	if jsonErr != nil {
		slog.Error("marshal tx_context failed", "wallet_id", wallet.ID, "error", jsonErr)
		jsonErrorDetails(c, http.StatusInternalServerError, "marshal tx_context failed", gin.H{"stage": "marshal_tx_context", "wallet_id": wallet.ID})
		return
	}
	am, akl := authInfo(c)
	approval := model.ApprovalRequest{
		WalletID:     &wallet.ID,
		UserID:       userID,
		Message:      req.Message,
		TxContext:    string(txContextJSON),
		TxParams:     txParams,
		Status:       "pending",
		AuthMode:     am,
		APIKeyPrefix: akl,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(h.approvalExpiry),
	}
	if err := h.db.Create(&approval).Error; err != nil {
		slog.Error("create approval request failed", "wallet_id", wallet.ID, "error", err)
		jsonErrorDetails(c, http.StatusInternalServerError, "create approval request failed", gin.H{"stage": "create_approval", "wallet_id": wallet.ID})
		return
	}

	approvalURL := fmt.Sprintf("%s/#/approve/%d", requestBaseURL(c, h.baseURL), approval.ID)
	msg := buildApprovalMessage(req.TxContext, wallet)

	// Log: distinguish transfer (has txParams) vs generic sign.
	auditAction := "sign"
	if txParams != "" {
		auditAction = "transfer"
	}
	writeAuditCtx(h.db, c, auditAction, "pending", &wallet.ID, map[string]interface{}{
		"approval_id": approval.ID, "tx_context": req.TxContext,
	}, approval.ID)

	c.JSON(http.StatusAccepted, gin.H{
		"status":        "pending_approval",
		"approval_id":   approval.ID,
		"message":       msg,
		"tx_context":    req.TxContext,
		"threshold_usd": policy.ThresholdUSD,
		"approval_url":  approvalURL,
	})
}

// TokenParams specifies the ERC-20 token to use in a transfer (optional).
type TokenParams struct {
	Contract string `json:"contract" binding:"required"` // ERC-20 contract address (0x...)
	Decimals int    `json:"decimals"`                    // token decimals, default 18
	Symbol   string `json:"symbol"`                      // e.g. "USDC"
}

// TransferRequest is the body for POST /api/wallets/:id/transfer.
type TransferRequest struct {
	To     string       `json:"to" binding:"required"`
	Amount string       `json:"amount" binding:"required"`
	Memo   string       `json:"memo"`
	Token  *TokenParams `json:"token"` // optional: ERC-20 token info
}

// Transfer constructs a blockchain transaction in the backend, signs it via TEE, and broadcasts it.
// This is the preferred way to send crypto — the transaction is never built on the client side,
// so the approval UI always shows exactly what will be broadcast.
// POST /api/wallets/:id/transfer
func (h *WalletHandler) Transfer(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	// Check idempotency key — return cached response if this request was already processed.
	idemKey := IdempotencyKey(c)
	if CheckIdempotency(c, h.idempotency, idemKey, userID) {
		return
	}

	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	if wallet.Status != "ready" {
		jsonError(c, http.StatusBadRequest, "wallet is not ready (status: "+wallet.Status+")")
		return
	}

	var req TransferRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	amount, ok2 := new(big.Float).SetString(req.Amount)
	if !ok2 || amount.Sign() <= 0 {
		jsonError(c, http.StatusBadRequest, "invalid amount")
		return
	}

	// Build the unsigned transaction on the backend (chain-specific).
	var signingMsg []byte // signing hash (ETH) or raw message bytes (SOL)
	var txParamsJSON string
	var currency string
	txType := "transfer"
	var tokenContractAddr string // set for ERC-20 transfers

	chainCfg, ok := model.GetChain(wallet.Chain)
	if !ok {
		jsonError(c, http.StatusBadRequest, "unsupported chain: "+wallet.Chain)
		return
	}
	rpcURL := chainCfg.RPCURL

	// Resolve address book nickname if the input doesn't look like a raw address.
	if !LooksLikeAddress(req.To, chainCfg.Family) {
		resolved, resolveErr := ResolveNickname(h.db, userID, req.To, wallet.Chain)
		if resolveErr != nil {
			jsonError(c, http.StatusBadRequest, fmt.Sprintf("nickname %q not found in address book for chain %s", req.To, wallet.Chain))
			return
		}
		req.To = resolved
	}

	if strings.EqualFold(req.To, wallet.Address) {
		jsonError(c, http.StatusBadRequest, "cannot transfer to the same wallet address")
		return
	}

	switch chainCfg.Family {
	case "evm":
		// Validate and normalize recipient address.
		toAddr, addrErr := normalizeEVMAddress(req.To)
		if addrErr != nil {
			jsonError(c, http.StatusBadRequest, "to: "+addrErr.Error())
			return
		}
		req.To = toAddr

		if req.Token != nil {
			// ERC-20 transfer: validate contract is whitelisted, then build contract call tx.
			tokenContractAddr = strings.ToLower(strings.TrimSpace(req.Token.Contract))
			var allowed model.AllowedContract
			if err := h.db.Where("user_id = ? AND chain = ? AND contract_address = ?", wallet.UserID, wallet.Chain, tokenContractAddr).First(&allowed).Error; err != nil {
				jsonError(c, http.StatusForbidden, "contract not whitelisted: "+tokenContractAddr)
				return
			}

			decimals := allowed.Decimals
			if decimals == 0 {
				decimals = 18
			}
			exp := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
			tokenUnitsF := new(big.Float).SetPrec(256).Mul(amount, new(big.Float).SetInt(exp))
			tokenUnits, _ := tokenUnitsF.Int(nil)
			if tokenUnits.Sign() <= 0 {
				jsonError(c, http.StatusBadRequest, "amount must be positive")
				return
			}
			if len(tokenUnits.Bytes()) > 32 {
				jsonError(c, http.StatusBadRequest, "token amount exceeds uint256 range")
				return
			}

			callData, encodeErr := chain.EncodeERC20Transfer(req.To, tokenUnits)
			if encodeErr != nil {
				slog.Error("encode ERC-20 transfer failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "error", encodeErr)
				jsonErrorDetails(c, http.StatusBadRequest, "failed to encode transfer", gin.H{
					"stage": "encode_calldata", "wallet_id": wallet.ID, "chain": wallet.Chain,
				})
				return
			}
			txData, err := chain.BuildETHContractCallTx(rpcURL, wallet.Address, tokenContractAddr, callData, nil)
			if err != nil {
				slog.Error("build ERC-20 tx failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "rpc_url", rpcURL, "error", err)
				jsonErrorDetails(c, http.StatusUnprocessableEntity, "failed to build transaction", gin.H{
					"stage": "build_tx", "wallet_id": wallet.ID, "chain": wallet.Chain,
					"to": req.To, "contract": tokenContractAddr,
				})
				return
			}
			signingMsg = txData.SigningHash
			b, marshalErr := json.Marshal(txData.Params)
			if marshalErr != nil {
				jsonErrorDetails(c, http.StatusInternalServerError, "marshal tx params failed", gin.H{"stage": "marshal_tx_params"})
				return
			}
			txParamsJSON = string(b)
			currency = strings.ToUpper(allowed.Symbol)
			if currency == "" {
				currency = strings.ToUpper(req.Token.Symbol)
			}
			txType = "erc20_transfer"
		} else {
			txData, err := chain.BuildETHTx(rpcURL, wallet.Address, req.To, amount)
			if err != nil {
				slog.Error("build ETH tx failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "rpc_url", rpcURL, "error", err)
				jsonErrorDetails(c, http.StatusUnprocessableEntity, "failed to build transaction", gin.H{
					"stage": "build_tx", "wallet_id": wallet.ID, "chain": wallet.Chain,
					"to": req.To, "amount": req.Amount,
				})
				return
			}
			signingMsg = txData.SigningHash
			b, marshalErr := json.Marshal(txData.Params)
			if marshalErr != nil {
				jsonErrorDetails(c, http.StatusInternalServerError, "marshal tx params failed", gin.H{"stage": "marshal_tx_params"})
				return
			}
			txParamsJSON = string(b)
			currency = chainCfg.Currency
		}

	case "solana":
		if req.Token != nil {
			// SPL Token transfer
			mintAddr := strings.TrimSpace(req.Token.Contract)
			if _, err := chain.Base58Decode(mintAddr); err != nil {
				jsonError(c, http.StatusBadRequest, "token contract: invalid Solana address")
				return
			}
			if _, err := chain.Base58Decode(req.To); err != nil {
				jsonError(c, http.StatusBadRequest, "to: invalid Solana address")
				return
			}
			// Whitelist check
			var allowed model.AllowedContract
			if err := h.db.Where("user_id = ? AND chain = ? AND contract_address = ?", wallet.UserID, wallet.Chain, mintAddr).First(&allowed).Error; err != nil {
				jsonError(c, http.StatusForbidden, "token not whitelisted: "+mintAddr)
				return
			}
			decimals := allowed.Decimals
			if decimals == 0 && req.Token.Decimals > 0 {
				decimals = req.Token.Decimals
			}
			if decimals == 0 {
				decimals = 9
			}
			exp := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
			tokenUnitsF := new(big.Float).SetPrec(256).Mul(amount, new(big.Float).SetInt(exp))
			tokenUnits, _ := tokenUnitsF.Int(nil)
			if tokenUnits.Sign() <= 0 || !tokenUnits.IsUint64() {
				jsonError(c, http.StatusBadRequest, "amount out of range")
				return
			}
			txData, err := chain.BuildSOLTokenTransferTx(rpcURL, wallet.Address, req.To, mintAddr, tokenUnits.Uint64(), decimals)
			if err != nil {
				slog.Error("build SPL token tx failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "mint", mintAddr, "error", err)
				jsonErrorDetails(c, http.StatusUnprocessableEntity, "failed to build transaction", gin.H{
					"stage": "build_tx", "wallet_id": wallet.ID, "chain": wallet.Chain,
					"to": req.To, "mint": mintAddr, "amount": req.Amount,
				})
				return
			}
			signingMsg = txData.MessageBytes
			b, _ := json.Marshal(txData.Params)
			txParamsJSON = string(b)
			currency = strings.ToUpper(allowed.Symbol)
			if currency == "" {
				currency = strings.ToUpper(req.Token.Symbol)
			}
			txType = "spl_transfer"
			tokenContractAddr = mintAddr
		} else {
			if _, err := chain.Base58Decode(req.To); err != nil {
				jsonError(c, http.StatusBadRequest, "invalid Solana address")
				return
			}
			lamportsBF := new(big.Float).SetPrec(128).Mul(amount, new(big.Float).SetFloat64(1e9))
			lamports, _ := lamportsBF.Uint64()
			if lamports == 0 {
				jsonError(c, http.StatusBadRequest, "amount too small")
				return
			}
			txData, err := chain.BuildSOLTxFromLamports(rpcURL, wallet.Address, req.To, lamports)
			if err != nil {
				slog.Error("build SOL tx failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "rpc_url", rpcURL, "error", err)
				jsonErrorDetails(c, http.StatusUnprocessableEntity, "failed to build transaction", gin.H{
					"stage": "build_tx", "wallet_id": wallet.ID, "chain": wallet.Chain,
					"to": req.To, "amount": req.Amount,
				})
				return
			}
			signingMsg = txData.MessageBytes
			b, marshalErr := json.Marshal(txData.Params)
			if marshalErr != nil {
				jsonErrorDetails(c, http.StatusInternalServerError, "marshal tx params failed", gin.H{"stage": "marshal_tx_params"})
				return
			}
			txParamsJSON = string(b)
			currency = chainCfg.Currency
		}

	default:
		jsonError(c, http.StatusBadRequest, "unsupported chain: "+wallet.Chain)
		return
	}

	txContext := map[string]interface{}{
		"type":     txType,
		"from":     wallet.Address,
		"to":       req.To,
		"amount":   req.Amount,
		"currency": currency,
		"memo":     req.Memo,
	}
	if tokenContractAddr != "" {
		txContext["contract"] = tokenContractAddr
	}

	// Load the wallet's USD approval policy (if any).
	var policy model.ApprovalPolicy
	policyFound := h.db.Where("wallet_id = ? AND enabled = ?", wallet.ID, true).First(&policy).Error == nil

	// Convert amount to USD for threshold/limit checks.
	var amountUSD float64
	if policyFound && h.prices != nil {
		if usdPrice, priceErr := h.prices.GetUSDPrice(currency); priceErr == nil && usdPrice > 0 {
			if a, ok := new(big.Float).SetString(req.Amount); ok {
				f, _ := a.Float64()
				amountUSD = f * usdPrice
			}
		} else if tokenContractAddr != "" {
			// Fallback 1: CoinGecko token price by contract address.
			if usdPrice, priceErr := h.prices.GetTokenUSDPrice(chainCfg.Name, tokenContractAddr); priceErr == nil && usdPrice > 0 {
				if a, ok := new(big.Float).SetString(req.Amount); ok {
					f, _ := a.Float64()
					amountUSD = f * usdPrice
				}
			} else if chainCfg.Family == "solana" {
				// Fallback 2: Jupiter Price API for Solana SPL tokens.
				if usdPrice, priceErr := h.prices.GetJupiterPrice(tokenContractAddr); priceErr == nil && usdPrice > 0 {
					if a, ok := new(big.Float).SetString(req.Amount); ok {
						f, _ := a.Float64()
						amountUSD = f * usdPrice
					}
				}
			}
		}
	}

	// Final fallback: look up price by token symbol (handles testnet tokens).
	if policyFound && amountUSD == 0 && tokenContractAddr != "" && h.prices != nil && currency != "" {
		if usdPrice, priceErr := h.prices.GetPriceBySymbol(currency); priceErr == nil && usdPrice > 0 {
			if a, ok := new(big.Float).SetString(req.Amount); ok {
				f, _ := a.Float64()
				amountUSD = f * usdPrice
			}
		}
	}

	// Unknown token price with active policy → require approval (fail-closed).
	if policyFound && amountUSD == 0 && tokenContractAddr != "" && !isPasskeyAuth(c) {
		signReq := SignRequest{
			Message:   hex.EncodeToString(signingMsg),
			Encoding:  "hex",
			TxContext: txContext,
		}
		h.createApprovalRequest(c, wallet, signReq, signingMsg, &policy, txParamsJSON)
		return
	}

	// Check daily spend limit in USD atomically (pre-deduct, rollback on failure).
	var deductedUSDStr string // non-empty if we pre-deducted
	if policyFound && policy.DailyLimitUSD != "" && amountUSD > 0 {
		deductedUSDStr = new(big.Float).SetFloat64(amountUSD).Text('f', 6)
		exceeded, msg, err := checkAndDeductDailyLimitUSD(h.db, wallet.ID, deductedUSDStr)
		if err != nil {
			slog.Error("daily limit check error", "wallet_id", wallet.ID, "error", err)
			jsonErrorDetails(c, http.StatusInternalServerError, "failed to check daily limit", gin.H{"stage": "daily_limit_check"})
			return
		}
		if exceeded {
			deductedUSDStr = "" // nothing was deducted
			jsonError(c, http.StatusBadRequest, msg)
			return
		}
	}

	// Check single-transaction USD approval threshold.
	if policyFound && amountUSD > 0 && exceedsUSDThreshold(amountUSD, policy.ThresholdUSD) {
		// Approval path: rollback pre-deduction — addDailySpentUSD will add it back on approve+broadcast.
		if deductedUSDStr != "" {
			releaseDailySpentUSD(h.db, wallet.ID, deductedUSDStr)
		}
		signReq := SignRequest{
			Message:   hex.EncodeToString(signingMsg),
			Encoding:  "hex",
			TxContext: txContext,
		}
		h.createApprovalRequest(c, wallet, signReq, signingMsg, &policy, txParamsJSON)
		return
	}

	// Direct path: sign via TEE and broadcast.
	sbResult, sbErr := h.signAndBroadcast(c, wallet, signingMsg, txParamsJSON)
	if sbErr != nil {
		if deductedUSDStr != "" {
			releaseDailySpentUSD(h.db, wallet.ID, deductedUSDStr)
		}
		var bfe *broadcastFailedError
		if errors.As(sbErr, &bfe) {
			slog.Error("transfer broadcast failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "to", req.To, "error", sbErr)
			respondBroadcastErrorDetails(c, sbErr, gin.H{
				"wallet_id": wallet.ID, "chain": wallet.Chain,
				"to": req.To, "amount": req.Amount,
			})
		} else {
			slog.Error("TEE signing failed", "wallet_id", wallet.ID, "key", wallet.KeyName, "error", sbErr)
			jsonErrorDetails(c, http.StatusUnprocessableEntity, "signing failed", gin.H{
				"stage": "signing", "wallet_id": wallet.ID, "chain": wallet.Chain,
				"to": req.To, "amount": req.Amount,
			})
		}
		return
	}

	writeAuditCtx(h.db, c, "transfer", "success", &wallet.ID, map[string]interface{}{
		"to": req.To, "amount": req.Amount, "currency": currency,
		"chain": wallet.Chain, "tx_hash": sbResult.txHash,
	})
	resp := gin.H{
		"status":   "completed",
		"tx_hash":  sbResult.txHash,
		"chain":    wallet.Chain,
		"from":     wallet.Address,
		"to":       req.To,
		"amount":   req.Amount,
		"currency": currency,
	}
	respondWithIdempotency(c, h.idempotency, idemKey, userID, http.StatusOK, resp)
}

// WrapSOL wraps native SOL into wSOL (SPL Wrapped SOL).
// POST /api/wallets/:id/wrap-sol  { "amount": "0.1" }
func (h *WalletHandler) WrapSOL(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	idemKey := IdempotencyKey(c)
	if CheckIdempotency(c, h.idempotency, idemKey, userID) {
		return
	}

	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	if wallet.Status != "ready" {
		jsonError(c, http.StatusBadRequest, "wallet is not ready")
		return
	}
	chainCfg, ok := model.GetChain(wallet.Chain)
	if !ok {
		jsonError(c, http.StatusBadRequest, "unsupported chain: "+wallet.Chain)
		return
	}
	if chainCfg.Family != "solana" {
		jsonError(c, http.StatusBadRequest, "wrap-sol is only supported on Solana chains")
		return
	}

	var req struct {
		Amount string `json:"amount" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	amount, ok2 := new(big.Float).SetString(req.Amount)
	if !ok2 || amount.Sign() <= 0 {
		jsonError(c, http.StatusBadRequest, "invalid amount")
		return
	}
	lamportsBF := new(big.Float).SetPrec(128).Mul(amount, new(big.Float).SetFloat64(1e9))
	lamports, _ := lamportsBF.Uint64()
	if lamports == 0 {
		jsonError(c, http.StatusBadRequest, "amount too small")
		return
	}
	txData, err := chain.BuildSOLWrapTxFromLamports(chainCfg.RPCURL, wallet.Address, lamports)
	if err != nil {
		slog.Error("build wrap-sol tx failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "error", err)
		jsonErrorDetails(c, http.StatusUnprocessableEntity, "failed to build transaction", gin.H{
			"stage": "build_tx", "wallet_id": wallet.ID, "chain": wallet.Chain, "amount": req.Amount,
		})
		return
	}

	txParamsJSON, _ := json.Marshal(txData.Params)
	currency := chainCfg.Currency
	txContext := map[string]interface{}{
		"type":     "wrap_sol",
		"from":     wallet.Address,
		"amount":   req.Amount,
		"currency": currency,
	}

	// Check approval policy (same logic as Transfer).
	var policy model.ApprovalPolicy
	policyFound := h.db.Where("wallet_id = ? AND enabled = ?", wallet.ID, true).First(&policy).Error == nil

	var amountUSD float64
	if policyFound && h.prices != nil {
		if usdPrice, priceErr := h.prices.GetUSDPrice(currency); priceErr == nil && usdPrice > 0 {
			f, _ := amount.Float64()
			amountUSD = f * usdPrice
		}
	}

	// Check daily spend limit.
	var deductedUSDStr string
	if policyFound && policy.DailyLimitUSD != "" && amountUSD > 0 {
		deductedUSDStr = new(big.Float).SetFloat64(amountUSD).Text('f', 6)
		exceeded, msg, limitErr := checkAndDeductDailyLimitUSD(h.db, wallet.ID, deductedUSDStr)
		if limitErr != nil {
			slog.Error("daily limit check error", "wallet_id", wallet.ID, "error", limitErr)
			jsonErrorDetails(c, http.StatusInternalServerError, "failed to check daily limit", gin.H{"stage": "daily_limit_check"})
			return
		}
		if exceeded {
			deductedUSDStr = ""
			jsonError(c, http.StatusBadRequest, msg)
			return
		}
	}

	// Check single-transaction USD threshold.
	if policyFound && amountUSD > 0 && exceedsUSDThreshold(amountUSD, policy.ThresholdUSD) {
		if deductedUSDStr != "" {
			releaseDailySpentUSD(h.db, wallet.ID, deductedUSDStr)
		}
		signReq := SignRequest{
			Message:   hex.EncodeToString(txData.MessageBytes),
			Encoding:  "hex",
			TxContext: txContext,
		}
		h.createApprovalRequest(c, wallet, signReq, txData.MessageBytes, &policy, string(txParamsJSON))
		return
	}

	sbResult, sbErr := h.signAndBroadcast(c, wallet, txData.MessageBytes, string(txParamsJSON))
	if sbErr != nil {
		if deductedUSDStr != "" {
			releaseDailySpentUSD(h.db, wallet.ID, deductedUSDStr)
		}
		var bfe *broadcastFailedError
		if errors.As(sbErr, &bfe) {
			slog.Error("wrap-sol broadcast failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "error", sbErr)
			respondBroadcastErrorDetails(c, sbErr, gin.H{
				"wallet_id": wallet.ID, "chain": wallet.Chain, "amount": req.Amount,
			})
		} else {
			slog.Error("wrap-sol signing failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "error", sbErr)
			jsonErrorDetails(c, http.StatusUnprocessableEntity, "signing failed", gin.H{
				"stage": "signing", "wallet_id": wallet.ID, "chain": wallet.Chain, "amount": req.Amount,
			})
		}
		return
	}

	writeAuditCtx(h.db, c, "wrap_sol", "success", &wallet.ID, map[string]interface{}{
		"amount": req.Amount, "chain": wallet.Chain, "tx_hash": sbResult.txHash,
	})
	resp := gin.H{
		"status":  "completed",
		"tx_hash": sbResult.txHash,
		"chain":   wallet.Chain,
		"from":    wallet.Address,
		"amount":  req.Amount,
		"action":  "wrap",
	}
	respondWithIdempotency(c, h.idempotency, idemKey, userID, http.StatusOK, resp)
}

// UnwrapSOL unwraps all wSOL back to native SOL by closing the wSOL ATA.
// POST /api/wallets/:id/unwrap-sol
func (h *WalletHandler) UnwrapSOL(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	idemKey := IdempotencyKey(c)
	if CheckIdempotency(c, h.idempotency, idemKey, userID) {
		return
	}

	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	if wallet.Status != "ready" {
		jsonError(c, http.StatusBadRequest, "wallet is not ready")
		return
	}
	chainCfg, ok := model.GetChain(wallet.Chain)
	if !ok {
		jsonError(c, http.StatusBadRequest, "unsupported chain: "+wallet.Chain)
		return
	}
	if chainCfg.Family != "solana" {
		jsonError(c, http.StatusBadRequest, "unwrap-sol is only supported on Solana chains")
		return
	}

	txData, err := chain.BuildSOLUnwrapTx(chainCfg.RPCURL, wallet.Address)
	if err != nil {
		slog.Error("build unwrap-sol tx failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "error", err)
		jsonErrorDetails(c, http.StatusUnprocessableEntity, "failed to build transaction", gin.H{
			"stage": "build_tx", "wallet_id": wallet.ID, "chain": wallet.Chain,
		})
		return
	}

	txParamsJSON, _ := json.Marshal(txData.Params)
	txContext := map[string]interface{}{
		"type":     "unwrap_sol",
		"from":     wallet.Address,
		"currency": chainCfg.Currency,
	}

	// Check approval policy: unwrap has no known amount, so if a policy is active and
	// this is an API key request, require approval (fail-closed, same as unknown token price).
	var policy model.ApprovalPolicy
	policyFound := h.db.Where("wallet_id = ? AND enabled = ?", wallet.ID, true).First(&policy).Error == nil
	if policyFound && !isPasskeyAuth(c) {
		signReq := SignRequest{
			Message:   hex.EncodeToString(txData.MessageBytes),
			Encoding:  "hex",
			TxContext: txContext,
		}
		h.createApprovalRequest(c, wallet, signReq, txData.MessageBytes, &policy, string(txParamsJSON))
		return
	}

	sbResult, sbErr := h.signAndBroadcast(c, wallet, txData.MessageBytes, string(txParamsJSON))
	if sbErr != nil {
		var bfe *broadcastFailedError
		if errors.As(sbErr, &bfe) {
			slog.Error("unwrap-sol broadcast failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "error", sbErr)
			respondBroadcastErrorDetails(c, sbErr, gin.H{
				"wallet_id": wallet.ID, "chain": wallet.Chain,
			})
		} else {
			slog.Error("unwrap-sol signing failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "error", sbErr)
			jsonErrorDetails(c, http.StatusUnprocessableEntity, "signing failed", gin.H{
				"stage": "signing", "wallet_id": wallet.ID, "chain": wallet.Chain,
			})
		}
		return
	}

	writeAuditCtx(h.db, c, "unwrap_sol", "success", &wallet.ID, map[string]interface{}{
		"chain": wallet.Chain, "tx_hash": sbResult.txHash,
	})
	resp := gin.H{
		"status":  "completed",
		"tx_hash": sbResult.txHash,
		"chain":   wallet.Chain,
		"from":    wallet.Address,
		"action":  "unwrap",
	}
	respondWithIdempotency(c, h.idempotency, idemKey, userID, http.StatusOK, resp)
}

// signBroadcastResult holds the result of a sign-and-broadcast operation.
type signBroadcastResult struct {
	signature string
	txHash    string
}

// broadcastFailedError wraps a broadcast error so callers can distinguish it
// from a signing error using errors.As.
type broadcastFailedError struct{ cause error }

func (e *broadcastFailedError) Error() string { return e.cause.Error() }
func (e *broadcastFailedError) Unwrap() error { return e.cause }

// signAndBroadcast signs msgBytes via the TEE SDK using wallet.KeyName, then broadcasts
// the assembled transaction using txParamsJSON. If txParamsJSON is empty, only signing
// is performed and txHash is left empty. On signing failure the returned error is a plain
// error; on broadcast failure it is wrapped in *broadcastFailedError so callers can
// distinguish the two cases with errors.As. Rollback of daily-spent tracking is left
// to the caller.
func (h *WalletHandler) signAndBroadcast(c *gin.Context, wallet model.Wallet, msgBytes []byte, txParamsJSON string) (*signBroadcastResult, error) {
	result, signErr := h.sdk.Sign(c.Request.Context(), msgBytes, wallet.KeyName)
	if signErr != nil || !result.Success {
		errMsg := "signing failed"
		if signErr != nil {
			errMsg = signErr.Error()
		} else if result != nil {
			errMsg = result.Error
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	sig := "0x" + hex.EncodeToString(result.Signature)

	if txParamsJSON == "" {
		return &signBroadcastResult{signature: sig}, nil
	}

	txHash, broadcastErr := broadcastSigned(wallet, txParamsJSON, result.Signature)
	if broadcastErr != nil {
		return nil, &broadcastFailedError{cause: broadcastErr}
	}

	return &signBroadcastResult{signature: sig, txHash: txHash}, nil
}

// broadcastSigned assembles a signed transaction and broadcasts it to the chain.
// Exported as a package-level function so the approval handler can reuse it.
func broadcastSigned(wallet model.Wallet, txParamsJSON string, sig []byte) (string, error) {
	cfg, ok := model.GetChain(wallet.Chain)
	if !ok {
		return "", fmt.Errorf("unsupported chain: %s", wallet.Chain)
	}
	switch cfg.Family {
	case "evm":
		var params chain.ETHTxParams
		if err := json.Unmarshal([]byte(txParamsJSON), &params); err != nil {
			return "", fmt.Errorf("parse eth tx params: %w", err)
		}
		txHash, err := chain.AssembleAndBroadcastETH(cfg.RPCURL, params, sig, wallet.Address)
		if err != nil {
			chain.ResetNonceForChain(cfg.RPCURL, wallet.Address)
			return "", err
		}
		return txHash, nil
	case "solana":
		// Try SPL Token transfer params (has "mint" field)
		var tokenParams chain.SOLTokenTransferParams
		if json.Unmarshal([]byte(txParamsJSON), &tokenParams) == nil && tokenParams.Mint != "" {
			return chain.AssembleAndBroadcastSOLToken(cfg.RPCURL, tokenParams, sig)
		}
		// Try program call params (has "program_id" field)
		var progParams chain.SOLProgramCallParams
		if json.Unmarshal([]byte(txParamsJSON), &progParams) == nil && progParams.ProgramID != "" {
			return chain.AssembleAndBroadcastSOLProgram(cfg.RPCURL, progParams, sig)
		}
		// Try wrap/unwrap SOL params (distinguished from native transfer by absence of "to" field)
		var wrapParams chain.SOLWrapParams
		var solNative chain.SOLTxParams
		if json.Unmarshal([]byte(txParamsJSON), &wrapParams) == nil &&
			wrapParams.From != "" && wrapParams.Blockhash != "" &&
			(json.Unmarshal([]byte(txParamsJSON), &solNative) != nil || solNative.To == "") {
			return chain.AssembleAndBroadcastSOLWrap(cfg.RPCURL, wrapParams, sig)
		}
		// Fall back to native SOL transfer
		var params chain.SOLTxParams
		if err := json.Unmarshal([]byte(txParamsJSON), &params); err != nil {
			return "", fmt.Errorf("parse sol tx params: %w", err)
		}
		return chain.AssembleAndBroadcastSOL(cfg.RPCURL, params, sig)
	default:
		return "", fmt.Errorf("unsupported chain family: %s", cfg.Family)
	}
}

// loadUserWallet loads a wallet by :id param and verifies it belongs to the current user.
// Shared by WalletHandler, ContractHandler, and BalanceHandler (same package).
func loadUserWallet(c *gin.Context, db *gorm.DB) (model.Wallet, bool) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return model.Wallet{}, false
	}
	id := c.Param("id")
	if id == "" {
		jsonError(c, http.StatusBadRequest, "invalid wallet id")
		return model.Wallet{}, false
	}
	var wallet model.Wallet
	if err := db.Where("id = ? AND user_id = ?", id, userID).First(&wallet).Error; err != nil {
		jsonError(c, http.StatusNotFound, "wallet not found")
		return model.Wallet{}, false
	}
	return wallet, true
}

func decodeMessage(msg, encoding string) ([]byte, error) {
	switch strings.ToLower(encoding) {
	case "base64":
		b, err := base64.StdEncoding.DecodeString(msg)
		if err != nil {
			// Try URL-safe encoding as fallback.
			b, err = base64.URLEncoding.DecodeString(msg)
			if err != nil {
				return nil, fmt.Errorf("invalid base64 message: %w", err)
			}
		}
		return b, nil
	default: // hex
		b, err := hex.DecodeString(strings.TrimPrefix(msg, "0x"))
		if err != nil {
			return nil, fmt.Errorf("invalid hex message: %w", err)
		}
		return b, nil
	}
}

func extractAmountCurrency(txCtx map[string]interface{}) (string, string) {
	amount, _ := txCtx["amount"].(string)
	currency, _ := txCtx["currency"].(string)
	return amount, strings.ToUpper(currency)
}

// exceedsUSDThreshold checks if amountUSD (float64) exceeds a USD threshold string.
func exceedsUSDThreshold(amountUSD float64, thresholdUSD string) bool {
	t, ok := new(big.Float).SetString(thresholdUSD)
	if !ok || t.Sign() <= 0 {
		return false
	}
	a := new(big.Float).SetFloat64(amountUSD)
	return a.Cmp(t) > 0
}

// respondBroadcastErrorDetails is like respondBroadcastError but attaches extra context fields.
func respondBroadcastErrorDetails(c *gin.Context, err error, details gin.H) {
	msg := err.Error()
	if details == nil {
		details = gin.H{}
	}
	details["stage"] = "broadcast"
	// "rpc error:" means the node responded but rejected the tx — that's a client error.
	if strings.Contains(msg, "rpc error:") {
		jsonErrorDetails(c, http.StatusBadRequest, "transaction rejected: "+msg, details)
		return
	}
	jsonErrorDetails(c, http.StatusUnprocessableEntity, "broadcast failed: "+msg, details)
}

// checkAndDeductDailyLimitUSD atomically checks whether adding amountUSD would exceed
// the wallet's daily USD spend limit. Uses a per-wallet mutex to prevent TOCTOU races.
// Returns (exceeded, message, error).
func checkAndDeductDailyLimitUSD(db *gorm.DB, walletID string, amountUSD string) (bool, string, error) {
	mu := getWalletMutex(walletID)
	mu.Lock()
	defer mu.Unlock()

	var policy model.ApprovalPolicy
	if err := db.Where("wallet_id = ? AND enabled = ?", walletID, true).First(&policy).Error; err != nil {
		return false, "", nil
	}
	if policy.DailyLimitUSD == "" {
		return false, "", nil
	}

	dailyLimit, ok := new(big.Float).SetString(policy.DailyLimitUSD)
	if !ok || dailyLimit.Sign() <= 0 {
		return false, "", nil
	}
	amountF, ok2 := new(big.Float).SetString(amountUSD)
	if !ok2 || amountF.Sign() <= 0 {
		return false, "", nil
	}

	startOfDay := utcStartOfDay()
	currentSpent := policy.DailySpentUSD
	resetAt := policy.DailyResetAt
	if resetAt.Before(startOfDay) {
		currentSpent = "0"
		resetAt = startOfDay
	}
	spent, _ := new(big.Float).SetString(currentSpent)
	if spent == nil {
		spent = new(big.Float)
	}

	newSpent := new(big.Float).Add(spent, amountF)
	if newSpent.Cmp(dailyLimit) > 0 {
		return true, fmt.Sprintf(
			"daily spend limit exceeded: limit $%s USD, already spent $%s USD today",
			policy.DailyLimitUSD, currentSpent,
		), nil
	}

	if err := db.Model(&policy).Updates(map[string]interface{}{
		"daily_spent_usd": newSpent.Text('f', 6),
		"daily_reset_at":  resetAt,
	}).Error; err != nil {
		return false, "", fmt.Errorf("failed to update daily spent: %w", err)
	}
	return false, "", nil
}

// releaseDailySpentUSD rolls back a prior pre-deduction when signing or broadcast fails.
// Uses the per-wallet mutex to stay consistent with checkAndDeductDailyLimitUSD.
func releaseDailySpentUSD(db *gorm.DB, walletID string, amountUSD string) {
	amountF, ok := new(big.Float).SetString(amountUSD)
	if !ok || amountF.Sign() <= 0 {
		return
	}

	mu := getWalletMutex(walletID)
	mu.Lock()
	defer mu.Unlock()

	var policy model.ApprovalPolicy
	if db.Where("wallet_id = ? AND enabled = ?", walletID, true).First(&policy).Error != nil {
		return
	}

	startOfDay := utcStartOfDay()
	currentSpent := policy.DailySpentUSD
	if policy.DailyResetAt.Before(startOfDay) {
		return // already reset for a new day, nothing to release
	}
	spent, _ := new(big.Float).SetString(currentSpent)
	if spent == nil {
		return
	}
	newSpent := new(big.Float).Sub(spent, amountF)
	if newSpent.Sign() < 0 {
		newSpent = new(big.Float)
	}
	db.Model(&policy).Update("daily_spent_usd", newSpent.Text('f', 6))
}

// addDailySpentUSD increments the daily USD spend counter after a successful broadcast.
func addDailySpentUSD(db *gorm.DB, walletID string, amountUSD string) {
	amountF, ok := new(big.Float).SetString(amountUSD)
	if !ok || amountF.Sign() <= 0 {
		return
	}

	mu := getWalletMutex(walletID)
	mu.Lock()
	defer mu.Unlock()

	var policy model.ApprovalPolicy
	if db.Where("wallet_id = ? AND enabled = ?", walletID, true).First(&policy).Error != nil {
		return
	}

	startOfDay := utcStartOfDay()
	currentSpent := policy.DailySpentUSD
	resetAt := policy.DailyResetAt
	if resetAt.Before(startOfDay) {
		currentSpent = "0"
		resetAt = startOfDay
	}
	spent, _ := new(big.Float).SetString(currentSpent)
	if spent == nil {
		spent = new(big.Float)
	}
	newSpent := new(big.Float).Add(spent, amountF)
	db.Model(&policy).Updates(map[string]interface{}{
		"daily_spent_usd": newSpent.Text('f', 6),
		"daily_reset_at":  resetAt,
	})
}

func buildApprovalMessage(txCtx map[string]interface{}, wallet model.Wallet) string {
	if txCtx == nil {
		return fmt.Sprintf("Signing request for wallet %s (%s) requires approval", wallet.Address, wallet.Chain)
	}
	amount, _ := txCtx["amount"].(string)
	currency, _ := txCtx["currency"].(string)
	to, _ := txCtx["to"].(string)
	txType, _ := txCtx["type"].(string)
	from := wallet.Address
	if f, ok := txCtx["from"].(string); ok && f != "" {
		from = f
	}
	if txType == "erc20_transfer" {
		contract, _ := txCtx["contract"].(string)
		return fmt.Sprintf("ERC-20 transfer %s %s from %s to %s (contract: %s) requires approval", amount, currency, from, to, contract)
	}
	return fmt.Sprintf("Transfer %s %s from %s to %s requires approval", amount, currency, from, to)
}
