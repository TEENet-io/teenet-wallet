// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package model

import "time"

// EmailVerification is a single email-ownership challenge.
//
// Lifecycle:
//  1. send-code inserts a row with CodeHash, ExpiresAt, LastSentAt set,
//     VerifiedAt = nil, ConsumedAt = nil, VerificationID = nil.
//  2. verify-code on success sets VerifiedAt = now and assigns a fresh
//     VerificationID (a 64-char hex random string).
//  3. register/verify deletes the row inside a DB transaction.
//
// CodeHash is sha256(code) hex; the plaintext code is never stored.
//
// VerificationID is a *string (not string) so multiple unverified rows
// can coexist with NULL. SQLite's UNIQUE index treats NULLs as distinct
// but treats two empty strings as a duplicate, which would otherwise
// break a second concurrent send-code call.
type EmailVerification struct {
	ID             uint       `gorm:"primaryKey"`
	Email          string     `gorm:"not null;index"`
	CodeHash       string     `gorm:"not null"`
	Attempts       int        `gorm:"not null;default:0"`
	ExpiresAt      time.Time  `gorm:"not null;index"`
	VerifiedAt     *time.Time `gorm:"index"`
	ConsumedAt     *time.Time
	VerificationID *string    `gorm:"uniqueIndex"`
	CreatedAt      time.Time
	LastSentAt     time.Time
}
