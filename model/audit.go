package model

import "time"

// AuditLog records significant user operations for self-auditing.
type AuditLog struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	UserID   uint   `json:"user_id" gorm:"not null;index"`
	Action   string `json:"action" gorm:"size:64;not null;index"` // e.g. "wallet_create", "transfer"
	WalletID *string `json:"wallet_id,omitempty" gorm:"size:36;index"`
	// Details stores optional JSON context (to, amount, tx_hash, etc.).
	Details  string `json:"details" gorm:"type:text"`
	AuthMode    string `json:"auth_mode" gorm:"size:16"` // "passkey" | "apikey" | ""
	APIKeyLabel string `json:"api_key_label,omitempty" gorm:"size:100"`
	IP       string `json:"ip" gorm:"size:64"`
	// Status: "success" | "pending" | "failed"
	Status    string    `json:"status" gorm:"size:16;not null;default:'success'"`
	CreatedAt time.Time `json:"created_at"`
}
