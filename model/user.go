// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package model

import "time"

// User represents a local wallet user.
// PasskeyUserID links to UMS PasskeyUser.
// Email is set when the user registers via the new email-verified flow;
// it is NULL for users created before email verification was added.
// SQLite's UNIQUE index treats NULLs as distinct, so multiple legacy rows
// without an email coexist.
type User struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	Username      string    `json:"username" gorm:"not null;index"`
	Email         *string   `json:"email,omitempty" gorm:"uniqueIndex"`
	PasskeyUserID uint      `json:"passkey_user_id" gorm:"uniqueIndex"`
	CreatedAt     time.Time `json:"created_at"`
}
