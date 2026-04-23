// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import "sync"

// addrLocks holds a per-(rpcURL, address) *sync.Mutex created lazily via
// LoadOrStore. Entries are never evicted — the number of distinct tuples is
// bounded by the configured wallet/user caps, and each *sync.Mutex is tiny.
var addrLocks sync.Map

// LockAddr acquires an in-process mutex scoped to (rpcURL, address) and
// returns a function that releases it. Callers should defer the returned
// function.
//
// Use this to serialize concurrent transfer / contract-call flows that build
// and broadcast EVM transactions for the same address. Holding the mutex
// across fetch-nonce → sign → broadcast ensures eth_getTransactionCount("pending")
// observes a consistent view so two goroutines cannot pick the same nonce.
//
// The mutex is NOT reentrant. Nested acquisition on the same (rpcURL, address)
// from a single goroutine will deadlock.
func LockAddr(rpcURL, address string) func() {
	key := rpcURL + ":" + address
	v, _ := addrLocks.LoadOrStore(key, &sync.Mutex{})
	m := v.(*sync.Mutex)
	m.Lock()
	return m.Unlock
}
