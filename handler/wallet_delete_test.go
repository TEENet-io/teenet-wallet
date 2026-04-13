// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/handler"
	"github.com/TEENet-io/teenet-wallet/model"
)

func walletDeleteRouter(db *gorm.DB, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	wh := handler.NewWalletHandler(db, nil, "", 0)
	r.Use(func(c *gin.Context) {
		c.Set("userID", userID)
		c.Set("authMode", "passkey")
		c.Next()
	})
	r.DELETE("/wallets/:id", wh.DeleteWallet)
	return r
}

func TestDeleteWallet_DoesNotTouchSharedAllowedContracts(t *testing.T) {
	db := testDB(t)
	user, wallet := seedWallet(t, db)

	contract := model.AllowedContract{
		UserID:          user.ID,
		Chain:           wallet.Chain,
		ContractAddress: "0x1234567890123456789012345678901234567890",
		Label:           "USDC",
	}
	if err := db.Create(&contract).Error; err != nil {
		t.Fatalf("create allowed contract: %v", err)
	}

	r := walletDeleteRouter(db, user.ID)
	req := httptest.NewRequest(http.MethodDelete, "/wallets/"+wallet.ID, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}

	var walletCount int64
	db.Model(&model.Wallet{}).Where("id = ?", wallet.ID).Count(&walletCount)
	if walletCount != 0 {
		t.Fatalf("expected wallet to be deleted, found %d rows", walletCount)
	}

	var contractCount int64
	db.Model(&model.AllowedContract{}).Where("id = ?", contract.ID).Count(&contractCount)
	if contractCount != 1 {
		t.Fatalf("expected shared allowed contract to remain, found %d rows", contractCount)
	}
}
