package ratelimit

import (
	"sync"
	"time"
)

// Storage implements a simple in-memory rate limiter.
type Storage struct {
	entries map[int64]*Entry
	mu      sync.Mutex
}

type Entry struct {
	Count   int
	ResetAt time.Time
}

func NewStorage() *Storage {
	return &Storage{
		entries: make(map[int64]*Entry),
		mu:      sync.Mutex{},
	}
}

func (s *Storage) IncrementAndGet(userID int64, window time.Duration) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	entry, exists := s.entries[userID]

	if !exists || now.After(entry.ResetAt) {
		// Create new entry or reset expired entry
		entry = &Entry{
			Count:   1,
			ResetAt: now.Add(window),
		}
		s.entries[userID] = entry
		return entry.Count, nil
	}

	// Increment existing entry
	entry.Count++
	return entry.Count, nil
}

// Cleanup removes expired entries to prevent memory leaks.
func (s *Storage) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for userID, entry := range s.entries {
		if now.After(entry.ResetAt) {
			delete(s.entries, userID)
		}
	}
}
