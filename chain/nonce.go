// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import (
	"fmt"
	"math/big"
	"strings"
)

// fetchNonceFromChain queries eth_getTransactionCount with the "pending" tag,
// which counts confirmed txs plus any currently in the RPC node's mempool.
// Callers should serialize same-address access via LockAddr so the returned
// value stays consistent across the fetch → sign → broadcast sequence.
func fetchNonceFromChain(rpcURL, address string) (uint64, error) {
	nonceRaw, err := jsonRPCWithRetry(rpcURL, map[string]interface{}{
		"jsonrpc": "2.0", "method": "eth_getTransactionCount",
		"params": []interface{}{address, "pending"}, "id": 1,
	})
	if err != nil {
		return 0, fmt.Errorf("get nonce: %w", err)
	}
	nonceHex, ok := nonceRaw["result"].(string)
	if !ok || nonceHex == "" {
		return 0, fmt.Errorf("unexpected nonce response: %v", nonceRaw["result"])
	}
	nonceInt, ok2 := new(big.Int).SetString(strings.TrimPrefix(nonceHex, "0x"), 16)
	if !ok2 {
		return 0, fmt.Errorf("invalid nonce value: %s", nonceHex)
	}
	return nonceInt.Uint64(), nil
}
