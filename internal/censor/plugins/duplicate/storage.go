package duplicate

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-core-fx/cachefx/cache"
)

// Storage implements duplicate detection as atomic counters in the app cache.
//
// One counter lives per (chat, message hash) with the detection window as its
// TTL: the counter is created on first sight and incremented while it lives,
// so an expired counter restarts the window. Idle counters are reclaimed by
// the cachefx sweeper.
type Storage struct {
	cache  cache.Cache
	window time.Duration
}

// NewStorage creates a Storage over the injected app cache.
func NewStorage(c cache.Cache, window time.Duration) *Storage {
	return &Storage{
		cache:  c,
		window: window,
	}
}

// generateKey creates a unique key from chatID and messageHash.
func generateKey(chatID int64, messageHash string) string {
	return strconv.FormatInt(chatID, 10) + ":" + messageHash
}

// Record increments the counter for chatID/messageHash and returns the new value.
func (s *Storage) Record(ctx context.Context, chatID int64, messageHash string) (int64, error) {
	count, err := s.cache.Increment(ctx, generateKey(chatID, messageHash), 1, cache.WithTTL(s.window))
	if err != nil {
		return 0, fmt.Errorf("failed to increment duplicate counter: %w", err)
	}

	return count, nil
}
