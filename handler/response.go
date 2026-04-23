// Copyright (C) 2026 TEENet Technology (Hong Kong) Limited. All rights reserved.
// SPDX-License-Identifier: GPL-3.0-only

package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

func jsonError(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "error": msg})
}

func jsonErrorDetails(c *gin.Context, status int, msg string, details gin.H) {
	resp := gin.H{"success": false, "error": msg}
	for k, v := range details {
		if k == "success" || k == "error" {
			continue
		}
		resp[k] = v
	}
	c.JSON(status, resp)
}

// sensitiveURLRe matches http(s)/grpc URLs in error strings. Go's net/http
// and net/url errors embed the full URL — including any credential or token
// in its path (e.g. QuickNode tokens) — so we redact it before putting an
// error into an HTTP response.
var sensitiveURLRe = regexp.MustCompile(`(?i)\b(?:https?|grpc)://\S+`)

// sanitizeErrString redacts any URLs that may appear in err.Error() before
// the string is returned to the client. Safe to use on any external error
// (RPC, gRPC, HTTP, SDK). Returns "" for nil.
func sanitizeErrString(err error) string {
	if err == nil {
		return ""
	}
	return sensitiveURLRe.ReplaceAllString(err.Error(), "<url>")
}

// sanitizeString is like sanitizeErrString but operates on a plain string.
func sanitizeString(s string) string {
	return sensitiveURLRe.ReplaceAllString(s, "<url>")
}

// newRequestID generates an opaque 8-byte correlation ID (16 hex chars)
// that ties a 500 response to its server-side log entry. Falls back to a
// fixed sentinel if the crypto source fails so the helper never panics.
func newRequestID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "0000000000000000"
	}
	return hex.EncodeToString(b[:])
}

// categorizeSigningError maps a TEE/SDK signing error to a short, stable
// category string that is safe to return to clients. The raw error is
// deliberately NOT exposed because it may contain internal gRPC endpoints,
// key names, or certificate paths.
func categorizeSigningError(err error) string {
	if err == nil {
		return ""
	}
	s := strings.ToLower(err.Error())
	switch {
	case strings.Contains(s, "context deadline exceeded") || strings.Contains(s, "timeout") || strings.Contains(s, "timed out"):
		return "timeout"
	case strings.Contains(s, "connection refused") || strings.Contains(s, "unavailable") || strings.Contains(s, "transport is closing"):
		return "tee_unavailable"
	case strings.Contains(s, "threshold") || strings.Contains(s, "not enough participants") || strings.Contains(s, "insufficient"):
		return "threshold_not_reached"
	case strings.Contains(s, "canceled") || strings.Contains(s, "cancelled"):
		return "cancelled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, context.Canceled):
		return "cancelled"
	default:
		return "sdk_error"
	}
}

// respondInternalError logs the underlying error with a request correlation
// ID and returns 500 with the correlation ID only. The raw err is NEVER
// forwarded: DB/internal errors can leak schema, internal endpoints, or
// credentials (e.g. QuickNode tokens embedded in RPC URLs). Clients get a
// stable `request_id` they can quote to support; operators cross-reference
// it in the server log.
func respondInternalError(c *gin.Context, msg string, err error, extra gin.H) {
	reqID := newRequestID()
	details := gin.H{"request_id": reqID}
	for k, v := range extra {
		if k == "request_id" {
			continue
		}
		details[k] = v
	}
	if err != nil {
		slog.Error(msg, "request_id", reqID, "error", err.Error())
	} else {
		slog.Error(msg, "request_id", reqID)
	}
	jsonErrorDetails(c, http.StatusInternalServerError, msg, details)
}
