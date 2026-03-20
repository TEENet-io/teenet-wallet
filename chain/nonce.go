package chain

import (
	"fmt"
	"math/big"
	"strings"
	"sync"
)

// NonceManager tracks per-address nonces to avoid concurrent nonce collisions.
// When multiple transactions are built concurrently for the same address, fetching
// the nonce from the chain for each one would return the same value, causing all
// but the first broadcast to fail. The NonceManager increments a local counter
// after each acquisition so concurrent callers get sequential nonces.
type NonceManager struct {
	mu     sync.Mutex
	nonces map[string]uint64
}

var nonceMgr = &NonceManager{nonces: make(map[string]uint64)}

// AcquireNonce returns the next nonce for the given address. On the first call
// (or after a reset) it fetches the pending nonce from the chain. Subsequent
// calls return locally-incremented values.
func (nm *NonceManager) AcquireNonce(rpcURL, address string) (uint64, error) {
	nm.mu.Lock()
	defer nm.mu.Unlock()
	if n, ok := nm.nonces[address]; ok {
		nm.nonces[address] = n + 1
		return n, nil
	}
	onChainNonce, err := fetchNonceFromChain(rpcURL, address)
	if err != nil {
		return 0, err
	}
	nm.nonces[address] = onChainNonce + 1
	return onChainNonce, nil
}

// ResetNonce removes the cached nonce for an address, forcing the next
// AcquireNonce call to re-fetch from the chain. Should be called after a
// broadcast failure.
func (nm *NonceManager) ResetNonce(address string) {
	nm.mu.Lock()
	delete(nm.nonces, address)
	nm.mu.Unlock()
}

// ResetNonce is a package-level convenience to reset the cached nonce for an address.
func ResetNonce(address string) {
	nonceMgr.ResetNonce(address)
}

// fetchNonceFromChain queries eth_getTransactionCount for the pending nonce.
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
