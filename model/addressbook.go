// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package model

import "time"

// AddressBookEntry stores a user's saved contact address for a specific chain.
// Nicknames are stored lowercase to enforce case-insensitive uniqueness.
type AddressBookEntry struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id" gorm:"not null;uniqueIndex:idx_user_nickname_chain"`
	Nickname  string    `json:"nickname" gorm:"size:100;not null;uniqueIndex:idx_user_nickname_chain"`
	Chain     string    `json:"chain" gorm:"size:32;not null;uniqueIndex:idx_user_nickname_chain"`
	Address   string    `json:"address" gorm:"size:128;not null"`
	Memo      string    `json:"memo,omitempty" gorm:"size:256"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
