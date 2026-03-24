package handler

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	sdk "github.com/TEENet-io/teenet-sdk/go"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/chain"
	"github.com/TEENet-io/teenet-wallet/model"
)

// highRiskMethods always require Passkey approval, even when AutoApprove=true on the contract.
var highRiskMethods = map[string]bool{
	"approve":           true,
	"increaseallowance": true,
	"setapprovalforall": true,
	"transferfrom":      true,
	"safetransferfrom":  true,
}

// highRiskSOLDiscriminators maps first-byte hex discriminators for Solana
// SPL Token instructions that are always high-risk (require passkey approval).
var highRiskSOLDiscriminators = map[string]bool{
	"04": true, // Approve
	"06": true, // SetAuthority
	"07": true, // MintTo
	"09": true, // CloseAccount
}

// ContractCallHandler handles general-purpose smart contract calls.
type ContractCallHandler struct {
	db             *gorm.DB
	sdk            *sdk.Client
	baseURL        string
	approvalExpiry time.Duration
	idempotency    *IdempotencyStore
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
	AmountUSD string                 `json:"amount_usd"`                  // optional: caller-reported USD value for threshold/daily-limit
	Memo      string                 `json:"memo"`
	Accounts  []chain.SOLAccountMeta `json:"accounts"`                    // Solana only
	Data      string                 `json:"data"`                        // Solana only: hex instruction data
}

// ContractCall executes a smart contract call with three-layer security:
//  1. Contract address whitelist
//  2. Per-contract method restriction
//  3. Approval policy / high-risk method gate
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

	chainCfg, cfgOk := model.Chains[wallet.Chain]
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

// contractCallEVM implements contract call logic for EVM chains.
func (h *ContractCallHandler) contractCallEVM(c *gin.Context, wallet model.Wallet, chainCfg model.ChainConfig) {
	var req ContractCallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}

	if req.FuncSig == "" {
		jsonError(c, http.StatusBadRequest, "func_sig is required for EVM contract calls")
		return
	}

	// Normalize contract address.
	contractAddr, addrErr := normalizeEVMAddress(req.Contract)
	if addrErr != nil {
		jsonError(c, http.StatusBadRequest, "contract: "+addrErr.Error())
		return
	}

	// Layer 1: Whitelist check.
	var allowed model.AllowedContract
	if err := h.db.Where("wallet_id = ? AND contract_address = ?", wallet.ID, contractAddr).First(&allowed).Error; err != nil {
		jsonError(c, http.StatusForbidden, "contract not whitelisted: "+contractAddr)
		return
	}

	// Extract method name from func_sig (everything before the first "(").
	methodName, err := extractMethodName(req.FuncSig)
	if err != nil {
		jsonError(c, http.StatusBadRequest, err.Error())
		return
	}
	methodNameLower := strings.ToLower(methodName)

	// Layer 2: Method restriction — if AllowedMethods is set, method must be in the list.
	if allowed.AllowedMethods != "" {
		methodAllowed := false
		for _, m := range strings.Split(allowed.AllowedMethods, ",") {
			if strings.TrimSpace(strings.ToLower(m)) == methodNameLower {
				methodAllowed = true
				break
			}
		}
		if !methodAllowed {
			jsonError(c, http.StatusForbidden, fmt.Sprintf("method %q is not in the allowed methods list for this contract", methodName))
			return
		}
	}

	// Encode calldata (validation only — no RPC yet).
	calldata, encErr := chain.EncodeCall(req.FuncSig, req.Args)
	if encErr != nil {
		jsonError(c, http.StatusBadRequest, "encode calldata: "+encErr.Error())
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
		jsonError(c, http.StatusBadGateway, "build contract tx: "+buildErr.Error())
		return
	}

	txParamsJSON, marshalErr := json.Marshal(txData.Params)
	if marshalErr != nil {
		jsonError(c, http.StatusInternalServerError, "marshal tx params failed")
		return
	}

	signingMsg := txData.SigningHash

	// Layer 3: Security decision.
	// Passkey auth: human is already present — proceed directly.
	// API Key auth: check high-risk methods and AutoApprove flag.
	needsApproval := false
	var approvalReason string

	if !isPasskeyAuth(c) {
		// API Key path.
		if highRiskMethods[methodNameLower] {
			needsApproval = true
			approvalReason = fmt.Sprintf("method %q is high-risk and requires passkey approval", methodName)
		} else if !allowed.AutoApprove {
			needsApproval = true
			approvalReason = "contract does not have auto-approve enabled; passkey approval required"
		}
	}

	// Compute effective USD amount for threshold / daily limit checks.
	// Sources: (1) native value × price, (2) caller-reported amount_usd. Use the larger value.
	var effectiveUSD float64
	if h.prices != nil {
		if valueWei != nil && valueWei.Sign() > 0 && req.Value != "" {
			currency := chainCfg.Currency
			if usdPrice, priceErr := h.prices.GetUSDPrice(currency); priceErr == nil && usdPrice > 0 {
				if a, ok := new(big.Float).SetString(req.Value); ok {
					f, _ := a.Float64()
					effectiveUSD = f * usdPrice
				}
			}
		}
		if req.AmountUSD != "" {
			if reported, ok := new(big.Float).SetString(req.AmountUSD); ok && reported.Sign() > 0 {
				f, _ := reported.Float64()
				if f > effectiveUSD {
					effectiveUSD = f
				}
			}
		}
	}

	// Check USD approval policy (threshold + daily limit).
	var policy model.ApprovalPolicy
	policyFound := h.db.Where("wallet_id = ? AND enabled = ?", wallet.ID, true).First(&policy).Error == nil

	var deductedUSDStr string // non-empty if we pre-deducted from daily limit
	if !needsApproval && policyFound && effectiveUSD > 0 {
		// Daily limit check (pre-deduct, rollback on failure).
		if policy.DailyLimitUSD != "" {
			deductedUSDStr = new(big.Float).SetFloat64(effectiveUSD).Text('f', 2)
			exceeded, msg, err := checkAndDeductDailyLimitUSD(h.db, wallet.ID, deductedUSDStr)
			if err != nil {
				slog.Error("daily limit check error", "wallet_id", wallet.ID, "error", err)
				jsonError(c, http.StatusInternalServerError, "failed to check daily limit")
				return
			}
			if exceeded {
				deductedUSDStr = ""
				jsonError(c, http.StatusBadRequest, msg)
				return
			}
		}
		// Threshold check.
		if exceedsUSDThreshold(effectiveUSD, policy.ThresholdUSD) {
			needsApproval = true
			approvalReason = fmt.Sprintf("amount ~$%.2f USD exceeds approval threshold $%s USD", effectiveUSD, policy.ThresholdUSD)
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
		// Approval path: rollback pre-deduction — addDailySpentUSD handles it on approve+broadcast.
		if deductedUSDStr != "" {
			releaseDailySpentUSD(h.db, wallet.ID, deductedUSDStr)
		}
		txContextJSON, _ := json.Marshal(txContext)

		userID := mustUserID(c)
		if c.IsAborted() {
			return
		}
		approval := model.ApprovalRequest{
			WalletID:     wallet.ID,
			UserID:       userID,
			ApprovalType: "contract_call",
			Message:      hex.EncodeToString(signingMsg),
			TxContext:    string(txContextJSON),
			TxParams:     string(txParamsJSON),
			Status:       "pending",
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(h.approvalExpiry),
		}
		if err := h.db.Create(&approval).Error; err != nil {
			jsonError(c, http.StatusInternalServerError, "create approval request failed")
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
		if deductedUSDStr != "" {
			releaseDailySpentUSD(h.db, wallet.ID, deductedUSDStr)
		}
		errMsg := "signing failed"
		if signErr != nil {
			errMsg = signErr.Error()
		} else if result != nil {
			errMsg = result.Error
		}
		jsonError(c, http.StatusBadGateway, "signing failed: "+errMsg)
		return
	}

	txHash, broadcastErr := broadcastSigned(wallet, string(txParamsJSON), result.Signature)
	if broadcastErr != nil {
		if deductedUSDStr != "" {
			releaseDailySpentUSD(h.db, wallet.ID, deductedUSDStr)
		}
		respondBroadcastError(c, broadcastErr)
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
	if err := h.db.Where("wallet_id = ? AND contract_address = ?", wallet.ID, programID).First(&allowed).Error; err != nil {
		jsonError(c, http.StatusForbidden, "program not whitelisted: "+programID)
		return
	}

	// Decode instruction data
	instrData, decErr := hex.DecodeString(strings.TrimPrefix(req.Data, "0x"))
	if decErr != nil {
		jsonError(c, http.StatusBadRequest, "data: invalid hex")
		return
	}

	// Layer 2: Discriminator restriction (first byte for SPL Token instructions)
	discriminator := ""
	if len(instrData) >= 1 {
		discriminator = hex.EncodeToString(instrData[:1])
	}
	if allowed.AllowedMethods != "" && discriminator != "" {
		methodAllowed := false
		for _, m := range strings.Split(allowed.AllowedMethods, ",") {
			if strings.TrimSpace(strings.ToLower(m)) == strings.ToLower(discriminator) {
				methodAllowed = true
				break
			}
		}
		if !methodAllowed {
			jsonError(c, http.StatusForbidden, fmt.Sprintf("instruction discriminator %q not in allowed list", discriminator))
			return
		}
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
		jsonError(c, http.StatusBadGateway, "build program call tx: "+buildErr.Error())
		return
	}
	txParamsJSON, _ := json.Marshal(txData.Params)
	signingMsg := txData.MessageBytes

	// Layer 3: Security decision
	needsApproval := false
	var approvalReason string
	if !isPasskeyAuth(c) {
		if highRiskSOLDiscriminators[discriminator] {
			needsApproval = true
			approvalReason = fmt.Sprintf("instruction discriminator %q is high-risk", discriminator)
		} else if !allowed.AutoApprove {
			needsApproval = true
			approvalReason = "program does not have auto-approve enabled"
		}
	}

	// Caller-reported USD amount for threshold / daily limit.
	var effectiveUSD float64
	if h.prices != nil && req.AmountUSD != "" {
		if reported, ok := new(big.Float).SetString(req.AmountUSD); ok && reported.Sign() > 0 {
			effectiveUSD, _ = reported.Float64()
		}
	}

	var policy model.ApprovalPolicy
	policyFound := h.db.Where("wallet_id = ? AND enabled = ?", wallet.ID, true).First(&policy).Error == nil

	var deductedUSDStr string
	if !needsApproval && policyFound && effectiveUSD > 0 {
		if policy.DailyLimitUSD != "" {
			deductedUSDStr = new(big.Float).SetFloat64(effectiveUSD).Text('f', 2)
			exceeded, msg, err := checkAndDeductDailyLimitUSD(h.db, wallet.ID, deductedUSDStr)
			if err != nil {
				slog.Error("daily limit check error", "wallet_id", wallet.ID, "error", err)
				jsonError(c, http.StatusInternalServerError, "failed to check daily limit")
				return
			}
			if exceeded {
				deductedUSDStr = ""
				jsonError(c, http.StatusBadRequest, msg)
				return
			}
		}
		if exceedsUSDThreshold(effectiveUSD, policy.ThresholdUSD) {
			needsApproval = true
			approvalReason = fmt.Sprintf("amount ~$%.2f USD exceeds approval threshold $%s USD", effectiveUSD, policy.ThresholdUSD)
		}
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
	if effectiveUSD > 0 {
		txContext["amount_usd"] = fmt.Sprintf("%.2f", effectiveUSD)
	}

	if needsApproval {
		if deductedUSDStr != "" {
			releaseDailySpentUSD(h.db, wallet.ID, deductedUSDStr)
		}
		txContextJSON, _ := json.Marshal(txContext)
		userID := mustUserID(c)
		if c.IsAborted() {
			return
		}
		approval := model.ApprovalRequest{
			WalletID:     wallet.ID,
			UserID:       userID,
			ApprovalType: "contract_call",
			Message:      hex.EncodeToString(signingMsg),
			TxContext:    string(txContextJSON),
			TxParams:     string(txParamsJSON),
			Status:       "pending",
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(h.approvalExpiry),
		}
		if err := h.db.Create(&approval).Error; err != nil {
			jsonError(c, http.StatusInternalServerError, "create approval request failed")
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
		if deductedUSDStr != "" {
			releaseDailySpentUSD(h.db, wallet.ID, deductedUSDStr)
		}
		errMsg := "signing failed"
		if signErr != nil {
			errMsg = signErr.Error()
		} else if result != nil {
			errMsg = result.Error
		}
		jsonError(c, http.StatusBadGateway, "signing failed: "+errMsg)
		return
	}

	txHash, broadcastErr := chain.AssembleAndBroadcastSOLProgram(chainCfg.RPCURL, txData.Params, result.Signature)
	if broadcastErr != nil {
		if deductedUSDStr != "" {
			releaseDailySpentUSD(h.db, wallet.ID, deductedUSDStr)
		}
		jsonError(c, http.StatusBadGateway, "broadcast failed: "+broadcastErr.Error())
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
// Approve is always treated as high-risk — API Key auth always gets a 202 pending approval.
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
// Always treated as high-risk — API Key auth always gets a 202 pending approval.
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

	chainCfg, cfgOk := model.Chains[wallet.Chain]
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
	if err := h.db.Where("wallet_id = ? AND contract_address = ?", wallet.ID, contractAddr).First(&allowed).Error; err != nil {
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
		jsonError(c, http.StatusBadGateway, "build approve tx: "+buildErr.Error())
		return
	}

	txParamsJSON, marshalErr := json.Marshal(txData.Params)
	if marshalErr != nil {
		jsonError(c, http.StatusInternalServerError, "marshal tx params failed")
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

	// Approve is ALWAYS high-risk for API Key — no AutoApprove check needed.
	if !isPasskeyAuth(c) {
		txContextJSON, _ := json.Marshal(txContext)
		userID := mustUserID(c)
		if c.IsAborted() {
			return
		}
		approval := model.ApprovalRequest{
			WalletID:     wallet.ID,
			UserID:       userID,
			ApprovalType: "contract_call",
			Message:      hex.EncodeToString(signingMsg),
			TxContext:    string(txContextJSON),
			TxParams:     string(txParamsJSON),
			Status:       "pending",
			CreatedAt:    time.Now(),
			ExpiresAt:    time.Now().Add(h.approvalExpiry),
		}
		if err := h.db.Create(&approval).Error; err != nil {
			jsonError(c, http.StatusInternalServerError, "create approval request failed")
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
		jsonError(c, http.StatusBadGateway, "signing failed: "+errMsg)
		return
	}

	txHash, broadcastErr := broadcastSigned(wallet, string(txParamsJSON), result.Signature)
	if broadcastErr != nil {
		respondBroadcastError(c, broadcastErr)
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
