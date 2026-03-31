package handler

import (
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sdk "github.com/TEENet-io/teenet-sdk/go"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/chain"
	"github.com/TEENet-io/teenet-wallet/model"
)

// ApprovalHandler handles approval request lifecycle.
type ApprovalHandler struct {
	db     *gorm.DB
	sdk    *sdk.Client
	prices *PriceService
}

func NewApprovalHandler(db *gorm.DB, sdkClient *sdk.Client) *ApprovalHandler {
	return &ApprovalHandler{db: db, sdk: sdkClient}
}

// SetPriceService sets the USD price service used for daily spent conversion.
func (h *ApprovalHandler) SetPriceService(ps *PriceService) { h.prices = ps }

// ListPending returns all pending approval requests for the current user.
// GET /api/approvals/pending
func (h *ApprovalHandler) ListPending(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	// Batch-expire all stale pending requests for this user in a single UPDATE.
	h.db.Model(&model.ApprovalRequest{}).
		Where("user_id = ? AND status = ? AND expires_at < ?", userID, "pending", time.Now()).
		Update("status", "expired")

	var pending []model.ApprovalRequest
	if err := h.db.Where("user_id = ? AND status = ?", userID, "pending").
		Order("created_at desc").Find(&pending).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "db error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "approvals": pending})
}

// GetApproval returns details and status of a single approval request.
// GET /api/approvals/:id
func (h *ApprovalHandler) GetApproval(c *gin.Context) {
	approval, ok := h.loadUserApproval(c)
	if !ok {
		return
	}
	// Auto-expire if needed.
	if approval.Status == "pending" && time.Now().After(approval.ExpiresAt) {
		h.db.Model(&approval).Update("status", "expired")
		approval.Status = "expired"
	}

	var txCtx interface{}
	_ = json.Unmarshal([]byte(approval.TxContext), &txCtx)

	c.JSON(http.StatusOK, gin.H{
		"success":    true,
		"approval":   approval,
		"tx_context": txCtx,
	})
}

// Approve approves an approval request (Passkey session only).
// Requires a fresh WebAuthn assertion in the request body alongside the session token,
// so a stolen session token alone cannot approve a transaction.
// POST /api/approvals/:id/approve
func (h *ApprovalHandler) Approve(c *gin.Context) {
	// Verify a fresh passkey credential before doing anything sensitive.
	if !verifyFreshPasskey(h.sdk, c, h.db) {
		return
	}

	approval, ok := h.loadUserApproval(c)
	if !ok {
		return
	}
	if approval.Status != "pending" {
		jsonError(c, http.StatusBadRequest, "approval is not pending (status: "+approval.Status+")")
		return
	}
	if time.Now().After(approval.ExpiresAt) {
		h.db.Model(&approval).Update("status", "expired")
		jsonError(c, http.StatusBadRequest, "approval has expired")
		return
	}

	// Contract whitelist approvals: add the contract and finish.
	if approval.ApprovalType == "contract_add" {
		var proposed model.AllowedContract
		if err := json.Unmarshal([]byte(approval.PolicyData), &proposed); err != nil {
			jsonError(c, http.StatusInternalServerError, "invalid contract data")
			return
		}
		proposed.ID = 0 // let DB assign a new ID
		if err := h.db.Create(&proposed).Error; err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				// Already whitelisted — treat as success.
			} else {
				jsonError(c, http.StatusInternalServerError, "failed to add contract")
				return
			}
		}
		approverPasskeyID := passkeyUserIDFromCtx(c)
		h.db.Model(&approval).Updates(map[string]interface{}{"status": "approved", "approved_by": approverPasskeyID})
		updateAuditByApprovalID(h.db, approval.ID, "success", map[string]interface{}{
			"type": "contract_add", "contract": proposed.ContractAddress,
		})
		c.JSON(http.StatusOK, gin.H{"success": true, "status": "approved", "contract": proposed})
		return
	}

	// Contract whitelist update approvals: apply changes to existing contract.
	if approval.ApprovalType == "contract_update" {
		var proposed model.AllowedContract
		if err := json.Unmarshal([]byte(approval.PolicyData), &proposed); err != nil {
			jsonError(c, http.StatusInternalServerError, "invalid contract data")
			return
		}
		// Update the existing record by ID.
		if err := h.db.Model(&model.AllowedContract{}).Where("id = ? AND user_id = ?", proposed.ID, approval.UserID).
			Updates(map[string]interface{}{
				"label":    proposed.Label,
				"symbol":   proposed.Symbol,
				"decimals": proposed.Decimals,
			}).Error; err != nil {
			jsonError(c, http.StatusInternalServerError, "failed to update contract")
			return
		}
		approverPasskeyID := passkeyUserIDFromCtx(c)
		h.db.Model(&approval).Updates(map[string]interface{}{"status": "approved", "approved_by": approverPasskeyID})
		updateAuditByApprovalID(h.db, approval.ID, "success", map[string]interface{}{
			"type": "contract_update", "contract": proposed.ContractAddress,
		})
		c.JSON(http.StatusOK, gin.H{"success": true, "status": "approved", "contract": proposed})
		return
	}

	// Address book add approvals.
	if approval.ApprovalType == "addressbook_add" {
		var proposed model.AddressBookEntry
		if err := json.Unmarshal([]byte(approval.PolicyData), &proposed); err != nil {
			jsonError(c, http.StatusInternalServerError, "invalid address book data")
			return
		}
		proposed.ID = 0
		if err := h.db.Create(&proposed).Error; err != nil {
			if strings.Contains(err.Error(), "UNIQUE") {
				// Already exists — treat as success.
			} else {
				jsonError(c, http.StatusInternalServerError, "failed to add address book entry")
				return
			}
		}
		approverPasskeyID := passkeyUserIDFromCtx(c)
		h.db.Model(&approval).Updates(map[string]interface{}{"status": "approved", "approved_by": approverPasskeyID})
		updateAuditByApprovalID(h.db, approval.ID, "success", map[string]interface{}{
			"type": "addressbook_add", "nickname": proposed.Nickname, "chain": proposed.Chain,
		})
		c.JSON(http.StatusOK, gin.H{"success": true, "status": "approved", "entry": proposed})
		return
	}

	// Address book update approvals.
	if approval.ApprovalType == "addressbook_update" {
		var proposed model.AddressBookEntry
		if err := json.Unmarshal([]byte(approval.PolicyData), &proposed); err != nil {
			jsonError(c, http.StatusInternalServerError, "invalid address book data")
			return
		}
		result := h.db.Model(&model.AddressBookEntry{}).Where("id = ? AND user_id = ?", proposed.ID, approval.UserID).
			Updates(map[string]interface{}{
				"nickname": proposed.Nickname,
				"address":  proposed.Address,
				"memo":     proposed.Memo,
			})
		if result.Error != nil {
			jsonError(c, http.StatusInternalServerError, "failed to update address book entry")
			return
		}
		if result.RowsAffected == 0 {
			jsonError(c, http.StatusNotFound, "address book entry no longer exists")
			return
		}
		approverPasskeyID := passkeyUserIDFromCtx(c)
		h.db.Model(&approval).Updates(map[string]interface{}{"status": "approved", "approved_by": approverPasskeyID})
		updateAuditByApprovalID(h.db, approval.ID, "success", map[string]interface{}{
			"type": "addressbook_update", "nickname": proposed.Nickname, "chain": proposed.Chain,
		})
		c.JSON(http.StatusOK, gin.H{"success": true, "status": "approved", "entry": proposed})
		return
	}

	// Policy change approvals: apply the proposed policy and finish.
	if approval.ApprovalType == "policy_change" {
		var proposed model.ApprovalPolicy
		if err := json.Unmarshal([]byte(approval.PolicyData), &proposed); err != nil {
			jsonError(c, http.StatusInternalServerError, "invalid policy data")
			return
		}
		var policy model.ApprovalPolicy
		if approval.WalletID == nil {
			jsonError(c, http.StatusBadRequest, "approval has no wallet")
			return
		}
		if h.db.Where("wallet_id = ?", *approval.WalletID).First(&policy).Error != nil {
			policy = model.ApprovalPolicy{WalletID: *approval.WalletID}
		}
		policy.ThresholdUSD = proposed.ThresholdUSD
		policy.Enabled = proposed.Enabled
		policy.DailyLimitUSD = proposed.DailyLimitUSD
		if err := h.db.Save(&policy).Error; err != nil {
			jsonError(c, http.StatusInternalServerError, "failed to apply policy")
			return
		}
		approverPasskeyID := passkeyUserIDFromCtx(c)
		updates := map[string]interface{}{"status": "approved", "approved_by": approverPasskeyID}
		h.db.Model(&approval).Updates(updates)
		updateAuditByApprovalID(h.db, approval.ID, "success", map[string]interface{}{
			"type": "policy_change", "threshold_usd": proposed.ThresholdUSD,
		})
		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"status":  "approved",
			"policy":  policy,
		})
		return
	}

	// Load wallet to get the key name.
	if approval.WalletID == nil {
		jsonError(c, http.StatusBadRequest, "approval has no wallet")
		return
	}
	var wallet model.Wallet
	if err := h.db.First(&wallet, "id = ?", *approval.WalletID).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "wallet not found")
		return
	}

	// Decode the original message.
	msgBytes, err := decodeMessage(approval.Message, "hex")
	if err != nil {
		jsonError(c, http.StatusInternalServerError, "invalid stored message: "+err.Error())
		return
	}

	// Rebuild the transaction with fresh chain state before signing.
	// - Solana: blockhash expires in ~60s
	// - ETH: nonce may have advanced since the approval was created (e.g. another tx was sent)
	txParamsToUse := approval.TxParams
	cfg, cfgOk := model.Chains[wallet.Chain]
	if cfgOk && approval.TxParams != "" {
		switch cfg.Family {
		case "solana":
			var tokenParams chain.SOLTokenTransferParams
			if json.Unmarshal([]byte(approval.TxParams), &tokenParams) == nil && tokenParams.Mint != "" {
				if freshTx, buildErr := chain.RebuildSOLTokenTransferTx(cfg.RPCURL, tokenParams); buildErr == nil {
					msgBytes = freshTx.MessageBytes
					if freshJSON, mErr := json.Marshal(freshTx.Params); mErr == nil {
						txParamsToUse = string(freshJSON)
					}
				}
			} else {
				var progParams chain.SOLProgramCallParams
				if json.Unmarshal([]byte(approval.TxParams), &progParams) == nil && progParams.ProgramID != "" {
					if freshTx, buildErr := chain.RebuildSOLProgramCallTx(cfg.RPCURL, progParams); buildErr == nil {
						msgBytes = freshTx.MessageBytes
						if freshJSON, mErr := json.Marshal(freshTx.Params); mErr == nil {
							txParamsToUse = string(freshJSON)
						}
					}
				} else {
					var wrapParams chain.SOLWrapParams
					if json.Unmarshal([]byte(approval.TxParams), &wrapParams) == nil && wrapParams.Blockhash != "" && (wrapParams.Wrap || wrapParams.Lamports == 0) {
						if freshTx, buildErr := chain.RebuildSOLWrapTx(cfg.RPCURL, wrapParams); buildErr == nil {
							msgBytes = freshTx.MessageBytes
							if freshJSON, mErr := json.Marshal(freshTx.Params); mErr == nil {
								txParamsToUse = string(freshJSON)
							}
						}
					} else {
						// Existing native SOL transfer rebuild
						var solParams chain.SOLTxParams
						if jsonErr := json.Unmarshal([]byte(approval.TxParams), &solParams); jsonErr == nil {
							amountSOL := float64(solParams.Lamports) / 1e9
							if freshTx, buildErr := chain.BuildSOLTx(cfg.RPCURL, solParams.From, solParams.To, amountSOL); buildErr == nil {
								msgBytes = freshTx.MessageBytes
								if freshJSON, mErr := json.Marshal(freshTx.Params); mErr == nil {
									txParamsToUse = string(freshJSON)
								}
							}
						}
					}
				}
			}
		case "evm":
			var ethParams chain.ETHTxParams
			if jsonErr := json.Unmarshal([]byte(approval.TxParams), &ethParams); jsonErr != nil {
				slog.Error("ETH tx params unmarshal failed", "approval_id", approval.ID, "error", jsonErr)
				jsonError(c, http.StatusInternalServerError, "invalid stored tx params")
				return
			}
			freshTx, buildErr := chain.RebuildETHTx(cfg.RPCURL, ethParams)
			if buildErr != nil {
				slog.Error("ETH tx rebuild failed", "approval_id", approval.ID, "error", buildErr)
				jsonError(c, http.StatusBadGateway, "failed to refresh transaction params: "+buildErr.Error())
				return
			}
			msgBytes = freshTx.SigningHash
			if freshJSON, mErr := json.Marshal(freshTx.Params); mErr == nil {
				txParamsToUse = string(freshJSON)
			}
		}
	}

	// Execute TEE signing now that approval is granted.
	result, signErr := h.sdk.Sign(c.Request.Context(), msgBytes, wallet.KeyName)
	if signErr != nil || !result.Success {
		errMsg := "signing failed"
		if signErr != nil {
			errMsg = signErr.Error()
		} else if result != nil {
			errMsg = result.Error
		}
		slog.Error("TEE signing failed", "approval_id", approval.ID, "wallet_id", wallet.ID, "key", wallet.KeyName, "error", errMsg)
		jsonError(c, http.StatusBadGateway, "signing failed: "+errMsg)
		return
	}

	sig := "0x" + hex.EncodeToString(result.Signature)
	approverPasskeyID := passkeyUserIDFromCtx(c)

	// If TxParams is set, this was a /transfer approval: assemble and broadcast.
	var txHash string
	if txParamsToUse != "" {
		var broadcastErr error
		txHash, broadcastErr = broadcastSigned(wallet, txParamsToUse, result.Signature)
		if broadcastErr != nil {
			slog.Error("broadcast failed", "approval_id", approval.ID, "wallet_id", wallet.ID, "address", wallet.Address, "error", broadcastErr)
			respondBroadcastError(c, broadcastErr)
			return
		}
	}

	updates := map[string]interface{}{
		"status":      "approved",
		"signature":   sig,
		"approved_by": approverPasskeyID,
	}
	if txHash != "" {
		updates["tx_hash"] = txHash
	}
	if err := h.db.Model(&approval).Updates(updates).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "update approval failed")
		return
	}
	auditExtra := map[string]interface{}{"type": approval.ApprovalType}
	if txHash != "" {
		auditExtra["tx_hash"] = txHash
	}
	// Parse TxContext once; reuse for audit enrichment and daily-limit update below.
	var txCtx map[string]interface{}
	_ = json.Unmarshal([]byte(approval.TxContext), &txCtx)
	if txCtx != nil {
		if to, ok := txCtx["to"].(string); ok && to != "" {
			auditExtra["to"] = to
		}
		if amount, currency := extractAmountCurrency(txCtx); amount != "" {
			auditExtra["amount"] = amount
			auditExtra["currency"] = currency
		}
	}
	updateAuditByApprovalID(h.db, approval.ID, "success", auditExtra)

	// Update daily USD spent counter for /transfer approvals that were successfully broadcast.
	if txHash != "" && txCtx != nil && h.prices != nil {
		amount, currency := extractAmountCurrency(txCtx)
		if amount != "" && currency != "" {
			if usdPrice, priceErr := h.prices.GetUSDPrice(currency); priceErr == nil && usdPrice > 0 {
				if a, ok := new(big.Float).SetString(amount); ok {
					f, _ := a.Float64()
					amountUSD := new(big.Float).SetFloat64(f * usdPrice).Text('f', 2)
					if approval.WalletID != nil {
						addDailySpentUSD(h.db, *approval.WalletID, amountUSD)
					}
				}
			}
		}
	}

	resp := gin.H{
		"success":        true,
		"status":         "approved",
		"signature":      sig,
		"wallet_address": wallet.Address,
		"chain":          wallet.Chain,
	}
	if txHash != "" {
		resp["tx_hash"] = txHash
	}
	c.JSON(http.StatusOK, resp)
}

// Reject rejects an approval request (Passkey session only).
// Also requires a fresh WebAuthn assertion to prevent session-token-only attacks.
// POST /api/approvals/:id/reject
func (h *ApprovalHandler) Reject(c *gin.Context) {
	if !verifyFreshPasskey(h.sdk, c, h.db) {
		return
	}

	approval, ok := h.loadUserApproval(c)
	if !ok {
		return
	}
	if approval.Status != "pending" {
		jsonError(c, http.StatusBadRequest, "approval is not pending")
		return
	}
	if err := h.db.Model(&approval).Update("status", "rejected").Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "update failed")
		return
	}
	updateAuditByApprovalID(h.db, approval.ID, "rejected", map[string]interface{}{
		"type": approval.ApprovalType,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "status": "rejected"})
}

func (h *ApprovalHandler) loadUserApproval(c *gin.Context) (model.ApprovalRequest, bool) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return model.ApprovalRequest{}, false
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid approval id")
		return model.ApprovalRequest{}, false
	}
	var approval model.ApprovalRequest
	if err := h.db.First(&approval, id).Error; err != nil {
		jsonError(c, http.StatusNotFound, "approval not found")
		return model.ApprovalRequest{}, false
	}
	if approval.UserID != userID {
		jsonError(c, http.StatusForbidden, "access denied")
		return model.ApprovalRequest{}, false
	}
	return approval, true
}

// verifyFreshPasskey reads {login_session_id, credential} from the request body and
// calls PasskeyLoginVerifyAs to confirm both a live hardware key assertion AND that the
// verified PasskeyUserID matches the currently logged-in user.
// Returns true if verification passes (c is NOT written to). Returns false and writes a
// 401/400 response if verification fails (caller must return immediately).
// When sdkClient is nil the check is skipped (test / offline mode only).
func verifyFreshPasskey(sdkClient *sdk.Client, c *gin.Context, db *gorm.DB) bool {
	if sdkClient == nil {
		if gin.Mode() == gin.TestMode {
			return true // allow nil SDK in tests
		}
		slog.Error("SECURITY: SDK client is nil, cannot verify passkey, rejecting")
		jsonError(c, http.StatusServiceUnavailable, "passkey verification unavailable")
		return false
	}
	var body struct {
		LoginSessionID uint64      `json:"login_session_id"`
		Credential     interface{} `json:"credential"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.LoginSessionID == 0 || body.Credential == nil {
		jsonError(c, http.StatusBadRequest, "login_session_id and credential are required for this action")
		return false
	}
	return verifyFreshPasskeyParsed(sdkClient, c, body.LoginSessionID, body.Credential, db)
}

// verifyFreshPasskeyParsed verifies an already-parsed credential and confirms that the
// verified PasskeyUserID matches the currently logged-in user's PasskeyUserID.
// Uses SDK's PasskeyLoginVerifyAs for identity-bound verification when db is provided.
// Used by endpoints that carry both business fields and credential in a single JSON body.
// When sdkClient is nil the check is skipped (test / offline mode only).
func verifyFreshPasskeyParsed(sdkClient *sdk.Client, c *gin.Context, loginSessionID uint64, credential interface{}, db *gorm.DB) bool {
	if sdkClient == nil || db == nil {
		if gin.Mode() == gin.TestMode {
			return true // allow nil SDK/db in tests
		}
		slog.Error("SECURITY: SDK or db is nil, cannot verify passkey, rejecting")
		jsonError(c, http.StatusServiceUnavailable, "passkey verification unavailable")
		return false
	}
	if loginSessionID == 0 || credential == nil {
		jsonError(c, http.StatusBadRequest, "login_session_id and credential are required for this action")
		return false
	}
	credBytes, err := json.Marshal(credential)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid credential")
		return false
	}

	// Identity-bound verification: confirms the passkey assertion is valid AND
	// the PasskeyUserID matches the currently logged-in user.
	sessionUserID := mustUserID(c)
	if c.IsAborted() {
		return false
	}
	var user model.User
	if err := db.First(&user, sessionUserID).Error; err != nil {
		slog.Error("SECURITY: failed to load user for passkey verification", "user_id", sessionUserID, "error", err)
		jsonError(c, http.StatusInternalServerError, "failed to verify user identity")
		return false
	}
	res, err := sdkClient.PasskeyLoginVerifyAs(c.Request.Context(), loginSessionID, credBytes, user.PasskeyUserID)
	if err != nil || !res.Success {
		errMsg := "passkey verification failed"
		if res != nil && res.Error != "" {
			errMsg = res.Error
		}
		jsonError(c, http.StatusUnauthorized, errMsg)
		return false
	}
	return true
}

// passkeyUserIDFromCtx retrieves the PasskeyUserID from session context.
// This is stored when the passkey session was created in PasskeyVerify.
func passkeyUserIDFromCtx(c *gin.Context) *uint {
	v, exists := c.Get("userID")
	if !exists {
		return nil
	}
	userID, _ := v.(uint)
	return &userID
}
