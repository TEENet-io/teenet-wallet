// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

var rpcTimeout = 15 * time.Second

func init() {
	if v := os.Getenv("RPC_TIMEOUT_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rpcTimeout = time.Duration(n) * time.Second
		}
	}
}

var httpClient = &http.Client{
	Timeout: rpcTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// BalanceResult holds a balance query response.
type BalanceResult struct {
	Chain    string `json:"chain"`
	Address  string `json:"address"`
	Balance  string `json:"balance"`  // human-readable, e.g. "1.234"
	Currency string `json:"currency"` // "ETH", "SOL"
	Raw      string `json:"raw"`      // raw smallest unit value
}

// GetBalance queries the on-chain balance for the given chain and address.
func GetBalance(family, address, rpcURL, chainName, currency string) (*BalanceResult, error) {
	switch family {
	case "evm":
		return getETHBalance(address, rpcURL, chainName, currency)
	case "solana":
		return getSOLBalance(address, rpcURL, chainName, currency)
	default:
		return nil, fmt.Errorf("unsupported chain family: %s", family)
	}
}

func getETHBalance(address, rpcURL, chainName, currency string) (*BalanceResult, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("ETH_RPC_URL is not configured")
	}
	body := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_getBalance",
		"params":  []interface{}{address, "latest"},
		"id":      1,
	}
	raw, err := jsonRPCWithRetry(rpcURL, body)
	if err != nil {
		return nil, err
	}
	hexVal, ok := raw["result"].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected eth_getBalance response: %v", raw["result"])
	}
	hexVal = strings.TrimPrefix(hexVal, "0x")
	wei := new(big.Int)
	wei.SetString(hexVal, 16)

	// Convert Wei → ETH (divide by 1e18)
	eth := new(big.Float).Quo(new(big.Float).SetInt(wei), big.NewFloat(1e18))
	return &BalanceResult{
		Chain:    chainName,
		Address:  address,
		Balance:  eth.Text('f', 8),
		Currency: currency,
		Raw:      wei.String(),
	}, nil
}

func getSOLBalance(address, rpcURL, chainName, currency string) (*BalanceResult, error) {
	if rpcURL == "" {
		rpcURL = "https://api.mainnet-beta.solana.com"
	}
	raw, err := jsonRPCWithRetry(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "getBalance",
		"params":  []interface{}{address},
		"id":      1,
	})
	if err != nil {
		return nil, err
	}

	resultMap, ok := raw["result"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected getBalance response")
	}
	valueRaw := resultMap["value"]
	var lamports int64
	switch v := valueRaw.(type) {
	case float64:
		lamports = int64(v)
	case json.Number:
		var numErr error
		lamports, numErr = v.Int64()
		if numErr != nil {
			return nil, fmt.Errorf("invalid lamport value: %w", numErr)
		}
	default:
		return nil, fmt.Errorf("unexpected lamport value type: %T", valueRaw)
	}
	solBig := new(big.Float).Quo(new(big.Float).SetInt64(lamports), big.NewFloat(1e9))
	return &BalanceResult{
		Chain:    chainName,
		Address:  address,
		Balance:  solBig.Text('f', 9),
		Currency: currency,
		Raw:      fmt.Sprintf("%d", lamports),
	}, nil
}

// ETHCall performs a read-only eth_call against a contract.
// Returns the raw ABI-encoded return data.
// This does NOT create a transaction or cost gas.
func ETHCall(rpcURL, fromAddr, toAddr string, calldata []byte) ([]byte, error) {
	if rpcURL == "" {
		return nil, fmt.Errorf("RPC URL is not configured")
	}
	calldataHex := "0x" + hex.EncodeToString(calldata)
	callObj := map[string]interface{}{
		"to":   toAddr,
		"data": calldataHex,
	}
	if fromAddr != "" {
		callObj["from"] = fromAddr
	}
	result, err := jsonRPCWithRetry(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "eth_call",
		"params":  []interface{}{callObj, "latest"},
		"id":      1,
	})
	if err != nil {
		return nil, fmt.Errorf("eth_call: %w", err)
	}
	resultHex, ok := result["result"].(string)
	if !ok {
		return nil, fmt.Errorf("unexpected eth_call response: %v", result["result"])
	}
	return hex.DecodeString(strings.TrimPrefix(resultHex, "0x"))
}

// jsonRPCWithRetry wraps jsonRPC with up to 3 attempts and exponential backoff.
// Suitable for read-only queries (balance, chain params) where retrying is safe.
func jsonRPCWithRetry(url string, payload interface{}) (map[string]interface{}, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		body, err := jsonRPC(url, payload)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}
	return nil, fmt.Errorf("RPC failed after 3 attempts: %w", lastErr)
}

func jsonRPC(url string, payload interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Post(url, "application/json", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("rpc request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("rpc http error: status %d", resp.StatusCode)
	}

	var result map[string]interface{}
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	if err := dec.Decode(&result); err != nil {
		return nil, fmt.Errorf("rpc decode failed: %w", err)
	}
	if errField, ok := result["error"]; ok && errField != nil {
		return nil, fmt.Errorf("rpc error: %v", errField)
	}
	return result, nil
}
