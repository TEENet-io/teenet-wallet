// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package model

import "time"

// APIKey represents one API key belonging to a user.
// A user may have up to 10 active keys.
type APIKey struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"not null;index"`
	KeyHash   string    `json:"-" gorm:"not null"`
	KeySalt   string    `json:"-" gorm:"size:32;not null"`
	Prefix    string    `json:"prefix" gorm:"size:16;uniqueIndex"`
	Label     string    `json:"label" gorm:"size:100"`
	CreatedAt time.Time `json:"created_at"`
}
