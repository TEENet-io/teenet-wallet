package model

import "time"

// User represents a local wallet user.
// PasskeyUserID links to UMS PasskeyUser; APIKeyHash stores SHA-256 of the raw key.
type User struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Username      string    `json:"username" gorm:"not null"`
	PasskeyUserID uint      `json:"passkey_user_id" gorm:"uniqueIndex"`
	APIKeyHash    *string   `json:"-" gorm:"index"`
	APIKeySalt    *string   `json:"-" gorm:"size:32"` // hex-encoded 16-byte salt
	APIPrefix     string    `json:"api_prefix" gorm:"size:16"`
	CreatedAt     time.Time `json:"created_at"`
}
