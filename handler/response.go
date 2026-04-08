// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import "github.com/gin-gonic/gin"

func jsonError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "error": msg})
}

func jsonErrorDetails(c *gin.Context, status int, msg string, details gin.H) {
	resp := gin.H{"success": false, "error": msg}
	for k, v := range details {
		resp[k] = v
	}
	c.JSON(status, resp)
}
