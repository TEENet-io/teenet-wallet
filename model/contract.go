package model

import "time"

// AllowedContract is a whitelisted ERC-20 (or other) contract address for a wallet.
// Only contracts in this list can be interacted with via the /transfer endpoint.
// Adding/removing entries requires Passkey authentication (human security gate).
type AllowedContract struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	WalletID        uint      `json:"wallet_id" gorm:"not null;uniqueIndex:idx_wallet_contract"`
	ContractAddress string    `json:"contract_address" gorm:"not null;uniqueIndex:idx_wallet_contract"` // lowercase hex
	Label           string    `json:"label"`    // e.g. "USDC on Ethereum"
	Symbol          string    `json:"symbol"`   // e.g. "USDC"
	Decimals        int       `json:"decimals"` // e.g. 6
	AllowedMethods  string    `json:"allowed_methods"`                   // comma-separated method names, empty = all allowed
	AutoApprove     bool      `json:"auto_approve" gorm:"default:false"` // true = API Key can execute without Passkey approval
	CreatedAt       time.Time `json:"created_at"`
}
