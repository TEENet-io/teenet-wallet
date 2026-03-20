package handler

import (
	"context"
	"encoding/json"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/TEENet-io/teenet-wallet/model"
)

const idempotencyTTL = 24 * time.Hour

// IdempotencyStore manages cached responses for idempotent transfer requests.
// It periodically removes expired records.
type IdempotencyStore struct {
	db     *gorm.DB
	cancel context.CancelFunc
}

// NewIdempotencyStore creates a store and starts a background cleanup goroutine.
func NewIdempotencyStore(db *gorm.DB) *IdempotencyStore {
	ctx, cancel := context.WithCancel(context.Background())
	s := &IdempotencyStore{db: db, cancel: cancel}
	go s.cleanup(ctx)
	return s
}

// Stop cancels the background cleanup goroutine.
func (s *IdempotencyStore) Stop() { s.cancel() }

func (s *IdempotencyStore) cleanup(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.db.Where("expires_at < ?", time.Now()).Delete(&model.IdempotencyRecord{})
		}
	}
}

// Check looks up a cached idempotency record for the given key and user.
// If found, it writes the cached response and returns true; the caller should return.
// If not found, returns false — the caller should proceed with the request.
func (s *IdempotencyStore) Check(c *gin.Context, key string, userID uint) bool {
	if key == "" {
		return false
	}
	var rec model.IdempotencyRecord
	if err := s.db.Where("key = ? AND user_id = ? AND expires_at > ?", key, userID, time.Now()).First(&rec).Error; err != nil {
		return false
	}
	// Return the cached response.
	c.Data(rec.StatusCode, "application/json; charset=utf-8", []byte(rec.Response))
	c.Abort()
	return true
}

// Save stores the response for a given idempotency key.
func (s *IdempotencyStore) Save(key string, userID uint, statusCode int, response gin.H) {
	if key == "" {
		return
	}
	respBytes, _ := json.Marshal(response)
	rec := model.IdempotencyRecord{
		Key:        key,
		UserID:     userID,
		StatusCode: statusCode,
		Response:   string(respBytes),
		ExpiresAt:  time.Now().Add(idempotencyTTL),
		CreatedAt:  time.Now(),
	}
	s.db.Create(&rec)
}

// IdempotencyKey extracts the Idempotency-Key header from the request.
func IdempotencyKey(c *gin.Context) string {
	return c.GetHeader("Idempotency-Key")
}

// SetIdempotencyStore sets the idempotency store on a WalletHandler.
func (h *WalletHandler) SetIdempotencyStore(store *IdempotencyStore) {
	h.idempotency = store
}

// respondWithIdempotency returns a JSON response and caches it for idempotency.
func respondWithIdempotency(c *gin.Context, store *IdempotencyStore, key string, userID uint, statusCode int, response gin.H) {
	if store != nil && key != "" {
		store.Save(key, userID, statusCode, response)
	}
	c.JSON(statusCode, response)
}

// CheckIdempotency checks if the request has already been processed.
// Returns true if a cached response was sent (caller should return).
func CheckIdempotency(c *gin.Context, store *IdempotencyStore, key string, userID uint) bool {
	if store == nil || key == "" {
		return false
	}
	return store.Check(c, key, userID)
}
