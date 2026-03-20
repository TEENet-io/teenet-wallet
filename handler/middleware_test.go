package handler_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/handler"
	"github.com/TEENet-io/teenet-wallet/model"
)

func testHashKeyWithSalt(raw, salt string) string {
	h := sha256.New()
	h.Write([]byte(salt))
	h.Write([]byte(raw))
	return hex.EncodeToString(h.Sum(nil))
}

// ─── helpers ──────────────────────────────────────────────────────────────────

func testHashKey(raw string) string {
	h := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(h[:])
}

func authRouter(db *gorm.DB, sessions *handler.SessionStore) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(handler.AuthMiddleware(db, sessions))
	r.GET("/test", func(c *gin.Context) {
		userID, _ := c.Get("userID")
		authMode, _ := c.Get("authMode")
		c.JSON(http.StatusOK, gin.H{"user_id": userID, "auth_mode": authMode})
	})
	return r
}

// ─── AuthMiddleware ───────────────────────────────────────────────────────────

func TestAuthMiddleware_NoAuthHeader(t *testing.T) {
	db := testDB(t)
	r := authRouter(db, handler.NewSessionStore())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_InvalidAPIKey(t *testing.T) {
	db := testDB(t)
	r := authRouter(db, handler.NewSessionStore())

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer ocw_doesnotexist")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
}

func TestAuthMiddleware_ValidAPIKey(t *testing.T) {
	db := testDB(t)
	rawKey := "ocw_testkey_abc123"
	salt := "aabbccdd11223344"
	hash := testHashKeyWithSalt(rawKey, salt)
	prefix := rawKey[:12]
	user := model.User{Username: "apikeyuser", PasskeyUserID: 8001, APIKeyHash: &hash, APIKeySalt: &salt, APIPrefix: prefix}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}

	r := authRouter(db, handler.NewSessionStore())
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+rawKey)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_ValidPasskeySession(t *testing.T) {
	db := testDB(t)
	sessions := handler.NewSessionStore()
	sessions.Set("ps_validtoken", 42, 10*time.Minute)

	r := authRouter(db, sessions)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer ps_validtoken")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestAuthMiddleware_ExpiredPasskeySession(t *testing.T) {
	db := testDB(t)
	sessions := handler.NewSessionStore()
	sessions.Set("ps_expired", 42, -1*time.Second) // TTL already in the past

	r := authRouter(db, sessions)
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer ps_expired")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired session, got %d", w.Code)
	}
}

// ─── PasskeyOnlyMiddleware ────────────────────────────────────────────────────

func passkeyOnlyRouter(authMode string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", uint(1))
		c.Set("authMode", authMode)
		c.Next()
	})
	r.Use(handler.PasskeyOnlyMiddleware())
	r.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	return r
}

func TestPasskeyOnlyMiddleware_APIKeyRejected(t *testing.T) {
	r := passkeyOnlyRouter("apikey")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestPasskeyOnlyMiddleware_PasskeyAllowed(t *testing.T) {
	r := passkeyOnlyRouter("passkey")
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
