package model

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// ApprovalPolicy defines when a signing request requires human approval.
// One policy per wallet — all thresholds and limits are denominated in USD.
// Token prices (ETH, SOL) are fetched at request time; stablecoins are pegged to $1.
type ApprovalPolicy struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	WalletID      string    `json:"wallet_id" gorm:"size:36;not null;uniqueIndex"`
	ThresholdUSD  string    `json:"threshold_usd" gorm:"not null"`   // single-tx USD threshold, e.g. "100"
	DailyLimitUSD string    `json:"daily_limit_usd"`                 // optional: max USD spend per UTC day
	DailySpentUSD string    `json:"daily_spent_usd" gorm:"default:'0'"` // cumulative USD spent today
	DailyResetAt  time.Time `json:"daily_reset_at"`
	Enabled       bool      `json:"enabled" gorm:"default:true"`
	CreatedAt     time.Time `json:"created_at"`
}

// ApprovalRequest is created when a sign/transfer request exceeds the policy threshold,
// or when an API key requests a policy change that requires passkey confirmation.
type ApprovalRequest struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement:false"`
	WalletID     string    `json:"wallet_id" gorm:"size:36;not null;index"`
	UserID       uint      `json:"user_id" gorm:"not null"`
	ApprovalType string    `json:"approval_type" gorm:"default:'sign'"` // "sign", "transfer", "contract_call", "contract_add", "policy_change"
	Message      string    `json:"message" gorm:"type:text"`            // hex signing hash (ETH) or message bytes (SOL); empty for policy_change
	TxContext    string    `json:"tx_context" gorm:"type:text"`         // JSON display info for sign/transfer
	TxParams     string    `json:"tx_params" gorm:"type:text"`          // JSON chain params for broadcast (transfer only)
	PolicyData   string    `json:"policy_data" gorm:"type:text"`        // JSON proposed ApprovalPolicy (policy_change only)
	TxHash       string    `json:"tx_hash" gorm:"type:text"`            // filled after approval + broadcast
	Status       string    `json:"status" gorm:"default:'pending'"`
	Signature    string    `json:"signature" gorm:"type:text"` // filled after sign/transfer approval
	ApprovedBy   *uint     `json:"approved_by"`                // PasskeyUserID of approver
	CreatedAt    time.Time `json:"created_at"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// BeforeCreate generates a random ID for ApprovalRequest so IDs are not sequential.
// Uses 8 digits (10000000–99999999) for ~90M possible values. Retries on collision.
func (a *ApprovalRequest) BeforeCreate(db *gorm.DB) error {
	if a.ID != 0 {
		return nil
	}
	for range 10 {
		var buf [4]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return err
		}
		id := uint(binary.BigEndian.Uint32(buf[:]))%90000000 + 10000000
		var count int64
		db.Model(&ApprovalRequest{}).Where("id = ?", id).Count(&count)
		if count == 0 {
			a.ID = id
			return nil
		}
	}
	return fmt.Errorf("failed to generate unique approval ID after 10 attempts")
}
