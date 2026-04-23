// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestLockAddr_SameKeySerializes verifies concurrent callers on the same
// (rpcURL, address) do NOT overlap inside the critical section.
func TestLockAddr_SameKeySerializes(t *testing.T) {
	const rpcURL = "http://test/locktest1"
	const addr = "0xA"

	var active int32 // number of goroutines inside the critical section
	var maxActive int32
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			unlock := LockAddr(rpcURL, addr)
			defer unlock()

			cur := atomic.AddInt32(&active, 1)
			for {
				m := atomic.LoadInt32(&maxActive)
				if cur <= m || atomic.CompareAndSwapInt32(&maxActive, m, cur) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond) // hold lock
			atomic.AddInt32(&active, -1)
		}()
	}
	wg.Wait()
	if got := atomic.LoadInt32(&maxActive); got != 1 {
		t.Fatalf("LockAddr(same key) must serialize; saw %d concurrent holders", got)
	}
}

// TestLockAddr_DifferentKeysDoNotBlock verifies that locks on different
// addresses are independent — one goroutine holding addr A does not block
// another goroutine acquiring addr B.
func TestLockAddr_DifferentKeysDoNotBlock(t *testing.T) {
	const rpcURL = "http://test/locktest2"

	holdA := make(chan struct{})
	releaseA := make(chan struct{})
	doneB := make(chan struct{})

	go func() {
		unlock := LockAddr(rpcURL, "0xA")
		close(holdA)
		<-releaseA
		unlock()
	}()
	<-holdA // ensure A is held before B tries to lock

	go func() {
		unlock := LockAddr(rpcURL, "0xB")
		defer unlock()
		close(doneB)
	}()

	select {
	case <-doneB:
		// Expected: B proceeds while A is held.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("LockAddr(different keys) should not block; B blocked by A")
	}
	close(releaseA)
}

// TestLockAddr_ReleaseAllowsNext verifies that after the first holder
// releases, the next waiter makes progress.
func TestLockAddr_ReleaseAllowsNext(t *testing.T) {
	const rpcURL = "http://test/locktest3"
	const addr = "0xC"

	unlock1 := LockAddr(rpcURL, addr)

	gotLock := make(chan struct{})
	go func() {
		unlock2 := LockAddr(rpcURL, addr)
		defer unlock2()
		close(gotLock)
	}()

	select {
	case <-gotLock:
		t.Fatal("second LockAddr returned while first holder still active")
	case <-time.After(50 * time.Millisecond):
		// Expected: second waiter is blocked.
	}

	unlock1()
	select {
	case <-gotLock:
		// Expected: release lets next waiter through.
	case <-time.After(500 * time.Millisecond):
		t.Fatal("second LockAddr did not proceed after first release")
	}
}
