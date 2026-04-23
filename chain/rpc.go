// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
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

// rpcAppError wraps JSON-RPC application errors (the "error" field in the
// response body — e.g. "execution reverted", "nonce too low"). These are
// deterministic and any fallback RPC would return the same thing, so the
// fallback path is NOT triggered for them. Transport / HTTP errors use plain
// error values and DO trigger fallback.
type rpcAppError struct{ msg string }

func (e *rpcAppError) Error() string { return e.msg }

// rpcFallbacks maps a primary RPC URL → fallback URL. Populated at startup
// in main.go when QuickNode overrides a chain's original RPC — the publicnode
// URL is registered here so transient QuickNode transport failures transparently
// fall back. Unset chains behave as before (single URL, no fallback).
var rpcFallbacks sync.Map // map[string]string

// SetRPCFallback registers fallback for primary. Idempotent; empty or identical
// pairs are ignored. Call once per chain at startup.
func SetRPCFallback(primary, fallback string) {
	if primary == "" || fallback == "" || primary == fallback {
		return
	}
	rpcFallbacks.Store(primary, fallback)
}

// ClearRPCFallbacks removes all registered fallbacks. Test helper.
func ClearRPCFallbacks() {
	rpcFallbacks.Range(func(k, _ any) bool { rpcFallbacks.Delete(k); return true })
}

// jsonRPCWithRetry runs up to 3 attempts (exponential backoff) against the
// primary URL. On transport/HTTP failure, if a fallback is registered it runs
// another 3 attempts against the fallback before giving up. JSON-RPC application
// errors short-circuit immediately and do not trigger the fallback.
func jsonRPCWithRetry(primary string, payload interface{}) (map[string]interface{}, error) {
	body, err := retryJSONRPC(primary, payload)
	if err == nil {
		return body, nil
	}
	if isAppError(err) {
		return nil, fmt.Errorf("RPC failed: %w", err)
	}
	fbRaw, ok := rpcFallbacks.Load(primary)
	if !ok {
		return nil, fmt.Errorf("RPC failed after 3 attempts: %w", err)
	}
	fb := fbRaw.(string)
	slog.Warn("primary RPC failed, falling back",
		"primary", HostOnly(primary), "fallback", HostOnly(fb), "error", err)
	body, err2 := retryJSONRPC(fb, payload)
	if err2 == nil {
		return body, nil
	}
	return nil, fmt.Errorf("RPC failed on primary (%s: %v) and fallback (%s): %w",
		HostOnly(primary), err, HostOnly(fb), err2)
}

// retryJSONRPC does up to 3 attempts with exponential backoff against a single URL.
// Application errors short-circuit (retrying won't help).
func retryJSONRPC(url string, payload interface{}) (map[string]interface{}, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		body, err := jsonRPC(url, payload)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if isAppError(err) {
			return nil, err
		}
		if attempt < 2 {
			time.Sleep(time.Duration(attempt+1) * time.Second)
		}
	}
	return nil, lastErr
}

func isAppError(err error) bool {
	var e *rpcAppError
	return errors.As(err, &e)
}

// HostOnly extracts the hostname for log safety — RPC URLs include the
// QuickNode token in the path, so full URLs must not be logged. Exported
// for use by other packages that log RPC-related errors.
func HostOnly(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Host == "" {
		return "<invalid-url>"
	}
	return u.Host
}

func jsonRPC(endpoint string, payload interface{}) (map[string]interface{}, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	resp, err := httpClient.Post(endpoint, "application/json", bytes.NewReader(b))
	if err != nil {
		// Go's net/http returns *url.Error on transport failures, whose
		// Error() embeds the full URL — including any provider token in its
		// path (e.g. QuickNode). Strip the URL here so the token can't leak
		// through logs, traces, or HTTP responses further up the stack.
		// HostOnly preserves just the hostname for diagnostic context.
		var ue *url.Error
		if errors.As(err, &ue) {
			return nil, fmt.Errorf("rpc request to %s failed: %w", HostOnly(ue.URL), ue.Err)
		}
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
		return nil, &rpcAppError{msg: fmt.Sprintf("rpc error: %v", errField)}
	}
	return result, nil
}
