// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package chain

import (
	"errors"
	"math/big"
	"testing"
)

// TestBumpETHGas_BumpsBothFees verifies a standard 25%-ish bump is applied to
// both fee fields and both land comfortably above the EIP-1559 +12.5% floor.
func TestBumpETHGas_BumpsBothFees(t *testing.T) {
	p := ETHTxParams{
		MaxFeePerGas:         "1010000000",  // 1.01 gwei
		MaxPriorityFeePerGas: "1000000000",  // 1 gwei — exactly what teenet-wallet's floor produces
	}
	if !BumpETHGas(&p) {
		t.Fatal("BumpETHGas returned false for normal values")
	}

	newTip, _ := new(big.Int).SetString(p.MaxPriorityFeePerGas, 10)
	newMax, _ := new(big.Int).SetString(p.MaxFeePerGas, 10)

	// EIP-1559 requires replacement gas to be >= original * 1.125.
	oldTip := big.NewInt(1_000_000_000)
	oldMax := big.NewInt(1_010_000_000)
	minTip := new(big.Int).Mul(oldTip, big.NewInt(1125))
	minTip.Div(minTip, big.NewInt(1000))
	minMax := new(big.Int).Mul(oldMax, big.NewInt(1125))
	minMax.Div(minMax, big.NewInt(1000))

	if newTip.Cmp(minTip) < 0 {
		t.Errorf("bumped tip %s does not exceed +12.5%% floor (need >= %s)", newTip, minTip)
	}
	if newMax.Cmp(minMax) < 0 {
		t.Errorf("bumped maxFee %s does not exceed +12.5%% floor (need >= %s)", newMax, minMax)
	}
	// Actual ratio should be roughly 1.25x.
	if newTip.Cmp(big.NewInt(1_250_000_000)) < 0 {
		t.Errorf("bumped tip %s unexpectedly low (want ~1.25 gwei)", newTip)
	}
}

// TestBumpETHGas_RefusesAtCap verifies the 10 gwei priority-fee ceiling kicks
// in and BumpETHGas returns false rather than escalating forever.
func TestBumpETHGas_RefusesAtCap(t *testing.T) {
	p := ETHTxParams{
		MaxFeePerGas:         "20000000000", // 20 gwei
		MaxPriorityFeePerGas: "9000000000",  // 9 gwei — one bump should exceed 10 gwei cap
	}
	if BumpETHGas(&p) {
		t.Errorf("BumpETHGas should refuse to bump above the 10 gwei priority cap, got tip=%s", p.MaxPriorityFeePerGas)
	}
}

// TestBumpETHGas_ProgressiveRBFChain models a 3-step RBF escalation so we can
// eyeball the gas trajectory stays sane and each step beats the prior one by
// >=12.5%.
func TestBumpETHGas_ProgressiveRBFChain(t *testing.T) {
	p := ETHTxParams{
		MaxFeePerGas:         "1010000000",
		MaxPriorityFeePerGas: "1000000000",
	}
	var prevTip *big.Int
	prevTip, _ = new(big.Int).SetString(p.MaxPriorityFeePerGas, 10)
	for step := 1; step <= 3; step++ {
		if !BumpETHGas(&p) {
			t.Fatalf("step %d: BumpETHGas refused prematurely", step)
		}
		tip, _ := new(big.Int).SetString(p.MaxPriorityFeePerGas, 10)
		minRequired := new(big.Int).Mul(prevTip, big.NewInt(1125))
		minRequired.Div(minRequired, big.NewInt(1000))
		if tip.Cmp(minRequired) < 0 {
			t.Errorf("step %d: tip %s fails +12.5%% vs previous %s", step, tip, prevTip)
		}
		prevTip = tip
	}
	// After 3 bumps starting from 1 gwei, tip should be roughly 1.95 gwei —
	// well under the 10 gwei cap.
	if prevTip.Cmp(big.NewInt(3_000_000_000)) > 0 {
		t.Errorf("3 bumps from 1 gwei yielded %s (wanted < 3 gwei)", prevTip)
	}
}

// TestIsReplacementUnderpriced covers the error-string variants the handler
// uses to decide whether an RBF retry is worth attempting.
func TestIsReplacementUnderpriced(t *testing.T) {
	cases := []struct {
		name   string
		err    error
		expect bool
	}{
		{"nil", nil, false},
		{"replacement full",
			errors.New("broadcast: rpc error: map[code:-32000 message:replacement transaction underpriced]"), true},
		{"replacement short",
			errors.New("rpc error: replacement underpriced"), true},
		{"bare underpriced",
			errors.New("transaction underpriced"), true},
		{"mixed case",
			errors.New("REPLACEMENT Transaction Underpriced"), true},
		{"unrelated broadcast error",
			errors.New("broadcast: nonce too low"), false},
		{"revert",
			errors.New("execution reverted: ERC20: insufficient balance"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsReplacementUnderpriced(tc.err); got != tc.expect {
				t.Errorf("IsReplacementUnderpriced(%q) = %v, want %v", tc.err, got, tc.expect)
			}
		})
	}
}

// TestETHSigningHash_StableAndChangesWithGas proves two invariants:
//   - signing hash is deterministic for the same params
//   - bumping gas via BumpETHGas does change the hash (required, or the TEE
//     re-sign would produce the same sig and QN would reject "already known")
func TestETHSigningHash_StableAndChangesWithGas(t *testing.T) {
	base := ETHTxParams{
		ChainID:              "84532",
		Nonce:                5,
		MaxFeePerGas:         "1010000000",
		MaxPriorityFeePerGas: "1000000000",
		GasLimit:             21000,
		From:                 "0x31184fe88A211C7fc2f44849D7B0542Db457D6E5",
		To:                   "0x294816af8f27faf0536f853b116a313813c703e1",
		Value:                "100000000000000",
	}
	h1, err := ETHSigningHash(base)
	if err != nil {
		t.Fatalf("ETHSigningHash failed: %v", err)
	}
	h2, err := ETHSigningHash(base)
	if err != nil {
		t.Fatalf("ETHSigningHash failed on repeat: %v", err)
	}
	if string(h1) != string(h2) {
		t.Fatal("ETHSigningHash not deterministic for identical params")
	}

	bumped := base
	if !BumpETHGas(&bumped) {
		t.Fatal("BumpETHGas refused bump")
	}
	h3, err := ETHSigningHash(bumped)
	if err != nil {
		t.Fatalf("ETHSigningHash on bumped params failed: %v", err)
	}
	if string(h1) == string(h3) {
		t.Fatal("bumping gas must change the signing hash, but it did not")
	}
}
