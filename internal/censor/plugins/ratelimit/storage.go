package ratelimit

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-core-fx/cachefx/cache"
)

// Storage implements rate limiting as atomic counters in the app cache.
//
// Each user gets one counter per fixed window: the counter is created with
// the window as its TTL and incremented while it lives, so the next message
// after expiry restarts the window. Idle counters are reclaimed by the
// cachefx sweeper.
type Storage struct {
	cache cache.Cache
}

// NewStorage creates a Storage over the injected app cache.
func NewStorage(c cache.Cache) *Storage {
	return &Storage{cache: c}
}

// IncrementAndGet increments the user's window counter and returns the new value.
func (s *Storage) IncrementAndGet(ctx context.Context, userID int64, window time.Duration) (int64, error) {
	count, err := s.cache.Increment(ctx, strconv.FormatInt(userID, 10), 1, cache.WithTTL(window))
	if err != nil {
		return 0, fmt.Errorf("failed to increment rate limit counter: %w", err)
	}

	return count, nil
}
