package handler

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sdk "github.com/TEENet-io/teenet-sdk/go"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/chain"
	"github.com/TEENet-io/teenet-wallet/model"
)

// ContractCallHandler handles general-purpose smart contract calls.
type ContractCallHandler struct {
	db             *gorm.DB
	sdk            *sdk.Client
	baseURL        string
	approvalExpiry time.Duration
	prices         *PriceService
}

func NewContractCallHandler(db *gorm.DB, sdkClient *sdk.Client, baseURL string, approvalExpiry ...time.Duration) *ContractCallHandler {
	expiry := 30 * time.Minute
	if len(approvalExpiry) > 0 && approvalExpiry[0] > 0 {
		expiry = approvalExpiry[0]
	}
	return &ContractCallHandler{db: db, sdk: sdkClient, baseURL: baseURL, approvalExpiry: expiry}
}

// SetPriceService sets the USD price service used for threshold conversion.
func (h *ContractCallHandler) SetPriceService(ps *PriceService) { h.prices = ps }

// ContractCallRequest is the body for POST /api/wallets/:id/contract-call.
type ContractCallRequest struct {
	Contract  string                 `json:"contract" binding:"required"`
	FuncSig   string                 `json:"func_sig"`                    // EVM only
	Args      []interface{}          `json:"args"`                        // EVM only
	Value     string                 `json:"value"`                       // EVM only: ETH to send (optional, in ETH units)
	Memo      string                 `json:"memo"`
	Accounts  []chain.SOLAccountMeta `json:"accounts"`                    // Solana only
	Data      string                 `json:"data"`                        // Solana only: hex instruction data
}

// ContractCall executes a smart contract call with two-layer security:
//  1. Contract address whitelist
//  2. Approval — API Key auth always requires Passkey approval; Passkey auth executes directly
//
// POST /api/wallets/:id/contract-call
func (h *ContractCallHandler) ContractCall(c *gin.Context) {
	// Load and validate wallet.
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	if wallet.Status != "ready" {
		jsonError(c, http.StatusBadRequest, "wallet is not ready (status: "+wallet.Status+")")
		return
	}

	chainCfg, cfgOk := model.GetChain(wallet.Chain)
	if !cfgOk {
		jsonError(c, http.StatusBadRequest, "unsupported chain")
		return
	}
	switch chainCfg.Family {
	case "evm":
		h.contractCallEVM(c, wallet, chainCfg)
	case "solana":
		h.contractCallSolana(c, wallet, chainCfg)
	default:
		jsonError(c, http.StatusBadRequest, "contract calls not supported on "+chainCfg.Family)
	}
}

var revertReasonRe = regexp.MustCompile(`execution reverted(?:: )?(.+)?$`)

func extractRevertReason(err error) string {
	if err == nil {
		return ""
	}
	m := revertReasonRe.FindStringSubmatch(err.Error())
	if len(m) >= 2 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func contractCallDebug(wallet model.Wallet, contractAddr, methodName, funcSig string, args []interface{}, value string, selector string, calldataLen int) gin.H {
	return gin.H{
		"wallet_address": wallet.Address,
		"chain":          wallet.Chain,
		"method":         methodName,
		"args":           args,
		"value":          value,
		"tx_preview": gin.H{
			"to":           contractAddr,
			"value":        value,
			"selector":     selector,
			"calldata_len": calldataLen,
		},
	}
}

func respondContractCallStageError(c *gin.Context, status int, stage string, msg string, wallet model.Wallet, contractAddr, methodName, funcSig string, args []interface{}, value string, selector string, calldataLen int, err error) {
	revertReason := extractRevertReason(err)
	jsonErrorDetails(c, status, msg, gin.H{
		"stage":         stage,
		"contract":      contractAddr,
		"func_sig":      funcSig,
		"selector":      selector,
		"revert_reason": revertReason,
		"debug":         contractCallDebug(wallet, contractAddr, methodName, funcSig, args, value, selector, calldataLen),
	})
}

// contractCallEVM implements contract call logic for EVM chains.
func (h *ContractCallHandler) contractCallEVM(c *gin.Context, wallet model.Wallet, chainCfg model.ChainConfig) {
	var req ContractCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonErrorDetails(c, http.StatusBadRequest, err.Error(), gin.H{"stage": "bind_json"})
		return
	}

	if req.FuncSig == "" {
		jsonErrorDetails(c, http.StatusBadRequest, "func_sig is required for EVM contract calls", gin.H{"stage": "validation"})
		return
	}

	// Normalize contract address.
	contractAddr, addrErr := normalizeEVMAddress(req.Contract)
	if addrErr != nil {
		jsonErrorDetails(c, http.StatusBadRequest, "contract: "+addrErr.Error(), gin.H{"stage": "validation", "contract": req.Contract})
		return
	}

	// Layer 1: Whitelist check.
	var allowed model.AllowedContract
	if err := h.db.Where("user_id = ? AND chain = ? AND contract_address = ?", wallet.UserID, wallet.Chain, contractAddr).First(&allowed).Error; err != nil {
		jsonError(c, http.StatusForbidden, "contract not whitelisted: "+contractAddr)
		return
	}

	// Extract method name from func_sig (everything before the first "(").
	methodName, err := extractMethodName(req.FuncSig)
	if err != nil {
		jsonErrorDetails(c, http.StatusBadRequest, err.Error(), gin.H{"stage": "validation", "func_sig": req.FuncSig})
		return
	}

	// Encode calldata (validation only — no RPC yet).
	calldata, encErr := chain.EncodeCall(req.FuncSig, req.Args)
	if encErr != nil {
		respondContractCallStageError(c, http.StatusBadRequest, "encode_calldata", "encode calldata: "+encErr.Error(), wallet, contractAddr, methodName, req.FuncSig, req.Args, req.Value, "", 0, encErr)
		return
	}

	// Parse optional ETH value (ETH → Wei).
	var valueWei *big.Int
	if req.Value != "" {
		ethVal, ok2 := new(big.Float).SetString(req.Value)
		if !ok2 || ethVal.Sign() < 0 {
			jsonError(c, http.StatusBadRequest, "invalid value: must be a non-negative number in ETH")
			return
		}
		if ethVal.Sign() > 0 {
			// 1 ETH = 1e18 Wei
			weiPerEth := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
			weiF := new(big.Float).Mul(ethVal, weiPerEth)
			valueWei, _ = weiF.Int(nil)
		}
	}

	// Build tx (hits RPC) before the approval check — both paths need ETHTxParams for correctness.
	// The approval path stores ETHTxParams so RebuildETHTx can refresh nonce/gas on approve,
	// consistent with how /transfer works.
	txData, buildErr := chain.BuildETHContractCallTx(chainCfg.RPCURL, wallet.Address, contractAddr, calldata, valueWei)
	if buildErr != nil {
		selector := ""
		if len(calldata) >= 4 {
			selector = "0x" + hex.EncodeToString(calldata[:4])
		}
		revertReason := extractRevertReason(buildErr)
		slog.Error("contract-call build tx failed",
			"wallet_id", wallet.ID,
			"wallet_address", wallet.Address,
			"chain", wallet.Chain,
			"contract", contractAddr,
			"method", methodName,
			"func_sig", req.FuncSig,
			"args", req.Args,
			"value", req.Value,
			"selector", selector,
			"revert_reason", revertReason,
			"error", buildErr.Error(),
		)
		respondContractCallStageError(c, http.StatusUnprocessableEntity, "estimate_gas", "build contract tx: "+buildErr.Error(), wallet, contractAddr, methodName, req.FuncSig, req.Args, req.Value, selector, len(calldata), buildErr)
		return
	}

	txParamsJSON, marshalErr := json.Marshal(txData.Params)
	if marshalErr != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "marshal tx params failed", gin.H{"stage": "marshal_tx_params"})
		return
	}

	signingMsg := txData.SigningHash

	// Layer 2: Security decision.
	// API Key auth: contract operations always require passkey approval.
	// Passkey auth: proceeds directly unless a value transfer exceeds the approval policy threshold.
	needsApproval := false
	var approvalReason string
	if !isPasskeyAuth(c) {
		needsApproval = true
		approvalReason = "contract operations require passkey approval"
	}

	// Compute effective USD amount for display context (payable ETH value only).
	var effectiveUSD float64
	if h.prices != nil && valueWei != nil && valueWei.Sign() > 0 && req.Value != "" {
		currency := chainCfg.Currency
		if usdPrice, priceErr := h.prices.GetUSDPrice(currency); priceErr == nil && usdPrice > 0 {
			if a, ok := new(big.Float).SetString(req.Value); ok {
				f, _ := a.Float64()
				effectiveUSD = f * usdPrice
			}
		}
	}

	// Passkey auth with ETH value: check approval policy threshold.
	if !needsApproval && effectiveUSD > 0 {
		var policy model.ApprovalPolicy
		if h.db.Where("wallet_id = ? AND enabled = ?", wallet.ID, true).First(&policy).Error == nil {
			if exceedsUSDThreshold(effectiveUSD, policy.ThresholdUSD) {
				needsApproval = true
				approvalReason = fmt.Sprintf("value transfer ($%.2f USD) exceeds approval threshold ($%s USD)", effectiveUSD, policy.ThresholdUSD)
			}
		}
	}

	// Build display context for both approval and direct paths.
	txContext := map[string]interface{}{
		"type":     "contract_call",
		"from":     wallet.Address,
		"contract": contractAddr,
		"method":   methodName,
		"func_sig": req.FuncSig,
		"args":     req.Args,
		"memo":     req.Memo,
	}
	if req.Value != "" {
		txContext["value_eth"] = req.Value
	}
	if effectiveUSD > 0 {
		txContext["amount_usd"] = fmt.Sprintf("%.2f", effectiveUSD)
	}

	if needsApproval {
		txContextJSON, _ := json.Marshal(txContext)

		userID := mustUserID(c)
		if c.IsAborted() {
			return
		}
		ccAM, ccAKL := authInfo(c)
		approval := model.ApprovalRequest{
			WalletID:     &wallet.ID,
			UserID:       userID,
			ApprovalType: "contract_call",
			Message:      hex.EncodeToString(signingMsg),
			TxContext:    string(txContextJSON),
			TxParams:     string(txParamsJSON),
			Status:       "pending",
			AuthMode:     ccAM,
			APIKeyPrefix: ccAKL,
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(h.approvalExpiry),
		}
		if err := h.db.Create(&approval).Error; err != nil {
			jsonErrorDetails(c, http.StatusInternalServerError, "create approval request failed", gin.H{"stage": "create_approval"})
			return
		}
		approvalURL := fmt.Sprintf("%s/#/approve/%d", requestBaseURL(c, h.baseURL), approval.ID)
		writeAuditCtx(h.db, c, "contract_call", "pending", &wallet.ID, map[string]interface{}{
			"approval_id": approval.ID, "tx_context": txContext,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"status":       "pending_approval",
			"approval_id":  approval.ID,
			"message":      approvalReason,
			"tx_context":   txContext,
			"approval_url": approvalURL,
		})
		return
	}

	result, signErr := h.sdk.Sign(c.Request.Context(), signingMsg, wallet.KeyName)
	if signErr != nil || !result.Success {
		errMsg := "signing failed"
		if signErr != nil {
			errMsg = signErr.Error()
		} else if result != nil {
			errMsg = result.Error
		}
		slog.Error("contract-call signing failed",
			"wallet_id", wallet.ID, "chain", wallet.Chain,
			"contract", contractAddr, "method", methodName, "error", errMsg,
		)
		jsonErrorDetails(c, http.StatusUnprocessableEntity, "signing failed: "+errMsg, gin.H{
			"stage": "signing", "wallet_id": wallet.ID, "chain": wallet.Chain,
			"contract": contractAddr, "method": methodName,
		})
		return
	}

	txHash, broadcastErr := broadcastSigned(wallet, string(txParamsJSON), result.Signature)
	if broadcastErr != nil {
		slog.Error("contract-call broadcast failed",
			"wallet_id", wallet.ID, "chain", wallet.Chain,
			"contract", contractAddr, "method", methodName, "error", broadcastErr.Error(),
		)
		respondBroadcastErrorDetails(c, broadcastErr, gin.H{
			"stage": "broadcast", "wallet_id": wallet.ID, "chain": wallet.Chain,
			"contract": contractAddr, "method": methodName,
		})
		return
	}

	writeAuditCtx(h.db, c, "contract_call", "success", &wallet.ID, map[string]interface{}{
		"tx_hash": txHash, "tx_context": txContext,
	})
	c.JSON(http.StatusOK, gin.H{
		"status":         "completed",
		"tx_hash":        txHash,
		"chain":          wallet.Chain,
		"from":           wallet.Address,
		"contract":       contractAddr,
		"method":         methodName,
		"wallet_address": wallet.Address,
	})
}

// contractCallSolana implements contract call logic for Solana chains (program calls).
func (h *ContractCallHandler) contractCallSolana(c *gin.Context, wallet model.Wallet, chainCfg model.ChainConfig) {
	var req ContractCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	programID := strings.TrimSpace(req.Contract)
	if _, err := chain.Base58Decode(programID); err != nil {
		jsonError(c, http.StatusBadRequest, "contract: invalid Solana program ID")
		return
	}

	// Layer 1: Whitelist
	var allowed model.AllowedContract
	if err := h.db.Where("user_id = ? AND chain = ? AND contract_address = ?", wallet.UserID, wallet.Chain, programID).First(&allowed).Error; err != nil {
		jsonError(c, http.StatusForbidden, "program not whitelisted: "+programID)
		return
	}

	// Decode instruction data
	instrData, decErr := hex.DecodeString(strings.TrimPrefix(req.Data, "0x"))
	if decErr != nil {
		jsonError(c, http.StatusBadRequest, "data: invalid hex")
		return
	}

	// Validate accounts
	if len(req.Accounts) == 0 {
		jsonError(c, http.StatusBadRequest, "accounts: at least one account is required")
		return
	}
	for i, a := range req.Accounts {
		if _, err := chain.Base58Decode(a.Pubkey); err != nil {
			jsonError(c, http.StatusBadRequest, fmt.Sprintf("accounts[%d].pubkey: invalid base58", i))
			return
		}
	}

	// Build tx
	txData, buildErr := chain.BuildSOLProgramCallTx(chainCfg.RPCURL, wallet.Address, programID, req.Accounts, instrData)
	if buildErr != nil {
		slog.Error("solana program-call build tx failed",
			"wallet_id", wallet.ID, "chain", wallet.Chain,
			"program_id", programID, "error", buildErr.Error(),
		)
		jsonErrorDetails(c, http.StatusUnprocessableEntity, "build program call tx: "+buildErr.Error(), gin.H{
			"stage": "build_tx", "wallet_id": wallet.ID, "chain": wallet.Chain,
			"program_id": programID,
		})
		return
	}
	txParamsJSON, _ := json.Marshal(txData.Params)
	signingMsg := txData.MessageBytes

	// Layer 2: Security decision.
	// Passkey auth: human is already present — proceed directly.
	// API Key auth: contract operations always require passkey approval.
	needsApproval := false
	var approvalReason string
	if !isPasskeyAuth(c) {
		needsApproval = true
		approvalReason = "contract operations require passkey approval"
	}

	txContext := map[string]interface{}{
		"type":       "program_call",
		"from":       wallet.Address,
		"program_id": programID,
		"accounts":   req.Accounts,
		"data":       req.Data,
		"memo":       req.Memo,
		"chain":      wallet.Chain,
	}

	if needsApproval {
		txContextJSON, _ := json.Marshal(txContext)
		userID := mustUserID(c)
		if c.IsAborted() {
			return
		}
		ccAM, ccAKL := authInfo(c)
		approval := model.ApprovalRequest{
			WalletID:     &wallet.ID,
			UserID:       userID,
			ApprovalType: "contract_call",
			Message:      hex.EncodeToString(signingMsg),
			TxContext:    string(txContextJSON),
			TxParams:     string(txParamsJSON),
			Status:       "pending",
			AuthMode:     ccAM,
			APIKeyPrefix: ccAKL,
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(h.approvalExpiry),
		}
		if err := h.db.Create(&approval).Error; err != nil {
			jsonErrorDetails(c, http.StatusInternalServerError, "create approval request failed", gin.H{"stage": "create_approval"})
			return
		}
		approvalURL := fmt.Sprintf("%s/#/approve/%d", requestBaseURL(c, h.baseURL), approval.ID)
		writeAuditCtx(h.db, c, "contract_call", "pending", &wallet.ID, map[string]interface{}{
			"approval_id": approval.ID, "tx_context": txContext,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"status":       "pending_approval",
			"approval_id":  approval.ID,
			"message":      approvalReason,
			"tx_context":   txContext,
			"approval_url": approvalURL,
		})
		return
	}

	result, signErr := h.sdk.Sign(c.Request.Context(), signingMsg, wallet.KeyName)
	if signErr != nil || !result.Success {
		errMsg := "signing failed"
		if signErr != nil {
			errMsg = signErr.Error()
		} else if result != nil {
			errMsg = result.Error
		}
		slog.Error("solana program-call signing failed",
			"wallet_id", wallet.ID, "chain", wallet.Chain,
			"program_id", programID, "error", errMsg,
		)
		jsonErrorDetails(c, http.StatusUnprocessableEntity, "signing failed: "+errMsg, gin.H{
			"stage": "signing", "wallet_id": wallet.ID, "chain": wallet.Chain,
			"program_id": programID,
		})
		return
	}

	txHash, broadcastErr := chain.AssembleAndBroadcastSOLProgram(chainCfg.RPCURL, txData.Params, result.Signature)
	if broadcastErr != nil {
		slog.Error("solana program-call broadcast failed",
			"wallet_id", wallet.ID, "chain", wallet.Chain,
			"program_id", programID, "error", broadcastErr.Error(),
		)
		respondBroadcastErrorDetails(c, broadcastErr, gin.H{
			"stage": "broadcast", "wallet_id": wallet.ID, "chain": wallet.Chain,
			"program_id": programID,
		})
		return
	}

	writeAuditCtx(h.db, c, "contract_call", "success", &wallet.ID, map[string]interface{}{
		"tx_hash": txHash, "tx_context": txContext,
	})
	c.JSON(http.StatusOK, gin.H{
		"status":         "completed",
		"tx_hash":        txHash,
		"chain":          wallet.Chain,
		"from":           wallet.Address,
		"program_id":     programID,
		"wallet_address": wallet.Address,
	})
}

// ApproveTokenRequest is the body for POST /api/wallets/:id/approve-token.
type ApproveTokenRequest struct {
	Contract string `json:"contract" binding:"required"`
	Spender  string `json:"spender" binding:"required"`
	Amount   string `json:"amount" binding:"required"`
}

// RevokeApprovalRequest is the body for POST /api/wallets/:id/revoke-approval.
type RevokeApprovalRequest struct {
	Contract string `json:"contract" binding:"required"`
	Spender  string `json:"spender" binding:"required"`
}

// ApproveToken is a convenience endpoint that calls ERC-20 approve(spender, amount).
// API Key auth: always requires Passkey approval. Passkey auth: executes directly.
//
// POST /api/wallets/:id/approve-token
func (h *ContractCallHandler) ApproveToken(c *gin.Context) {
	var req ApproveTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	h.executeApprove(c, req.Contract, req.Spender, req.Amount, "approve_token")
}

// RevokeApproval is a convenience endpoint that calls ERC-20 approve(spender, 0),
// effectively revoking a previously granted token allowance.
// API Key auth: always requires Passkey approval. Passkey auth: executes directly.
//
// POST /api/wallets/:id/revoke-approval
func (h *ContractCallHandler) RevokeApproval(c *gin.Context) {
	var req RevokeApprovalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	h.executeApprove(c, req.Contract, req.Spender, "0", "revoke_approval")
}

// executeApprove implements the shared logic for ApproveToken and RevokeApproval.
// It encodes an ERC-20 approve(spender, amount) call and either queues an
// ApprovalRequest (API Key auth) or signs+broadcasts directly (Passkey auth).
func (h *ContractCallHandler) executeApprove(c *gin.Context, contractRaw, spenderRaw, amount, auditAction string) {
	// Load and validate wallet.
	wallet, ok := loadUserWallet(c, h.db)
	if !ok {
		return
	}
	if wallet.Status != "ready" {
		jsonError(c, http.StatusBadRequest, "wallet is not ready (status: "+wallet.Status+")")
		return
	}

	chainCfg, cfgOk := model.GetChain(wallet.Chain)
	if !cfgOk || chainCfg.Family != "evm" {
		jsonError(c, http.StatusBadRequest, "contract calls are only supported on EVM chains")
		return
	}

	// Normalize contract and spender addresses.
	contractAddr, addrErr := normalizeEVMAddress(contractRaw)
	if addrErr != nil {
		jsonError(c, http.StatusBadRequest, "contract: "+addrErr.Error())
		return
	}
	spenderAddr, spenderErr := normalizeEVMAddress(spenderRaw)
	if spenderErr != nil {
		jsonError(c, http.StatusBadRequest, "spender: "+spenderErr.Error())
		return
	}

	// Whitelist check.
	var allowed model.AllowedContract
	if err := h.db.Where("user_id = ? AND chain = ? AND contract_address = ?", wallet.UserID, wallet.Chain, contractAddr).First(&allowed).Error; err != nil {
		jsonError(c, http.StatusForbidden, "contract not whitelisted: "+contractAddr)
		return
	}

	// Parse amount using AllowedContract.Decimals (default 18).
	var tokenAmount *big.Int
	if amount == "0" {
		tokenAmount = big.NewInt(0)
	} else {
		decimals := allowed.Decimals
		if decimals == 0 {
			decimals = 18
		}
		amtFloat, ok2 := new(big.Float).SetString(amount)
		if !ok2 || amtFloat.Sign() < 0 {
			jsonError(c, http.StatusBadRequest, "invalid amount: must be a non-negative number")
			return
		}
		multiplier := new(big.Float).SetInt(new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil))
		rawF := new(big.Float).Mul(amtFloat, multiplier)
		tokenAmount, _ = rawF.Int(nil)
	}

	// Encode calldata via ERC-20 approve(spender, amount).
	calldata := chain.EncodeERC20Approve(spenderAddr, tokenAmount)

	// Build tx (hits RPC).
	rpcURL := chainCfg.RPCURL
	walletAddr := wallet.Address
	txData, buildErr := chain.BuildETHContractCallTx(rpcURL, walletAddr, contractAddr, calldata, nil)
	if buildErr != nil {
		slog.Error("approve-token build tx failed",
			"wallet_id", wallet.ID, "chain", wallet.Chain,
			"contract", contractAddr, "spender", spenderAddr,
			"action", auditAction, "error", buildErr.Error(),
		)
		jsonErrorDetails(c, http.StatusUnprocessableEntity, "build approve tx: "+buildErr.Error(), gin.H{
			"stage": "build_tx", "wallet_id": wallet.ID, "chain": wallet.Chain,
			"contract": contractAddr, "spender": spenderAddr, "action": auditAction,
			"revert_reason": extractRevertReason(buildErr),
		})
		return
	}

	txParamsJSON, marshalErr := json.Marshal(txData.Params)
	if marshalErr != nil {
		jsonErrorDetails(c, http.StatusInternalServerError, "marshal tx params failed", gin.H{"stage": "marshal_tx_params"})
		return
	}

	signingMsg := txData.SigningHash

	txContext := map[string]interface{}{
		"type":     "contract_call",
		"from":     walletAddr,
		"contract": contractAddr,
		"spender":  spenderAddr,
		"amount":   amount,
		"symbol":   allowed.Symbol,
		"decimals": allowed.Decimals,
		"action":   auditAction,
	}

	// API Key auth: always requires Passkey approval. Passkey auth: executes directly.
	if !isPasskeyAuth(c) {
		txContextJSON, _ := json.Marshal(txContext)
		userID := mustUserID(c)
		if c.IsAborted() {
			return
		}
		ccAM, ccAKL := authInfo(c)
		approval := model.ApprovalRequest{
			WalletID:     &wallet.ID,
			UserID:       userID,
			ApprovalType: "contract_call",
			Message:      hex.EncodeToString(signingMsg),
			TxContext:    string(txContextJSON),
			TxParams:     string(txParamsJSON),
			Status:       "pending",
			AuthMode:     ccAM,
			APIKeyPrefix: ccAKL,
			CreatedAt:   time.Now(),
			ExpiresAt:   time.Now().Add(h.approvalExpiry),
		}
		if err := h.db.Create(&approval).Error; err != nil {
			jsonErrorDetails(c, http.StatusInternalServerError, "create approval request failed", gin.H{"stage": "create_approval"})
			return
		}
		approvalURL := fmt.Sprintf("%s/#/approve/%d", requestBaseURL(c, h.baseURL), approval.ID)
		writeAuditCtx(h.db, c, auditAction, "pending", &wallet.ID, map[string]interface{}{
			"approval_id": approval.ID, "tx_context": txContext,
		})
		c.JSON(http.StatusAccepted, gin.H{
			"status":       "pending_approval",
			"approval_id":  approval.ID,
			"message":      fmt.Sprintf("%s requires passkey approval", auditAction),
			"tx_context":   txContext,
			"approval_url": approvalURL,
		})
		return
	}

	// Passkey auth — sign and broadcast directly.
	result, signErr := h.sdk.Sign(c.Request.Context(), signingMsg, wallet.KeyName)
	if signErr != nil || !result.Success {
		errMsg := "signing failed"
		if signErr != nil {
			errMsg = signErr.Error()
		} else if result != nil {
			errMsg = result.Error
		}
		slog.Error("approve-token signing failed",
			"wallet_id", wallet.ID, "chain", wallet.Chain,
			"contract", contractAddr, "spender", spenderAddr,
			"action", auditAction, "error", errMsg,
		)
		jsonErrorDetails(c, http.StatusUnprocessableEntity, "signing failed: "+errMsg, gin.H{
			"stage": "signing", "wallet_id": wallet.ID, "chain": wallet.Chain,
			"contract": contractAddr, "spender": spenderAddr, "action": auditAction,
		})
		return
	}

	txHash, broadcastErr := broadcastSigned(wallet, string(txParamsJSON), result.Signature)
	if broadcastErr != nil {
		slog.Error("approve-token broadcast failed",
			"wallet_id", wallet.ID, "chain", wallet.Chain,
			"contract", contractAddr, "spender", spenderAddr,
			"action", auditAction, "error", broadcastErr.Error(),
		)
		respondBroadcastErrorDetails(c, broadcastErr, gin.H{
			"stage": "broadcast", "wallet_id": wallet.ID, "chain": wallet.Chain,
			"contract": contractAddr, "spender": spenderAddr, "action": auditAction,
		})
		return
	}

	writeAuditCtx(h.db, c, auditAction, "success", &wallet.ID, map[string]interface{}{
		"tx_hash": txHash, "tx_context": txContext,
	})
	c.JSON(http.StatusOK, gin.H{
		"status":         "completed",
		"tx_hash":        txHash,
		"chain":          wallet.Chain,
		"from":           walletAddr,
		"contract":       contractAddr,
		"spender":        spenderAddr,
		"wallet_address": walletAddr,
	})
}

// extractMethodName returns the function name portion of a Solidity func_sig.
// e.g. "transfer(address,uint256)" → "transfer"
func extractMethodName(funcSig string) (string, error) {
	idx := strings.Index(funcSig, "(")
	if idx <= 0 {
		return "", fmt.Errorf("invalid function signature: %q", funcSig)
	}
	name := strings.TrimSpace(funcSig[:idx])
	if name == "" {
		return "", fmt.Errorf("empty method name in signature: %q", funcSig)
	}
	return name, nil
}
