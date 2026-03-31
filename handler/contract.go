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
	approvalExpiry time.Duration
}

func NewContractHandler(db *gorm.DB, sdkClient *sdk.Client, approvalExpiry ...time.Duration) *ContractHandler {
	expiry := 30 * time.Minute
	if len(approvalExpiry) > 0 && approvalExpiry[0] > 0 {
		expiry = approvalExpiry[0]
	}
	return &ContractHandler{db: db, sdk: sdkClient, approvalExpiry: expiry}
}

// AddContract whitelists a contract address for a wallet.
// Passkey: applied immediately.
// API key: creates a pending approval for the passkey owner to review.
// POST /api/wallets/:id/contracts  (dual auth)
func (h *ContractHandler) AddContract(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}

	var req struct {
		LoginSessionID  uint64      `json:"login_session_id"`
		Credential      interface{} `json:"credential"`
		ContractAddress string      `json:"contract_address" binding:"required"`
		Label           string      `json:"label"`
		Symbol          string      `json:"symbol"`
		Decimals        int         `json:"decimals"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Decimals < 0 || req.Decimals > 77 {
		jsonError(c, http.StatusBadRequest, "decimals must be between 0 and 77")
		return
	}

	// Normalize and validate address based on chain family.
	chainCfg := model.Chains[wallet.Chain]
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
		UserID:          wallet.UserID,
		Chain:           wallet.Chain,
		ContractAddress: addr,
		Label:           req.Label,
		Symbol:          strings.ToUpper(strings.TrimSpace(req.Symbol)),
		Decimals:        req.Decimals,
	}

	// API key path: create approval request.
	if !isPasskeyAuth(c) {
		approval, created := createPendingApproval(h.db, c, wallet.ID, "contract_add", proposed, h.approvalExpiry)
		if !created {
			return
		}
		writeAuditCtx(h.db, c, "contract_add", "pending", &wallet.ID, map[string]interface{}{
			"contract": addr, "symbol": proposed.Symbol, "decimals": proposed.Decimals, "label": proposed.Label, "approval_id": approval.ID,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"success":     true,
			"pending":     true,
			"approval_id": approval.ID,
			"message":     "Contract whitelist request submitted for approval",
		})
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
		jsonError(c, http.StatusInternalServerError, "db error")
		return
	}
	writeAuditCtx(h.db, c, "contract_add", "success", &wallet.ID, map[string]interface{}{
		"contract": addr, "symbol": proposed.Symbol, "decimals": proposed.Decimals, "label": proposed.Label,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "contract": proposed})
}

// ListContracts returns all whitelisted contracts for a wallet.
// GET /api/wallets/:id/contracts  (dual auth)
func (h *ContractHandler) ListContracts(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}

	var contracts []model.AllowedContract
	if err := h.db.Where("user_id = ? AND chain = ?", wallet.UserID, wallet.Chain).Order("created_at asc").Find(&contracts).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "db error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "contracts": contracts})
}

// UpdateContract modifies a whitelisted contract's settings.
// Passkey: applied immediately.
// API key: creates a pending approval for the passkey owner to review.
// PUT /api/wallets/:id/contracts/:cid  (dual auth)
func (h *ContractHandler) UpdateContract(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}

	cid, err := strconv.ParseUint(c.Param("cid"), 10, 64)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid contract id")
		return
	}

	var existing model.AllowedContract
	if err := h.db.Where("id = ? AND user_id = ? AND chain = ?", cid, wallet.UserID, wallet.Chain).First(&existing).Error; err != nil {
		jsonError(c, http.StatusNotFound, "contract not found")
		return
	}

	var req struct {
		LoginSessionID uint64      `json:"login_session_id"`
		Credential     interface{} `json:"credential"`
		Label          *string     `json:"label"`
		Symbol         *string     `json:"symbol"`
		Decimals       *int        `json:"decimals"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.Decimals != nil && (*req.Decimals < 0 || *req.Decimals > 77) {
		jsonError(c, http.StatusBadRequest, "decimals must be between 0 and 77")
		return
	}

	// Build proposed state by merging changes into existing.
	proposed := existing
	if req.Label != nil {
		proposed.Label = *req.Label
	}
	if req.Symbol != nil {
		proposed.Symbol = strings.ToUpper(strings.TrimSpace(*req.Symbol))
	}
	if req.Decimals != nil {
		proposed.Decimals = *req.Decimals
	}

	// API key path: create approval request.
	if !isPasskeyAuth(c) {
		approval, created := createPendingApproval(h.db, c, wallet.ID, "contract_update", proposed, h.approvalExpiry)
		if !created {
			return
		}
		writeAuditCtx(h.db, c, "contract_update", "pending", &wallet.ID, map[string]interface{}{
			"contract_id": cid, "contract": existing.ContractAddress, "symbol": proposed.Symbol, "decimals": proposed.Decimals, "label": proposed.Label, "approval_id": approval.ID,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"success":     true,
			"pending":     true,
			"approval_id": approval.ID,
			"message":     "Contract whitelist update submitted for approval",
		})
		return
	}

	// Passkey path: require a fresh hardware assertion before applying.
	if !verifyFreshPasskeyParsed(h.sdk, c, req.LoginSessionID, req.Credential, h.db) {
		return
	}

	updates := map[string]interface{}{}
	if req.Label != nil {
		updates["label"] = proposed.Label
	}
	if req.Symbol != nil {
		updates["symbol"] = proposed.Symbol
	}
	if req.Decimals != nil {
		updates["decimals"] = proposed.Decimals
	}

	if len(updates) == 0 {
		jsonError(c, http.StatusBadRequest, "no fields to update")
		return
	}

	if err := h.db.Model(&existing).Updates(updates).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "update failed")
		return
	}
	writeAuditCtx(h.db, c, "contract_update", "success", &wallet.ID, map[string]interface{}{
		"contract_id": cid, "contract": existing.ContractAddress, "symbol": proposed.Symbol, "decimals": proposed.Decimals, "label": proposed.Label,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "contract": proposed})
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

	cid, err := strconv.ParseUint(c.Param("cid"), 10, 64)
	if err != nil {
		jsonError(c, http.StatusBadRequest, "invalid contract id")
		return
	}

	var contract model.AllowedContract
	if err := h.db.Where("id = ? AND user_id = ? AND chain = ?", cid, wallet.UserID, wallet.Chain).First(&contract).Error; err != nil {
		jsonError(c, http.StatusNotFound, "contract not found")
		return
	}

	if err := h.db.Delete(&contract).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "delete failed")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true})
}

