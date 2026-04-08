// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package model

import "time"

// User represents a local wallet user.
// PasskeyUserID links to UMS PasskeyUser.
type User struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Username      string    `json:"username" gorm:"not null;index"`
	PasskeyUserID uint      `json:"passkey_user_id" gorm:"uniqueIndex"`
	CreatedAt     time.Time `json:"created_at"`
}
