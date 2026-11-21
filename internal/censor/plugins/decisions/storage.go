package decisions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/go-core-fx/cachefx/cache"
)

// Storage is a typed cache for decisions answers over the app cache.
//
// Entries carry the plugin TTL; expired entries are reclaimed by Get and by
// the cachefx sweeper (and by the service cleanup ticker through Cleanup).
type Storage struct {
	cache *cache.Typed[*Response]
	ttl   time.Duration
}

// NewStorage creates a Storage over the injected app cache with the given TTL
// applied to every entry.
func NewStorage(c cache.Cache, ttl time.Duration) *Storage {
	return &Storage{
		cache: cache.NewTyped[*Response](c),
		ttl:   ttl,
	}
}

// generateCacheKey creates a stable hash key from message text, model, context
// and the effective question text. Editing a question in config therefore
// invalidates cached answers for the same message.
func generateCacheKey(text string, model string, contextText string, questionsJSON string) string {
	data := text + "\x00" + model + "\x00" + contextText + "\x00" + questionsJSON
	hash := sha256.Sum256([]byte(data))

	return hex.EncodeToString(hash[:])
}

// GetBy returns the cached answer for the given cache key material.
//
// The bool reports a hit; a missing or expired key is (nil, false, nil), and
// only a real cache failure returns an error.
func (s *Storage) GetBy(ctx context.Context, text, model, contextText, questionsJSON string) (*Response, bool, error) {
	resp, err := s.cache.Get(ctx, generateCacheKey(text, model, contextText, questionsJSON))
	if err != nil {
		if errors.Is(err, cache.ErrKeyNotFound) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("failed to get decisions answer: %w", err)
	}

	return resp, true, nil
}

// SetBy stores an answer under the given cache key material with the storage TTL.
func (s *Storage) SetBy(ctx context.Context, text, model, contextText, questionsJSON string, resp *Response) error {
	key := generateCacheKey(text, model, contextText, questionsJSON)
	if err := s.cache.Set(ctx, key, resp, cache.WithTTL(s.ttl)); err != nil {
		return fmt.Errorf("failed to set decisions answer: %w", err)
	}

	return nil
}
