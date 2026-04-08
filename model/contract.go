// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package model

import "time"

// AllowedContract is a whitelisted contract address scoped per user + chain.
// All wallets on the same chain share the same whitelist.
// Only contracts in this list can be interacted with via the /transfer endpoint.
// Adding/removing entries requires Passkey authentication (human security gate).
type AllowedContract struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	UserID          uint      `json:"user_id" gorm:"not null;uniqueIndex:idx_user_chain_contract"`
	Chain           string    `json:"chain" gorm:"size:32;not null;uniqueIndex:idx_user_chain_contract"`
	ContractAddress string    `json:"contract_address" gorm:"not null;uniqueIndex:idx_user_chain_contract"` // lowercase hex or base58
	Label           string    `json:"label"`    // e.g. "USDC on Ethereum"
	Symbol          string    `json:"symbol"`   // e.g. "USDC"
	Decimals        int       `json:"decimals"` // e.g. 6
	CreatedAt       time.Time `json:"created_at"`
}
