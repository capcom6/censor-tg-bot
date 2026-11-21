package storage

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/go-core-fx/cachefx/cache"
)

// Storage counts violations per key in the app cache with a sliding
// idle-window: every increment restarts the window, so a counter resets only
// after ttl of inactivity. Idle counters are reclaimed by the cachefx sweeper.
type Storage struct {
	cache cache.Cache
	ttl   time.Duration
}

// New creates a Storage over the injected app cache.
func New(c cache.Cache, config Config) (*Storage, error) {
	if config.TTL <= 0 {
		return nil, fmt.Errorf("%w: ttl must be greater than 0", ErrInvalidTTL)
	}

	return &Storage{
		cache: c,
		ttl:   config.TTL,
	}, nil
}

// Increment increments the counter for key and returns the new value while
// sliding the window forward by the configured ttl.
func (s *Storage) Increment(ctx context.Context, key string) (int64, error) {
	count, err := s.cache.Increment(ctx, key, 1, cache.WithTTL(s.ttl))
	if err != nil {
		return 0, fmt.Errorf("failed to increment counter: %w", err)
	}

	// Slide the window: restart the idle timer on every violation. A miss
	// between the two calls is benign - the count above already stands.
	_, getErr := s.cache.Get(ctx, key, cache.AndSetTTL(s.ttl))
	if getErr != nil && !errors.Is(getErr, cache.ErrKeyNotFound) {
		return count, fmt.Errorf("failed to extend counter window: %w", getErr)
	}

	return count, nil
}
