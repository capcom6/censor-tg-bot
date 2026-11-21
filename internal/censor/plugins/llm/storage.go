package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/go-core-fx/cachefx/cache"
)

// Storage is a typed cache for LLM responses over the app cache.
//
// Entries carry the plugin TTL; expired entries are reclaimed by the cachefx
// sweeper and evicted lazily on Get.
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

// generateCacheKey creates a stable hash key from message text, model, prompt,
// media identity, provider endpoint and temperature. This ensures identical
// content with same configuration gets cache hits, while the same caption with
// a different image, endpoint or temperature does not.
func generateCacheKey(
	text string,
	model string,
	prompt string,
	mediaHash string,
	endpoint string,
	temperature float64,
) string {
	data := text + "\x00" + model + "\x00" + prompt + "\x00" + mediaHash + "\x00" + endpoint +
		"\x00" + strconv.FormatFloat(temperature, 'g', -1, 64)
	hash := sha256.Sum256([]byte(data))

	return hex.EncodeToString(hash[:])
}

// GetBy returns the cached response for the given cache key material.
//
// The bool reports a hit; a missing or expired key is (nil, false, nil), and
// only a real cache failure returns an error.
func (s *Storage) GetBy(
	ctx context.Context,
	text, model, prompt, mediaHash, endpoint string,
	temperature float64,
) (*Response, bool, error) {
	resp, err := s.cache.Get(ctx, generateCacheKey(text, model, prompt, mediaHash, endpoint, temperature))
	if err != nil {
		if errors.Is(err, cache.ErrKeyNotFound) {
			return nil, false, nil
		}

		return nil, false, fmt.Errorf("failed to get llm response: %w", err)
	}

	return resp, true, nil
}

// SetBy stores a response under the given cache key material with the storage TTL.
func (s *Storage) SetBy(
	ctx context.Context,
	text, model, prompt, mediaHash, endpoint string,
	temperature float64,
	resp *Response,
) error {
	key := generateCacheKey(text, model, prompt, mediaHash, endpoint, temperature)
	if err := s.cache.Set(ctx, key, resp, cache.WithTTL(s.ttl)); err != nil {
		return fmt.Errorf("failed to set llm response: %w", err)
	}

	return nil
}
