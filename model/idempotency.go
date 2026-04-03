package model

import "time"

// IdempotencyRecord caches the result of a transfer request so that retries
// with the same Idempotency-Key header return the same response without
// re-executing the transaction.
type IdempotencyRecord struct {
	ID         uint      `gorm:"primaryKey"`
	Key        string    `gorm:"size:64;not null;uniqueIndex:idx_idem_key_user_wallet"`
	UserID     uint      `gorm:"not null;uniqueIndex:idx_idem_key_user_wallet"`
	WalletID   string    `gorm:"size:36;not null;uniqueIndex:idx_idem_key_user_wallet;default:''"`
	StatusCode int       `gorm:"not null"`
	Response   string    `gorm:"type:text"`
	ExpiresAt  time.Time `gorm:"index"`
	CreatedAt  time.Time
}
