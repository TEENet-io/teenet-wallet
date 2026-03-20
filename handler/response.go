package handler

import "github.com/gin-gonic/gin"

func jsonError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "error": msg})
}
