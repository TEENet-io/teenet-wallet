// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import (
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// chainID is immutable per RPC endpoint, so we cache it to avoid a redundant
// network round-trip on every transaction.
var (
	chainIDCache   = make(map[string]uint64)
	chainIDCacheMu sync.RWMutex
)

func getCachedChainID(rpcURL string) (uint64, bool) {
	chainIDCacheMu.RLock()
	defer chainIDCacheMu.RUnlock()
	id, ok := chainIDCache[rpcURL]
	return id, ok
}

func setCachedChainID(rpcURL string, id uint64) {
	chainIDCacheMu.Lock()
	defer chainIDCacheMu.Unlock()
	chainIDCache[rpcURL] = id
}

const ethDefaultGasLimit = uint64(21000)

// encodeAddrAmountCalldata is the shared body of EncodeERC20Transfer and EncodeERC20Approve.
// It builds 4-byte selector ++ ABI-encoded (address, uint256) calldata.
func encodeAddrAmountCalldata(funcSig string, targetAddr string, amount *big.Int) []byte {
	selector := crypto.Keccak256([]byte(funcSig))[:4]

	addr := common.HexToAddress(targetAddr)
	paddedAddr := make([]byte, 32)
	copy(paddedAddr[12:], addr.Bytes())

	paddedAmount := make([]byte, 32)
	amountBytes := amount.Bytes()
	copy(paddedAmount[32-len(amountBytes):], amountBytes)

	calldata := make([]byte, 0, 4+32+32)
	calldata = append(calldata, selector...)
	calldata = append(calldata, paddedAddr...)
	calldata = append(calldata, paddedAmount...)
	return calldata
}

// EncodeERC20Transfer encodes a ERC-20 transfer(address,uint256) call.
// Returns the ABI-encoded calldata without any 0x prefix.
func EncodeERC20Transfer(toAddr string, amount *big.Int) []byte {
	return encodeAddrAmountCalldata("transfer(address,uint256)", toAddr, amount)
}

// EncodeERC20Approve encodes a ERC-20 approve(address,uint256) call.
func EncodeERC20Approve(spenderAddr string, amount *big.Int) []byte {
	return encodeAddrAmountCalldata("approve(address,uint256)", spenderAddr, amount)
}

// ETHTxParams contains all fields needed to reconstruct and broadcast an ETH transaction after signing.
type ETHTxParams struct {
	Nonce                uint64 `json:"nonce"`
	MaxFeePerGas         string `json:"max_fee_per_gas"`          // Wei, decimal string
	MaxPriorityFeePerGas string `json:"max_priority_fee_per_gas"` // Wei, decimal string
	GasLimit             uint64 `json:"gas_limit"`
	ChainID              string `json:"chain_id"` // decimal string
	From                 string `json:"from"`
	To                   string `json:"to"`
	Value                string `json:"value"`          // Wei, decimal string
	Data                 string `json:"data,omitempty"` // hex-encoded calldata (optional, e.g. ERC-20)

	// Legacy field kept for backward compatibility with stored approval requests.
	GasPrice string `json:"gas_price,omitempty"`
}

// ETHTxData is returned by BuildETHTx.
type ETHTxData struct {
	Params      ETHTxParams
	SigningHash []byte // 32-byte signing hash
}

// ethChainParams holds common chain parameters fetched before building a transaction.
type ethChainParams struct {
	nonce                uint64
	maxFeePerGas         *big.Int
	maxPriorityFeePerGas *big.Int
	chainID              *big.Int
}

// fetchETHChainParams queries nonce (via NonceManager), gas fees, and chain ID.
func fetchETHChainParams(rpcURL, fromAddr string) (*ethChainParams, error) {
	nonce, err := nonceMgr.AcquireNonce(rpcURL, fromAddr)
	if err != nil {
		return nil, err
	}

	maxFee, priorityFee, chainID, err := fetchGasFeesAndChainID(rpcURL)
	if err != nil {
		nonceMgr.ResetNonce(rpcURL, fromAddr) // chain-specific reset
		return nil, err
	}

	return &ethChainParams{
		nonce:                nonce,
		maxFeePerGas:         maxFee,
		maxPriorityFeePerGas: priorityFee,
		chainID:              chainID,
	}, nil
}

// fetchETHChainParamsFresh queries nonce directly from the chain (bypassing the
// NonceManager). Used by RebuildETHTx at approval time.
func fetchETHChainParamsFresh(rpcURL, fromAddr string) (*ethChainParams, error) {
	nonce, err := fetchNonceFromChain(rpcURL, fromAddr)
	if err != nil {
		return nil, err
	}

	maxFee, priorityFee, chainID, err := fetchGasFeesAndChainID(rpcURL)
	if err != nil {
		return nil, err
	}

	return &ethChainParams{
		nonce:                nonce,
		maxFeePerGas:         maxFee,
		maxPriorityFeePerGas: priorityFee,
		chainID:              chainID,
	}, nil
}

// fetchGasFeesAndChainID queries maxPriorityFeePerGas, baseFee (via latest block), and chain ID.
// The two gas-fee RPC calls are issued concurrently. ChainID is served from an
// in-process cache (immutable per endpoint) and only fetched once per process lifetime.
func fetchGasFeesAndChainID(rpcURL string) (maxFee, priorityFee, chainID *big.Int, err error) {
	// Results from the two concurrent gas-fee calls.
	type priorityFeeResult struct {
		hex string
		err error
	}
	type baseFeeResult struct {
		hex string
		err error
	}

	priorityCh := make(chan priorityFeeResult, 1)
	baseFeeCh := make(chan baseFeeResult, 1)

	// 1a. Get max priority fee (tip) — concurrent.
	go func() {
		raw, e := jsonRPCWithRetry(rpcURL, map[string]interface{}{
			"jsonrpc": "2.0", "method": "eth_maxPriorityFeePerGas", "params": []interface{}{}, "id": 1,
		})
		if e != nil {
			priorityCh <- priorityFeeResult{err: fmt.Errorf("get max priority fee: %w", e)}
			return
		}
		h, ok := raw["result"].(string)
		if !ok || h == "" {
			priorityCh <- priorityFeeResult{err: fmt.Errorf("unexpected max priority fee response: %v", raw["result"])}
			return
		}
		priorityCh <- priorityFeeResult{hex: h}
	}()

	// 1b. Get base fee from latest block — concurrent.
	go func() {
		raw, e := jsonRPCWithRetry(rpcURL, map[string]interface{}{
			"jsonrpc": "2.0", "method": "eth_getBlockByNumber", "params": []interface{}{"latest", false}, "id": 1,
		})
		if e != nil {
			baseFeeCh <- baseFeeResult{err: fmt.Errorf("get latest block: %w", e)}
			return
		}
		blockResult, ok := raw["result"].(map[string]interface{})
		if !ok {
			baseFeeCh <- baseFeeResult{err: fmt.Errorf("unexpected block response")}
			return
		}
		h, _ := blockResult["baseFeePerGas"].(string)
		baseFeeCh <- baseFeeResult{hex: h}
	}()

	// Collect concurrent results.
	prRes := <-priorityCh
	if prRes.err != nil {
		return nil, nil, nil, prRes.err
	}
	priorityFee, ok := new(big.Int).SetString(strings.TrimPrefix(prRes.hex, "0x"), 16)
	if !ok {
		return nil, nil, nil, fmt.Errorf("invalid max priority fee: %s", prRes.hex)
	}

	bfRes := <-baseFeeCh
	if bfRes.err != nil {
		return nil, nil, nil, bfRes.err
	}
	baseFee := big.NewInt(0)
	if bfRes.hex != "" {
		baseFee, ok = new(big.Int).SetString(strings.TrimPrefix(bfRes.hex, "0x"), 16)
		if !ok {
			return nil, nil, nil, fmt.Errorf("invalid base fee: %s", bfRes.hex)
		}
	}

	// maxFeePerGas = 2 * baseFee + priorityFee (standard formula, handles base fee spikes)
	maxFee = new(big.Int).Mul(baseFee, big.NewInt(2))
	maxFee.Add(maxFee, priorityFee)

	// Ensure minimum priority fee of 1 gwei for networks with very low fees.
	minPriority := big.NewInt(1_000_000_000) // 1 gwei
	if priorityFee.Cmp(minPriority) < 0 {
		priorityFee = minPriority
		// Recalculate maxFee with the minimum priority.
		maxFee = new(big.Int).Mul(baseFee, big.NewInt(2))
		maxFee.Add(maxFee, priorityFee)
	}

	// 2. Get chain ID — served from cache when available.
	var chainIDUint64 uint64
	if cached, hit := getCachedChainID(rpcURL); hit {
		chainIDUint64 = cached
	} else {
		chainIDRaw, e := jsonRPCWithRetry(rpcURL, map[string]interface{}{
			"jsonrpc": "2.0", "method": "eth_chainId", "params": []interface{}{}, "id": 1,
		})
		if e != nil {
			return nil, nil, nil, fmt.Errorf("get chain id: %w", e)
		}
		chainIDHex, ok2 := chainIDRaw["result"].(string)
		if !ok2 || chainIDHex == "" {
			return nil, nil, nil, fmt.Errorf("unexpected chain id response: %v", chainIDRaw["result"])
		}
		parsed, ok3 := new(big.Int).SetString(strings.TrimPrefix(chainIDHex, "0x"), 16)
		if !ok3 {
			return nil, nil, nil, fmt.Errorf("invalid chain id: %s", chainIDHex)
		}
		chainIDUint64 = parsed.Uint64()
		setCachedChainID(rpcURL, chainIDUint64)
	}
	chainID = new(big.Int).SetUint64(chainIDUint64)

	return maxFee, priorityFee, chainID, nil
}

// buildDynamicTx creates an EIP-1559 DynamicFeeTx.
func buildDynamicTx(cp *ethChainParams, to common.Address, value *big.Int, gasLimit uint64, data []byte) *types.Transaction {
	return types.NewTx(&types.DynamicFeeTx{
		ChainID:   cp.chainID,
		Nonce:     cp.nonce,
		GasTipCap: cp.maxPriorityFeePerGas,
		GasFeeCap: cp.maxFeePerGas,
		Gas:       gasLimit,
		To:        &to,
		Value:     value,
		Data:      data,
	})
}

// BuildETHTx queries the chain and constructs an EIP-1559 ETH transfer transaction.
func BuildETHTx(rpcURL, fromAddr, toAddr string, amountETH *big.Float) (*ETHTxData, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("ETH_RPC_URL is not configured")
	}

	// Convert ETH → Wei.
	e18 := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	weiF := new(big.Float).SetPrec(256).Mul(amountETH, new(big.Float).SetInt(e18))
	wei, _ := weiF.Int(nil)
	if wei.Sign() <= 0 {
		return nil, fmt.Errorf("amount must be positive")
	}

	cp, err := fetchETHChainParams(rpcURL, fromAddr)
	if err != nil {
		return nil, err
	}

	toAddress := common.HexToAddress(toAddr)
	tx := buildDynamicTx(cp, toAddress, wei, ethDefaultGasLimit, nil)

	signer := types.LatestSignerForChainID(cp.chainID)
	sigHash := signer.Hash(tx)

	return &ETHTxData{
		Params: ETHTxParams{
			Nonce:                cp.nonce,
			MaxFeePerGas:         cp.maxFeePerGas.String(),
			MaxPriorityFeePerGas: cp.maxPriorityFeePerGas.String(),
			GasLimit:             ethDefaultGasLimit,
			ChainID:              cp.chainID.String(),
			From:                 fromAddr,
			To:                   toAddr,
			Value:                wei.String(),
		},
		SigningHash: sigHash[:],
	}, nil
}

// RebuildETHTx refreshes the nonce and gas fees of a previously-built ETH transaction.
// Used at approval time to avoid stale-nonce broadcast failures.
func RebuildETHTx(rpcURL string, params ETHTxParams) (*ETHTxData, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("ETH_RPC_URL is not configured")
	}
	cp, err := fetchETHChainParamsFresh(rpcURL, params.From)
	if err != nil {
		return nil, fmt.Errorf("refresh chain params: %w", err)
	}

	value, ok := new(big.Int).SetString(params.Value, 10)
	if !ok {
		return nil, fmt.Errorf("invalid value: %s", params.Value)
	}

	var txData []byte
	if params.Data != "" {
		txData, _ = hex.DecodeString(strings.TrimPrefix(params.Data, "0x"))
	}

	toAddr := common.HexToAddress(params.To)
	tx := buildDynamicTx(cp, toAddr, value, params.GasLimit, txData)

	signer := types.LatestSignerForChainID(cp.chainID)
	sigHash := signer.Hash(tx)

	return &ETHTxData{
		Params: ETHTxParams{
			Nonce:                cp.nonce,
			MaxFeePerGas:         cp.maxFeePerGas.String(),
			MaxPriorityFeePerGas: cp.maxPriorityFeePerGas.String(),
			GasLimit:             params.GasLimit,
			ChainID:              cp.chainID.String(),
			From:                 params.From,
			To:                   params.To,
			Value:                params.Value,
			Data:                 params.Data,
		},
		SigningHash: sigHash[:],
	}, nil
}

// AssembleAndBroadcastETH applies a TEE ECDSA signature to an EIP-1559 transaction and broadcasts it.
func AssembleAndBroadcastETH(rpcURL string, params ETHTxParams, sig []byte, fromAddr string) (string, error) {
	if len(sig) < 64 {
		return "", fmt.Errorf("signature too short: %d bytes (need 64)", len(sig))
	}

	chainID, ok := new(big.Int).SetString(params.ChainID, 10)
	if !ok {
		return "", fmt.Errorf("invalid chain id: %s", params.ChainID)
	}
	value, ok := new(big.Int).SetString(params.Value, 10)
	if !ok {
		return "", fmt.Errorf("invalid value: %s", params.Value)
	}

	// Parse gas fee fields. Support both EIP-1559 and legacy params (for old stored approvals).
	var maxFee, priorityFee *big.Int
	if params.MaxFeePerGas != "" {
		maxFee, ok = new(big.Int).SetString(params.MaxFeePerGas, 10)
		if !ok {
			return "", fmt.Errorf("invalid max fee per gas: %s", params.MaxFeePerGas)
		}
		priorityFee, ok = new(big.Int).SetString(params.MaxPriorityFeePerGas, 10)
		if !ok {
			return "", fmt.Errorf("invalid max priority fee: %s", params.MaxPriorityFeePerGas)
		}
	} else if params.GasPrice != "" {
		// Legacy fallback: use gasPrice as both maxFee and priorityFee.
		gasPrice, ok2 := new(big.Int).SetString(params.GasPrice, 10)
		if !ok2 {
			return "", fmt.Errorf("invalid gas price: %s", params.GasPrice)
		}
		maxFee = gasPrice
		priorityFee = gasPrice
	} else {
		return "", fmt.Errorf("no gas fee parameters provided")
	}

	toAddr := common.HexToAddress(params.To)
	fromAddress := common.HexToAddress(fromAddr)

	var txData []byte
	if params.Data != "" {
		var decodeErr error
		txData, decodeErr = hex.DecodeString(strings.TrimPrefix(params.Data, "0x"))
		if decodeErr != nil {
			return "", fmt.Errorf("invalid calldata hex in tx params: %w", decodeErr)
		}
	}

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     params.Nonce,
		GasTipCap: priorityFee,
		GasFeeCap: maxFee,
		Gas:       params.GasLimit,
		To:        &toAddr,
		Value:     value,
		Data:      txData,
	})

	signer := types.LatestSignerForChainID(chainID)
	sigHash := signer.Hash(tx)

	// Try both recovery values to find the one matching fromAddr.
	var signedTx *types.Transaction
	for v := byte(0); v <= 1; v++ {
		sig65 := make([]byte, 65)
		copy(sig65, sig[:64])
		sig65[64] = v
		pub, err := crypto.SigToPub(sigHash[:], sig65)
		if err != nil {
			continue
		}
		if crypto.PubkeyToAddress(*pub) == fromAddress {
			var signErr error
			signedTx, signErr = tx.WithSignature(signer, sig65)
			if signErr != nil {
				return "", fmt.Errorf("apply signature: %w", signErr)
			}
			break
		}
	}
	if signedTx == nil {
		return "", fmt.Errorf("could not determine valid recovery id for address %s", fromAddr)
	}

	rawBytes, err := signedTx.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal tx: %w", err)
	}

	result, err := jsonRPC(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_sendRawTransaction",
		"params":  []interface{}{"0x" + hex.EncodeToString(rawBytes)},
		"id":      1,
	})
	if err != nil {
		return "", fmt.Errorf("broadcast: %w", err)
	}
	txHash, _ := result["result"].(string)
	return txHash, nil
}

// BuildETHContractCallTx builds a contract call transaction (e.g. ERC-20 transfer).
// Gas limit is estimated via eth_estimateGas with a 20% buffer.
func BuildETHContractCallTx(rpcURL, fromAddr, contractAddr string, callData []byte, value *big.Int) (*ETHTxData, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("ETH_RPC_URL is not configured")
	}
	if value == nil {
		value = big.NewInt(0)
	}

	cp, err := fetchETHChainParams(rpcURL, fromAddr)
	if err != nil {
		return nil, err
	}

	// Estimate gas via eth_estimateGas.
	estimateRaw, err := jsonRPCWithRetry(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "method": "eth_estimateGas",
		"params": []interface{}{map[string]interface{}{
			"from":  fromAddr,
			"to":    contractAddr,
			"value": "0x" + value.Text(16),
			"data":  "0x" + hex.EncodeToString(callData),
		}},
		"id": 1,
	})
	if err != nil {
		selector := ""
		if len(callData) >= 4 {
			selector = "0x" + hex.EncodeToString(callData[:4])
		}
		slog.Error("eth_estimateGas failed for contract call",
			"from", fromAddr,
			"to", contractAddr,
			"value_wei", value.String(),
			"selector", selector,
			"calldata_len", len(callData),
			"error", err.Error(),
		)
		return nil, fmt.Errorf("estimate gas reverted or failed: %w", err)
	}
	estimateHex, ok := estimateRaw["result"].(string)
	if !ok || estimateHex == "" {
		return nil, fmt.Errorf("unexpected gas estimate response: %v", estimateRaw["result"])
	}
	estimatedGas, ok2 := new(big.Int).SetString(strings.TrimPrefix(estimateHex, "0x"), 16)
	if !ok2 {
		return nil, fmt.Errorf("invalid gas estimate value: %s", estimateHex)
	}
	// Add 20% buffer.
	gasLimit := new(big.Int).Mul(estimatedGas, big.NewInt(120))
	gasLimit.Div(gasLimit, big.NewInt(100))

	toAddress := common.HexToAddress(contractAddr)
	tx := buildDynamicTx(cp, toAddress, value, gasLimit.Uint64(), callData)

	signer := types.LatestSignerForChainID(cp.chainID)
	sigHash := signer.Hash(tx)

	return &ETHTxData{
		Params: ETHTxParams{
			Nonce:                cp.nonce,
			MaxFeePerGas:         cp.maxFeePerGas.String(),
			MaxPriorityFeePerGas: cp.maxPriorityFeePerGas.String(),
			GasLimit:             gasLimit.Uint64(),
			ChainID:              cp.chainID.String(),
			From:                 fromAddr,
			To:                   contractAddr,
			Value:                value.String(),
			Data:                 hex.EncodeToString(callData),
		},
		SigningHash: sigHash[:],
	}, nil
}
