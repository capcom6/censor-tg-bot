package llm

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// Storage implements an in-memory cache for LLM responses
// Uses content-based hashing with TTL expiration and LRU eviction.
type Storage struct {
	ttl     time.Duration
	maxSize int
	entries map[string]*CachedResponse
	mu      sync.Mutex
}

// CachedResponse represents a cached LLM response with metadata
// Includes access tracking for LRU eviction.
type CachedResponse struct {
	Response    *Response
	CachedAt    time.Time
	AccessCount int
	LastAccess  time.Time
}

// NewStorage creates a new cache storage with specified TTL and maximum size.
func NewStorage(ttl time.Duration, maxSize int) *Storage {
	return &Storage{
		ttl:     ttl,
		maxSize: maxSize,
		entries: make(map[string]*CachedResponse),
		mu:      sync.Mutex{},
	}
}

// generateCacheKey creates a stable hash key from message text, model, and prompt
// This ensures identical content with same configuration gets cache hits.
func (s *Storage) generateCacheKey(text string, model string, prompt string) string {
	// Include text, model, and prompt in hash to ensure cache validity
	data := text + "\x00" + model + "\x00" + prompt
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Get retrieves cached response if valid and not expired
// Returns the response and a boolean indicating if cache hit occurred.
func (s *Storage) Get(key string) (*Response, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.entries[key]
	if !exists {
		return nil, false
	}

	// Check expiration
	if time.Since(entry.CachedAt) > s.ttl {
		delete(s.entries, key)
		return nil, false
	}

	// Update access metadata
	entry.AccessCount++
	entry.LastAccess = time.Now()

	return entry.Response, true
}

func (s *Storage) GetBy(text, model, prompt string) (*Response, bool) {
	key := s.generateCacheKey(text, model, prompt)
	return s.Get(key)
}

// Set stores a response in cache with LRU eviction if needed.
func (s *Storage) Set(key string, resp *Response) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Evict if at capacity and key doesn't already exist
	if _, exists := s.entries[key]; !exists && len(s.entries) >= s.maxSize {
		s.evictLRU()
	}

	s.entries[key] = &CachedResponse{
		Response:    resp,
		CachedAt:    time.Now(),
		AccessCount: 1,
		LastAccess:  time.Now(),
	}
}

func (s *Storage) SetBy(text, model, prompt string, resp *Response) {
	key := s.generateCacheKey(text, model, prompt)
	s.Set(key, resp)
}

// evictLRU removes the least recently used entry
// Used when cache reaches maximum size.
func (s *Storage) evictLRU() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range s.entries {
		if oldestKey == "" || entry.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = entry.LastAccess
		}
	}

	if oldestKey != "" {
		delete(s.entries, oldestKey)
	}
}

// Cleanup removes all expired entries from cache
// Called periodically to maintain cache health.
func (s *Storage) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.entries {
		if now.Sub(entry.CachedAt) > s.ttl {
			delete(s.entries, key)
		}
	}
}
