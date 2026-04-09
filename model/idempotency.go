// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package model

import "time"

// IdempotencyRecord caches the result of a transfer request so that retries
// with the same Idempotency-Key header return the same response without
// re-executing the transaction.
type IdempotencyRecord struct {
	ID         uint      `gorm:"primaryKey"`
	Key        string    `gorm:"size:64;not null;uniqueIndex:idx_idem_unique"`
	UserID     uint      `gorm:"not null;uniqueIndex:idx_idem_unique"`
	WalletID   string    `gorm:"size:36;not null;uniqueIndex:idx_idem_unique;default:''"`
	StatusCode int       `gorm:"not null"`
	Response   string    `gorm:"type:text"`
	ExpiresAt  time.Time `gorm:"index"`
	CreatedAt  time.Time
}
