// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"errors"
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

// ContractHandler manages the per-user-per-chain contract whitelist.
type ContractHandler struct {
	db             *gorm.DB
	sdk            *sdk.Client
	baseURL        string
	approvalExpiry time.Duration
}

func NewContractHandler(db *gorm.DB, sdkClient *sdk.Client, baseURL string, approvalExpiry ...time.Duration) *ContractHandler {
	expiry := 30 * time.Minute
	if len(approvalExpiry) > 0 && approvalExpiry[0] > 0 {
		expiry = approvalExpiry[0]
	}
	return &ContractHandler{db: db, sdk: sdkClient, baseURL: baseURL, approvalExpiry: expiry}
}

// contractAddReq is the shared request shape for AddContract / AddContractByChain.
type contractAddReq struct {
	LoginSessionID  uint64      `json:"login_session_id"`
	Credential      interface{} `json:"credential"`
	ContractAddress string      `json:"contract_address" binding:"required"`
	Label           string      `json:"label"`
	Symbol          string      `json:"symbol"`
	Decimals        int         `json:"decimals"`
}

// addContractFor inserts a contract for (userID, chainName). walletIDForAudit
// is purely metadata for the audit log / approval record — pass nil for the
// chain-scoped route which has no wallet anchor.
func (h *ContractHandler) addContractFor(c *gin.Context, userID uint, chainName string, walletIDForAudit *string) {
	var req contractAddReq
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Decimals < 0 || req.Decimals > 77 {
		jsonError(c, http.StatusBadRequest, "decimals must be between 0 and 77")
		return
	}

	// Normalize and validate address based on chain family.
	chainCfg, ok := model.GetChain(chainName)
	if !ok {
		jsonError(c, http.StatusBadRequest, "unsupported chain: "+chainName)
		return
	}
	var addr string
	if chainCfg.Family == "solana" {
		addr = strings.TrimSpace(req.ContractAddress)
		pub, err := chain.Base58Decode(addr)
		if err != nil || len(pub) != 32 {
			jsonError(c, http.StatusBadRequest, "contract_address: invalid Solana program ID")
			return
		}
	} else {
		var addrErr error
		addr, addrErr = normalizeEVMAddress(req.ContractAddress)
		if addrErr != nil {
			jsonError(c, http.StatusBadRequest, "contract_address "+addrErr.Error())
			return
		}
		if addr == "0x"+strings.Repeat("0", 40) {
			jsonError(c, http.StatusBadRequest, "zero address is not a valid contract")
			return
		}
	}

	proposed := model.AllowedContract{
		UserID:          userID,
		Chain:           chainName,
		ContractAddress: addr,
		Label:           req.Label,
		Symbol:          strings.ToUpper(strings.TrimSpace(req.Symbol)),
		Decimals:        req.Decimals,
	}

	// Reject duplicates up-front. The passkey path already learns this
	// from the UNIQUE constraint at insert time, but the API-key path
	// would otherwise create a useless pending approval that quietly
	// resolves to a no-op when the user approves it. Checking here keeps
	// both paths' user-visible behaviour consistent (immediate 409) and
	// avoids polluting the approval queue. The insert-time check is kept
	// as the race-condition safety net.
	var existing model.AllowedContract
	if err := h.db.Where("user_id = ? AND chain = ? AND contract_address = ?", userID, chainName, addr).First(&existing).Error; err == nil {
		jsonError(c, http.StatusConflict, "contract already whitelisted for this chain")
		return
	}

	// API key path: create approval request.
	if !isPasskeyAuth(c) {
		approval, created := createPendingApproval(h.db, c, walletIDForAudit, "contract_add", proposed, h.approvalExpiry)
		if !created {
			return
		}
		writeAuditCtx(h.db, c, "contract_add", "pending", walletIDForAudit, map[string]interface{}{
			"contract": addr, "chain": chainName, "symbol": proposed.Symbol, "decimals": proposed.Decimals, "label": proposed.Label, "approval_id": approval.ID,
		}, approval.ID)
		respondPendingApproval(c, h.baseURL, approval.ID, "Contract whitelist request submitted for approval")
		return
	}

	// Passkey path: require a fresh hardware assertion before applying.
	if !verifyFreshPasskeyParsed(h.sdk, c, req.LoginSessionID, req.Credential, h.db) {
		return
	}

	if err := h.db.Create(&proposed).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE") {
			jsonError(c, http.StatusConflict, "contract already whitelisted for this chain")
			return
		}
		respondInternalError(c, "db error", err, gin.H{"stage": "contract_add", "chain": chainName, "contract": addr})
		return
	}
	writeAuditCtx(h.db, c, "contract_add", "success", walletIDForAudit, map[string]interface{}{
		"contract": addr, "chain": chainName, "symbol": proposed.Symbol, "decimals": proposed.Decimals, "label": proposed.Label,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "contract": proposed})
}

// listContractsFor returns the whitelist for (userID, chainName).
func (h *ContractHandler) listContractsFor(c *gin.Context, userID uint, chainName string) {
	var contracts []model.AllowedContract
	if err := h.db.Where("user_id = ? AND chain = ?", userID, chainName).Order("created_at asc").Find(&contracts).Error; err != nil {
		respondInternalError(c, "db error", err, gin.H{"stage": "contract_list", "chain": chainName})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "contracts": contracts})
}

// updateContractFor renames a whitelisted contract. Only `label` is editable.
func (h *ContractHandler) updateContractFor(c *gin.Context, userID uint, chainName string, walletIDForAudit *string) {
	cid, err := strconv.ParseUint(c.Param("cid"), 10, 64)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid contract id")
		return
	}

	var existing model.AllowedContract
	if err := h.db.Where("id = ? AND user_id = ? AND chain = ?", cid, userID, chainName).First(&existing).Error; err != nil {
		jsonError(c, http.StatusNotFound, "contract not found")
		return
	}

	var req struct {
		Label *string `json:"label"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.Label == nil {
		jsonError(c, http.StatusBadRequest, "label is required")
		return
	}

	newLabel := *req.Label
	if newLabel == existing.Label {
		c.JSON(http.StatusOK, gin.H{"success": true, "contract": existing})
		return
	}

	if err := h.db.Model(&existing).Update("label", newLabel).Error; err != nil {
		respondInternalError(c, "update failed", err, gin.H{"stage": "contract_update", "contract_id": cid})
		return
	}
	existing.Label = newLabel
	writeAuditCtx(h.db, c, "contract_update", "success", walletIDForAudit, map[string]interface{}{
		"contract_id": cid, "contract": existing.ContractAddress, "chain": chainName, "label": newLabel,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "contract": existing})
}

// deleteContractFor removes a whitelisted contract. Caller must have verified
// passkey freshness already.
func (h *ContractHandler) deleteContractFor(c *gin.Context, userID uint, chainName string) {
	cid, err := strconv.ParseUint(c.Param("cid"), 10, 64)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid contract id")
		return
	}

	var contract model.AllowedContract
	if err := h.db.Where("id = ? AND user_id = ? AND chain = ?", cid, userID, chainName).First(&contract).Error; err != nil {
		jsonError(c, http.StatusNotFound, "contract not found")
		return
	}

	if err := h.db.Delete(&contract).Error; err != nil {
		respondInternalError(c, "delete failed", err, gin.H{"stage": "contract_delete", "contract_id": cid})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

// ── Wallet-scoped routes (legacy: still used by the agent SDK / skill docs) ──

// AddContract whitelists a contract address for a wallet.
// Passkey: applied immediately.
// API key: creates a pending approval for the passkey owner to review.
// POST /api/wallets/:id/contracts  (dual auth)
func (h *ContractHandler) AddContract(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	h.addContractFor(c, wallet.UserID, wallet.Chain, &wallet.ID)
}

// ListContracts returns all whitelisted contracts for a wallet.
// GET /api/wallets/:id/contracts  (dual auth)
func (h *ContractHandler) ListContracts(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	h.listContractsFor(c, wallet.UserID, wallet.Chain)
}

// UpdateContract modifies a whitelisted contract's label.
//
// Only `label` is editable: symbol and decimals are on-chain metadata and
// changing them client-side could desync token amount calculations, so they
// are immutable here (re-add the contract if they need to change).
//
// Label updates do NOT require passkey approval for either auth mode —
// the label is display-only text and has no effect on transfer semantics
// or which contracts are whitelisted.
// PUT /api/wallets/:id/contracts/:cid  (dual auth)
func (h *ContractHandler) UpdateContract(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	h.updateContractFor(c, wallet.UserID, wallet.Chain, &wallet.ID)
}

// DeleteContract removes a whitelisted contract.
// DELETE /api/wallets/:id/contracts/:cid  (Passkey only)
func (h *ContractHandler) DeleteContract(c *gin.Context) {
	if !verifyFreshPasskey(h.sdk, c, h.db) {
		return
	}
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	h.deleteContractFor(c, wallet.UserID, wallet.Chain)
}

// ── Chain-scoped routes (preferred: whitelist is a per-(user, chain) concept) ──
//
// These don't require the user to have a wallet on the chain — useful for
// pre-configuring whitelists, and matches the underlying data model where
// AllowedContract is keyed by (user_id, chain), not by wallet.

// AddContractByChain — POST /api/chains/:chain/contracts  (dual auth)
func (h *ContractHandler) AddContractByChain(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	h.addContractFor(c, userID, c.Param("chain"), nil)
}

// ListContractsByChain — GET /api/chains/:chain/contracts  (dual auth)
func (h *ContractHandler) ListContractsByChain(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	h.listContractsFor(c, userID, c.Param("chain"))
}

// UpdateContractByChain — PUT /api/chains/:chain/contracts/:cid  (dual auth)
func (h *ContractHandler) UpdateContractByChain(c *gin.Context) {
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	h.updateContractFor(c, userID, c.Param("chain"), nil)
}

// DeleteContractByChain — DELETE /api/chains/:chain/contracts/:cid  (Passkey only)
func (h *ContractHandler) DeleteContractByChain(c *gin.Context) {
	if !verifyFreshPasskey(h.sdk, c, h.db) {
		return
	}
	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}
	h.deleteContractFor(c, userID, c.Param("chain"))
}

