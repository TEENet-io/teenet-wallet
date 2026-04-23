// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/model"
)

// FaucetHandler proxies testnet faucet requests to an internal faucet service.
type FaucetHandler struct {
	db        *gorm.DB
	faucetURL string     // internal faucet service URL
	client    http.Client
}

// NewFaucetHandler creates a FaucetHandler. If faucetURL is empty, all requests return 503.
func NewFaucetHandler(db *gorm.DB, faucetURL string) *FaucetHandler {
	return &FaucetHandler{
		db:        db,
		faucetURL: faucetURL,
		client:    http.Client{Timeout: 30 * time.Second},
	}
}

// chainToFaucetChain maps wallet chain names to faucet-robot chain IDs.
var chainToFaucetChain = map[string]string{
	"sepolia":       "eth_sepolia",
	"base-sepolia":  "base_sepolia",
	"solana-devnet": "solana_devnet",
}

// Claim handles POST /api/faucet — requests test tokens for a wallet.
func (h *FaucetHandler) Claim(c *gin.Context) {
	if h.faucetURL == "" {
		jsonError(c, http.StatusServiceUnavailable, "faucet service not configured")
		return
	}

	userID := mustUserID(c)
	if c.IsAborted() {
		return
	}

	var req struct {
		WalletID string `json:"wallet_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, "wallet_id is required")
		return
	}

	// Look up wallet, verify ownership.
	var wallet model.Wallet
	if err := h.db.Where("id = ? AND user_id = ?", req.WalletID, userID).First(&wallet).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			jsonError(c, http.StatusNotFound, "wallet not found")
		} else {
			respondInternalError(c, "database error", err, gin.H{"stage": "faucet_db_lookup"})
		}
		return
	}

	// Only testnet wallets allowed.
	faucetChain, ok := chainToFaucetChain[wallet.Chain]
	if !ok {
		jsonError(c, http.StatusBadRequest, fmt.Sprintf("faucet not available for chain %q (testnet only)", wallet.Chain))
		return
	}

	// Call internal faucet service.
	body, _ := json.Marshal(map[string]string{
		"address": wallet.Address,
		"chain":   faucetChain,
	})

	resp, err := h.client.Post(h.faucetURL+"/api/claim", "application/json", bytes.NewReader(body))
	if err != nil {
		slog.Error("faucet request failed", "chain", wallet.Chain, "error", err.Error())
		jsonErrorDetails(c, http.StatusUnprocessableEntity, "faucet service unavailable", gin.H{
			"stage": "faucet_request", "chain": wallet.Chain, "address": wallet.Address,
			"reason": sanitizeErrString(err),
		})
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1 MB max

	// Forward faucet response as-is.
	if resp.StatusCode != http.StatusOK {
		var faucetErr struct {
			Error string `json:"error"`
		}
		if json.Unmarshal(respBody, &faucetErr) == nil && faucetErr.Error != "" {
			jsonErrorDetails(c, resp.StatusCode, faucetErr.Error, gin.H{
				"stage": "faucet_claim", "chain": wallet.Chain, "address": wallet.Address,
			})
			return
		}
		jsonErrorDetails(c, resp.StatusCode, "faucet request failed", gin.H{
			"stage": "faucet_claim", "chain": wallet.Chain, "address": wallet.Address,
		})
		return
	}

	var result struct {
		TxHash string `json:"tx_hash"`
		Amount string `json:"amount"`
		Chain  string `json:"chain"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "invalid faucet response", gin.H{"stage": "faucet_response_parse"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"tx_hash": result.TxHash,
		"amount":  result.Amount,
		"chain":   wallet.Chain,
		"address": wallet.Address,
	})
}
