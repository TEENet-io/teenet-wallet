package handler

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	sdk "github.com/TEENet-io/teenet-sdk/go"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/chain"
	"github.com/TEENet-io/teenet-wallet/model"
)

// walletMu provides per-wallet mutexes to prevent TOCTOU races on daily spend limits.
var walletMu sync.Map // map[string]*sync.Mutex

func getWalletMutex(walletID string) *sync.Mutex {
	v, _ := walletMu.LoadOrStore(walletID, &sync.Mutex{})
	return v.(*sync.Mutex)
}

// WalletHandler handles wallet CRUD, signing, and on-chain transfers.
type WalletHandler struct {
	db             *gorm.DB
	sdk            *sdk.Client
	baseURL        string // used to build approval_url
	approvalExpiry time.Duration
	maxWallets     int
	idempotency    *IdempotencyStore
}

func NewWalletHandler(db *gorm.DB, sdkClient *sdk.Client, baseURL string, approvalExpiry ...time.Duration) *WalletHandler {
	expiry := 30 * time.Minute
	if len(approvalExpiry) > 0 && approvalExpiry[0] > 0 {
		expiry = approvalExpiry[0]
	}
	return &WalletHandler{db: db, sdk: sdkClient, baseURL: baseURL, approvalExpiry: expiry, maxWallets: 20}
}

// SetMaxWallets sets the maximum number of wallets a user can create.
func (h *WalletHandler) SetMaxWallets(n int) { h.maxWallets = n }

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
	chainCfg, ok := model.Chains[req.Chain]
	if !ok {
		jsonError(c, http.StatusBadRequest, "unsupported chain: "+req.Chain)
		return
	}

	// Enforce per-user wallet limit to prevent TEE DKG abuse.
	var count int64
	h.db.Model(&model.Wallet{}).Where("user_id = ?", userID).Count(&count)
	if count >= int64(h.maxWallets) {
		jsonError(c, http.StatusBadRequest, fmt.Sprintf("wallet limit reached (max %d)", h.maxWallets))
		return
	}

	// Create a pending wallet record immediately so the user can see progress.
	wallet := model.Wallet{
		UserID:    userID,
		Chain:     req.Chain,
		Label:     req.Label,
		Curve:     chainCfg.Curve,
		Protocol:  chainCfg.Protocol,
		Status:    "creating",
		CreatedAt: time.Now(),
	}
	if err := h.db.Create(&wallet).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "db error")
		return
	}

	// Generate key via TEE-DAO (may take 1-2 min for ECDSA).
	var keyResult *sdk.GenerateKeyResult
	var genErr error
	if chainCfg.Protocol == "ecdsa" {
		keyResult, genErr = h.sdk.GenerateECDSAKey(c.Request.Context(), chainCfg.Curve)
	} else {
		keyResult, genErr = h.sdk.GenerateSchnorrKey(c.Request.Context(), chainCfg.Curve)
	}
	if genErr != nil || !keyResult.Success {
		msg := "key generation failed"
		if genErr != nil {
			msg = genErr.Error()
		} else if keyResult != nil {
			msg = keyResult.Message
		}
		h.db.Model(&wallet).Updates(map[string]interface{}{"status": "error"})
		jsonError(c, http.StatusBadGateway, msg)
		return
	}

	// Derive chain address from public key.
	address, addrErr := chain.DeriveAddress(chainCfg.Family, keyResult.PublicKey.KeyData)
	if addrErr != nil {
		h.db.Model(&wallet).Updates(map[string]interface{}{"status": "error"})
		jsonError(c, http.StatusInternalServerError, "address derivation failed: "+addrErr.Error())
		return
	}

	// Update wallet with final data.
	if err := h.db.Model(&wallet).Updates(map[string]interface{}{
		"key_name":   keyResult.PublicKey.Name,
		"public_key": keyResult.PublicKey.KeyData,
		"address":    address,
		"status":     "ready",
	}).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "db update failed")
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
		jsonError(c, http.StatusInternalServerError, "db error")
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

// DeleteWallet deletes a wallet record.
// DELETE /api/wallets/:id
func (h *WalletHandler) DeleteWallet(c *gin.Context) {
	if !verifyFreshPasskey(h.sdk, c) {
		return
	}
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	if err := h.db.Delete(&wallet).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "delete failed")
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

// SignRequest is the body for POST /api/wallets/:id/sign.
type SignRequest struct {
	Message   string                 `json:"message" binding:"required"` // hex or base64
	Encoding  string                 `json:"encoding"`                   // "hex" (default) or "base64"
	TxContext map[string]interface{} `json:"tx_context"`
}

// Sign signs a message, applying the approval policy if configured.
// POST /api/wallets/:id/sign
func (h *WalletHandler) Sign(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	if wallet.Status != "ready" {
		jsonError(c, http.StatusBadRequest, "wallet is not ready (status: "+wallet.Status+")")
		return
	}

	var req SignRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	msgBytes, err := decodeMessage(req.Message, req.Encoding)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	// Check approval policy (per currency derived from tx_context).
	if req.TxContext != nil {
		amount, txCurrency := extractAmountCurrency(req.TxContext)
		if txCurrency != "" {
			var policy model.ApprovalPolicy
			policyFound := h.db.Where("wallet_id = ? AND currency = ? AND enabled = ?", wallet.ID, txCurrency, true).First(&policy).Error == nil
			if policyFound && exceedsThreshold(amount, policy.ThresholdAmount) {
				// Create an approval request; signing deferred until human approves.
				// txParams is empty: /sign callers handle their own broadcast after receiving the signature.
				h.createApprovalRequest(c, wallet, req, msgBytes, &policy, "")
				return
			}
		}
	}

	// Direct sign via TEE.
	result, signErr := h.sdk.Sign(c.Request.Context(), msgBytes, wallet.KeyName)
	if signErr != nil {
		jsonError(c, http.StatusBadGateway, signErr.Error())
		return
	}
	if !result.Success {
		jsonError(c, http.StatusBadGateway, result.Error)
		return
	}
	writeAuditCtx(h.db, c, "sign", "success", &wallet.ID, map[string]interface{}{
		"chain": wallet.Chain, "msg_len": len(msgBytes),
	})
	c.JSON(http.StatusOK, gin.H{
		"status":         "signed",
		"signature":      "0x" + hex.EncodeToString(result.Signature),
		"wallet_address": wallet.Address,
		"chain":          wallet.Chain,
	})
}

// SetPolicy upserts an approval policy for a wallet+currency pair.
// PUT /api/wallets/:id/policy
func (h *WalletHandler) SetPolicy(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	var req struct {
		LoginSessionID  uint64      `json:"login_session_id"`
		Credential      interface{} `json:"credential"`
		ThresholdAmount string      `json:"threshold_amount" binding:"required"`
		Currency        string      `json:"currency"         binding:"required"`
		Enabled         *bool       `json:"enabled"`
		DailyLimit      string      `json:"daily_limit"` // optional: max total spend per UTC day
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	t, ok3 := new(big.Float).SetString(req.ThresholdAmount)
	if !ok3 || t.Sign() <= 0 {
		jsonError(c, http.StatusBadRequest, "threshold_amount must be a positive number")
		return
	}
	if req.DailyLimit != "" {
		dl, ok4 := new(big.Float).SetString(req.DailyLimit)
		if !ok4 || dl.Sign() <= 0 {
			jsonError(c, http.StatusBadRequest, "daily_limit must be a positive number")
			return
		}
	}

	currency := strings.ToUpper(strings.TrimSpace(req.Currency))

	// API key requests create a pending approval instead of applying directly.
	// Passkey sessions apply immediately (the human is already authenticated).
	if !isPasskeyAuth(c) {
		proposed := model.ApprovalPolicy{
			WalletID:        wallet.ID,
			Currency:        currency,
			ThresholdAmount: req.ThresholdAmount,
			Enabled:         enabled,
			DailyLimit:      req.DailyLimit,
		}
		approval, created := createPendingApproval(h.db, c, wallet.ID, "policy_change", proposed, h.approvalExpiry)
		if !created {
			return
		}
		writeAuditCtx(h.db, c, "policy_update", "pending", &wallet.ID, map[string]interface{}{
			"currency": currency, "threshold": req.ThresholdAmount, "approval_id": approval.ID,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"success":     true,
			"pending":     true,
			"approval_id": approval.ID,
			"message":     "Policy change submitted for approval",
		})
		return
	}

	// Passkey path: require a fresh hardware assertion before applying.
	if !verifyFreshPasskeyParsed(h.sdk, c, req.LoginSessionID, req.Credential) {
		return
	}

	var policy model.ApprovalPolicy
	if h.db.Where("wallet_id = ? AND currency = ?", wallet.ID, currency).First(&policy).Error != nil {
		policy = model.ApprovalPolicy{WalletID: wallet.ID, Currency: currency}
	}
	policy.ThresholdAmount = req.ThresholdAmount
	policy.Enabled = enabled
	policy.DailyLimit = req.DailyLimit

	if err := h.db.Save(&policy).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "save policy failed")
		return
	}
	writeAuditCtx(h.db, c, "policy_update", "success", &wallet.ID, map[string]interface{}{
		"currency": currency, "threshold": req.ThresholdAmount, "daily_limit": req.DailyLimit,
	})
	c.JSON(http.StatusOK, gin.H{"success": true, "policy": policy})
}

// GetPolicy returns all approval policies for a wallet.
// GET /api/wallets/:id/policy
// Optional query param ?currency=ETH to filter to a single policy.
func (h *WalletHandler) GetPolicy(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	q := h.db.Where("wallet_id = ?", wallet.ID)
	if currency := strings.ToUpper(strings.TrimSpace(c.Query("currency"))); currency != "" {
		q = q.Where("currency = ?", currency)
	}
	var policies []model.ApprovalPolicy
	if err := q.Order("currency").Find(&policies).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "db error")
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "policies": policies})
}

// DeletePolicy deletes the approval policy for a specific currency (Passkey only).
// DELETE /api/wallets/:id/policy?currency=ETH
func (h *WalletHandler) DeletePolicy(c *gin.Context) {
	if !verifyFreshPasskey(h.sdk, c) {
		return
	}
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	currency := strings.ToUpper(strings.TrimSpace(c.Query("currency")))
	if currency == "" {
		jsonError(c, http.StatusBadRequest, "currency query param is required")
		return
	}
	result := h.db.Where("wallet_id = ? AND currency = ?", wallet.ID, currency).Delete(&model.ApprovalPolicy{})
	if result.Error != nil {
		jsonError(c, http.StatusInternalServerError, "delete failed")
		return
	}
	if result.RowsAffected == 0 {
		jsonError(c, http.StatusNotFound, "policy not found")
		return
	}
	writeAuditCtx(h.db, c, "policy_update", "success", &wallet.ID, map[string]interface{}{
		"currency": currency, "action": "delete",
	})
	c.JSON(http.StatusOK, gin.H{"success": true})
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
		jsonError(c, http.StatusInternalServerError, "marshal tx_context failed")
		return
	}
	approval := model.ApprovalRequest{
		WalletID:  wallet.ID,
		UserID:    userID,
		Message:   req.Message,
		TxContext: string(txContextJSON),
		TxParams:  txParams,
		Status:    "pending",
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(h.approvalExpiry),
	}
	if err := h.db.Create(&approval).Error; err != nil {
		jsonError(c, http.StatusInternalServerError, "create approval request failed")
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
	})

	c.JSON(http.StatusAccepted, gin.H{
		"status":       "pending_approval",
		"approval_id":  approval.ID,
		"message":      msg,
		"tx_context":   req.TxContext,
		"threshold":    policy.ThresholdAmount,
		"currency":     policy.Currency,
		"approval_url": approvalURL,
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

	chainCfg := model.Chains[wallet.Chain]
	rpcURL := chainCfg.RPCURL

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
			if err := h.db.Where("wallet_id = ? AND contract_address = ?", wallet.ID, tokenContractAddr).First(&allowed).Error; err != nil {
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

			callData := chain.EncodeERC20Transfer(req.To, tokenUnits)
			txData, err := chain.BuildETHContractCallTx(rpcURL, wallet.Address, tokenContractAddr, callData, nil)
			if err != nil {
				slog.Error("build ERC-20 tx failed", "wallet_id", wallet.ID, "chain", wallet.Chain, "rpc_url", rpcURL, "error", err)
				jsonError(c, http.StatusBadGateway, "build contract tx: "+err.Error())
				return
			}
			signingMsg = txData.SigningHash
			b, marshalErr := json.Marshal(txData.Params)
			if marshalErr != nil {
				jsonError(c, http.StatusInternalServerError, "marshal tx params failed")
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
				jsonError(c, http.StatusBadGateway, "build tx: "+err.Error())
				return
			}
			signingMsg = txData.SigningHash
			b, marshalErr := json.Marshal(txData.Params)
			if marshalErr != nil {
				jsonError(c, http.StatusInternalServerError, "marshal tx params failed")
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
			if err := h.db.Where("wallet_id = ? AND contract_address = ?", wallet.ID, mintAddr).First(&allowed).Error; err != nil {
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
				jsonError(c, http.StatusBadGateway, "build SPL token tx: "+err.Error())
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
			amountF, _ := amount.Float64()
			txData, err := chain.BuildSOLTx(rpcURL, wallet.Address, req.To, amountF)
			if err != nil {
				jsonError(c, http.StatusBadGateway, "build tx: "+err.Error())
				return
			}
			signingMsg = txData.MessageBytes
			b, marshalErr := json.Marshal(txData.Params)
			if marshalErr != nil {
				jsonError(c, http.StatusInternalServerError, "marshal tx params failed")
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

	// Load the per-currency approval policy (if any).
	var policy model.ApprovalPolicy
	policyFound := h.db.Where("wallet_id = ? AND currency = ? AND enabled = ?", wallet.ID, currency, true).First(&policy).Error == nil

	// Check daily spend limit atomically (hard block — no approval path for this).
	if policyFound && policy.DailyLimit != "" {
		exceeded, msg, err := checkAndDeductDailyLimit(h.db, wallet.ID, currency, req.Amount)
		if err != nil {
			slog.Error("daily limit check error", "wallet_id", wallet.ID, "error", err)
			jsonError(c, http.StatusInternalServerError, "failed to check daily limit")
			return
		}
		if exceeded {
			jsonError(c, http.StatusBadRequest, msg)
			return
		}
	}

	// Check single-transaction approval threshold.
	if policyFound && exceedsThreshold(req.Amount, policy.ThresholdAmount) {
		// Create approval request — TxParams is stored so the approval handler can broadcast after signing.
		signReq := SignRequest{
			Message:   hex.EncodeToString(signingMsg),
			Encoding:  "hex",
			TxContext: txContext,
		}
		h.createApprovalRequest(c, wallet, signReq, signingMsg, &policy, txParamsJSON)
		return
	}

	// Direct path: sign via TEE.
	result, err := h.sdk.Sign(c.Request.Context(), signingMsg, wallet.KeyName)
	if err != nil || !result.Success {
		errMsg := "signing failed"
		if err != nil {
			errMsg = err.Error()
		} else if result != nil {
			errMsg = result.Error
		}
		slog.Error("TEE signing failed", "wallet_id", wallet.ID, "key", wallet.KeyName, "error", errMsg)
		jsonError(c, http.StatusBadGateway, "signing failed: "+errMsg)
		return
	}

	// Assemble and broadcast.
	txHash, err := broadcastSigned(wallet, txParamsJSON, result.Signature)
	if err != nil {
		respondBroadcastError(c, err)
		return
	}

	writeAuditCtx(h.db, c, "transfer", "success", &wallet.ID, map[string]interface{}{
		"to": req.To, "amount": req.Amount, "currency": currency,
		"chain": wallet.Chain, "tx_hash": txHash,
	})
	resp := gin.H{
		"status":   "completed",
		"tx_hash":  txHash,
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
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	if wallet.Status != "ready" {
		jsonError(c, http.StatusBadRequest, "wallet is not ready")
		return
	}
	chainCfg := model.Chains[wallet.Chain]
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
	amountF, _ := amount.Float64()

	txData, err := chain.BuildSOLWrapTx(chainCfg.RPCURL, wallet.Address, amountF)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "build wrap tx: "+err.Error())
		return
	}

	result, err := h.sdk.Sign(c.Request.Context(), txData.MessageBytes, wallet.KeyName)
	if err != nil || !result.Success {
		errMsg := "signing failed"
		if err != nil {
			errMsg = err.Error()
		} else if result != nil {
			errMsg = result.Error
		}
		jsonError(c, http.StatusBadGateway, "signing failed: "+errMsg)
		return
	}

	txParamsJSON, _ := json.Marshal(txData.Params)
	txHash, err := broadcastSigned(wallet, string(txParamsJSON), result.Signature)
	if err != nil {
		respondBroadcastError(c, err)
		return
	}

	writeAuditCtx(h.db, c, "wrap_sol", "success", &wallet.ID, map[string]interface{}{
		"amount": req.Amount, "chain": wallet.Chain, "tx_hash": txHash,
	})
	c.JSON(http.StatusOK, gin.H{
		"status":  "completed",
		"tx_hash": txHash,
		"chain":   wallet.Chain,
		"from":    wallet.Address,
		"amount":  req.Amount,
		"action":  "wrap",
	})
}

// UnwrapSOL unwraps all wSOL back to native SOL by closing the wSOL ATA.
// POST /api/wallets/:id/unwrap-sol
func (h *WalletHandler) UnwrapSOL(c *gin.Context) {
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	if wallet.Status != "ready" {
		jsonError(c, http.StatusBadRequest, "wallet is not ready")
		return
	}
	chainCfg := model.Chains[wallet.Chain]
	if chainCfg.Family != "solana" {
		jsonError(c, http.StatusBadRequest, "unwrap-sol is only supported on Solana chains")
		return
	}

	txData, err := chain.BuildSOLUnwrapTx(chainCfg.RPCURL, wallet.Address)
	if err != nil {
		jsonError(c, http.StatusBadGateway, "build unwrap tx: "+err.Error())
		return
	}

	result, err := h.sdk.Sign(c.Request.Context(), txData.MessageBytes, wallet.KeyName)
	if err != nil || !result.Success {
		errMsg := "signing failed"
		if err != nil {
			errMsg = err.Error()
		} else if result != nil {
			errMsg = result.Error
		}
		jsonError(c, http.StatusBadGateway, "signing failed: "+errMsg)
		return
	}

	txParamsJSON, _ := json.Marshal(txData.Params)
	txHash, err := broadcastSigned(wallet, string(txParamsJSON), result.Signature)
	if err != nil {
		respondBroadcastError(c, err)
		return
	}

	writeAuditCtx(h.db, c, "unwrap_sol", "success", &wallet.ID, map[string]interface{}{
		"chain": wallet.Chain, "tx_hash": txHash,
	})
	c.JSON(http.StatusOK, gin.H{
		"status":  "completed",
		"tx_hash": txHash,
		"chain":   wallet.Chain,
		"from":    wallet.Address,
		"action":  "unwrap",
	})
}

// broadcastSigned assembles a signed transaction and broadcasts it to the chain.
// Exported as a package-level function so the approval handler can reuse it.
func broadcastSigned(wallet model.Wallet, txParamsJSON string, sig []byte) (string, error) {
	cfg, ok := model.Chains[wallet.Chain]
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
			chain.ResetNonce(wallet.Address)
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
	if err := db.First(&wallet, "id = ?", id).Error; err != nil {
		jsonError(c, http.StatusNotFound, "wallet not found")
		return model.Wallet{}, false
	}
	if wallet.UserID != userID {
		jsonError(c, http.StatusForbidden, "access denied")
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

func exceedsThreshold(amount, threshold string) bool {
	a, ok1 := new(big.Float).SetString(amount)
	t, ok2 := new(big.Float).SetString(threshold)
	if !ok1 || !ok2 {
		return false
	}
	return a.Cmp(t) > 0
}

// respondBroadcastError returns 400 when the chain rejected the tx (user/input error)
// and 502 when the RPC endpoint itself was unreachable or returned an unexpected error.
func respondBroadcastError(c *gin.Context, err error) {
	msg := err.Error()
	// "rpc error:" means the node responded but rejected the tx — that's a client error.
	if strings.Contains(msg, "rpc error:") {
		jsonError(c, http.StatusBadRequest, "transaction rejected: "+msg)
		return
	}
	jsonError(c, http.StatusBadGateway, "broadcast failed: "+msg)
}

// checkAndDeductDailyLimit atomically checks whether adding `amount` would exceed
// the wallet's daily spend limit, and if not, deducts it from the remaining allowance.
// Uses a per-wallet mutex to prevent TOCTOU races where concurrent requests could
// both pass the limit check independently.
// Returns (exceeded, message, error).
func checkAndDeductDailyLimit(db *gorm.DB, walletID string, currency string, amount string) (bool, string, error) {
	mu := getWalletMutex(walletID)
	mu.Lock()
	defer mu.Unlock()

	// Re-read the policy from DB under the lock to get the latest spent counter.
	var policy model.ApprovalPolicy
	if err := db.Where("wallet_id = ? AND currency = ? AND enabled = ?", walletID, currency, true).First(&policy).Error; err != nil {
		return false, "", nil // no policy — not exceeded
	}
	if policy.DailyLimit == "" {
		return false, "", nil
	}

	dailyLimit, ok := new(big.Float).SetString(policy.DailyLimit)
	if !ok || dailyLimit.Sign() <= 0 {
		return false, "", nil
	}
	amountF, ok2 := new(big.Float).SetString(amount)
	if !ok2 || amountF.Sign() <= 0 {
		return false, "", nil
	}

	startOfDay := utcStartOfDay()
	currentSpent := policy.DailySpent
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
			"daily spend limit exceeded: limit %s %s, already spent %s %s today",
			policy.DailyLimit, policy.Currency, currentSpent, policy.Currency,
		), nil
	}

	// Deduct: persist the updated counter while still holding the lock.
	if err := db.Model(&policy).Updates(map[string]interface{}{
		"daily_spent":    newSpent.Text('f', -1),
		"daily_reset_at": resetAt,
	}).Error; err != nil {
		return false, "", fmt.Errorf("failed to update daily spent: %w", err)
	}
	return false, "", nil
}

// addDailySpent increments the daily spend counter on the policy after a successful broadcast.
// Uses the per-wallet mutex to stay consistent with checkAndDeductDailyLimit.
// Resets the counter first if a new UTC day has started. Silently ignores DB errors.
func addDailySpent(db *gorm.DB, policy *model.ApprovalPolicy, amount string) {
	amountF, ok := new(big.Float).SetString(amount)
	if !ok || amountF.Sign() <= 0 {
		return
	}

	mu := getWalletMutex(policy.WalletID)
	mu.Lock()
	defer mu.Unlock()

	// Re-read policy under lock for fresh DailySpent value.
	var fresh model.ApprovalPolicy
	if db.First(&fresh, policy.ID).Error != nil {
		return
	}

	startOfDay := utcStartOfDay()
	currentSpent := fresh.DailySpent
	resetAt := fresh.DailyResetAt
	if resetAt.Before(startOfDay) {
		currentSpent = "0"
		resetAt = startOfDay
	}
	spent, _ := new(big.Float).SetString(currentSpent)
	if spent == nil {
		spent = new(big.Float)
	}
	newSpent := new(big.Float).Add(spent, amountF)
	db.Model(&fresh).Updates(map[string]interface{}{
		"daily_spent":    newSpent.Text('f', -1),
		"daily_reset_at": resetAt,
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
