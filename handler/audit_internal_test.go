// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TEENet-io/teenet-wallet/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// newTestDB spins up an isolated SQLite DB with the models the handler
// package touches. Kept minimal — this file only tests pure helpers.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dir := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(dir, "t.db")), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&model.AuditLog{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	return db
}

// TestUpdateAuditByApprovalID_Failed guards the regression where an
// approval that got approved but then failed at signing or broadcast
// left its audit log stuck at status="pending" (so AuditHistory in the
// UI still showed it as pending). The helper must flip the matching
// pending row to the new status and merge any extra details.
func TestUpdateAuditByApprovalID_Failed(t *testing.T) {
	db := newTestDB(t)
	approvalID := uint(42)

	if err := db.Create(&model.AuditLog{
		UserID:     1,
		Action:     "transfer",
		Status:     "pending",
		Details:    `{"type":"transfer","to":"0xaa","amount":"1.5"}`,
		ApprovalID: &approvalID,
		CreatedAt:  time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	updateAuditByApprovalID(db, approvalID, "failed", map[string]interface{}{
		"type":     "transfer",
		"stage":    "broadcast",
		"error":    "dial tcp: connection refused",
	})

	var got model.AuditLog
	if err := db.Where("approval_id = ?", approvalID).First(&got).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != "failed" {
		t.Fatalf("status = %q, want failed", got.Status)
	}
	if got.ApprovedAt == nil {
		t.Error("ApprovedAt should be set after resolution, got nil")
	}

	var details map[string]interface{}
	if err := json.Unmarshal([]byte(got.Details), &details); err != nil {
		t.Fatalf("unmarshal details: %v", err)
	}
	// Pre-existing keys survive.
	if details["to"] != "0xaa" || details["amount"] != "1.5" {
		t.Errorf("pre-existing details lost: %v", details)
	}
	// New keys applied.
	if details["stage"] != "broadcast" || details["error"] != "dial tcp: connection refused" {
		t.Errorf("new details missing: %v", details)
	}
}

// TestUpdateAuditByApprovalID_NoMatchingPending ensures the helper is a
// no-op (and does NOT error) when there is no pending audit row for the
// given approval — callers invoke it unconditionally in failure paths
// and it must be safe to call even if the pending row was already
// resolved or never existed.
func TestUpdateAuditByApprovalID_NoMatchingPending(t *testing.T) {
	db := newTestDB(t)
	// Existing row is *not* pending — helper should do nothing.
	approvalID := uint(99)
	if err := db.Create(&model.AuditLog{
		UserID:     1,
		Action:     "transfer",
		Status:     "success",
		ApprovalID: &approvalID,
		CreatedAt:  time.Now(),
	}).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}

	updateAuditByApprovalID(db, approvalID, "failed", nil)

	var got model.AuditLog
	if err := db.Where("approval_id = ?", approvalID).First(&got).Error; err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got.Status != "success" {
		t.Errorf("non-pending row was clobbered: status = %q", got.Status)
	}
}
