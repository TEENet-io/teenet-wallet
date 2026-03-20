package model

import "time"

// IdempotencyRecord caches the result of a transfer request so that retries
// with the same Idempotency-Key header return the same response without
// re-executing the transaction.
type IdempotencyRecord struct {
	ID         uint      `gorm:"primaryKey"`
	Key        string    `gorm:"uniqueIndex;size:64;not null"`
	UserID     uint      `gorm:"index;not null"`
	StatusCode int       `gorm:"not null"`
	Response   string    `gorm:"type:text"`
	ExpiresAt  time.Time `gorm:"index"`
	CreatedAt  time.Time
}
