package handler

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/model"
)

// ChainHandler manages custom chain CRUD operations.
type ChainHandler struct {
	db          *gorm.DB
	validateURL func(string) error // SSRF validation function
}

// NewChainHandler creates a new ChainHandler.
// validateURL is called to check that RPC URLs are not targeting internal networks.
func NewChainHandler(db *gorm.DB, validateURL func(string) error) *ChainHandler {
	return &ChainHandler{db: db, validateURL: validateURL}
}

// AddChain handles POST /api/chains — adds a new custom EVM chain.
func (h *ChainHandler) AddChain(c *gin.Context) {
	var req struct {
		Name     string `json:"name"`
		Label    string `json:"label"`
		Currency string `json:"currency"`
		RPCURL   string `json:"rpc_url"`
		ChainID  uint64 `json:"chain_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// Validate required fields.
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	if req.RPCURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rpc_url is required"})
		return
	}

	// SSRF protection: validate the RPC URL is not targeting internal networks.
	if err := h.validateURL(req.RPCURL); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid rpc_url: %s", err.Error())})
		return
	}

	// Only EVM custom chains are supported for now.
	// (Solana requires different protocol/curve and additional validation.)
	family := "evm"

	// Reject if it would collide with a built-in chain.
	if existing, exists := model.GetChain(req.Name); exists && !existing.Custom {
		c.JSON(http.StatusConflict, gin.H{"error": "cannot overwrite a built-in chain"})
		return
	}
	// Reject if a custom chain with this name already exists.
	if existing, exists := model.GetChain(req.Name); exists && existing.Custom {
		c.JSON(http.StatusConflict, gin.H{"error": "custom chain already exists"})
		return
	}

	row := model.CustomChain{
		Name:     req.Name,
		Label:    req.Label,
		Currency: req.Currency,
		Family:   family,
		RPCURL:   req.RPCURL,
		ChainID:  req.ChainID,
	}
	if err := h.db.Create(&row).Error; err != nil {
		slog.Error("create custom chain", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist custom chain"})
		return
	}

	cfg := model.ChainConfig{
		Name:     row.Name,
		Label:    row.Label,
		Protocol: "ecdsa",
		Curve:    "secp256k1",
		Currency: row.Currency,
		Family:   family,
		RPCURL:   row.RPCURL,
		ChainID:  row.ChainID,
		Custom:   true,
	}
	model.SetChain(cfg.Name, cfg)

	slog.Info("custom chain added", "name", cfg.Name, "chain_id", cfg.ChainID)
	c.JSON(http.StatusCreated, gin.H{"success": true, "chain": cfg})
}

// DeleteChain handles DELETE /api/chains/:name — removes a custom chain.
func (h *ChainHandler) DeleteChain(c *gin.Context) {
	name := c.Param("name")

	existing, exists := model.GetChain(name)
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "chain not found"})
		return
	}
	if !existing.Custom {
		c.JSON(http.StatusForbidden, gin.H{"error": "cannot delete a built-in chain"})
		return
	}

	// Refuse if any wallet is using this chain.
	var count int64
	if err := h.db.Model(&model.Wallet{}).Where("chain = ?", name).Count(&count).Error; err != nil {
		slog.Error("check wallets on chain", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to check wallets"})
		return
	}
	if count > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "chain has existing wallets; delete them first"})
		return
	}

	if err := h.db.Where("name = ?", name).Delete(&model.CustomChain{}).Error; err != nil {
		slog.Error("delete custom chain", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete custom chain"})
		return
	}
	model.DeleteChain(name)

	slog.Info("custom chain deleted", "name", name)
	c.JSON(http.StatusOK, gin.H{"success": true})
}
