package duplicate

import (
	"strconv"
	"sync"
	"time"
)

// Storage implements thread-safe duplicate detection storage.
type Storage struct {
	window  time.Duration
	entries map[string]*Entry // Key format: "chatID:messageHash"
	mu      sync.Mutex
}

// Entry represents a duplicate tracking entry.
type Entry struct {
	Count     int       // Number of duplicate messages seen
	FirstSeen time.Time // Timestamp of first occurrence
	LastSeen  time.Time // Timestamp of most recent occurrence
}

// NewStorage creates a new Storage instance.
func NewStorage(window time.Duration) *Storage {
	return &Storage{
		window:  window,
		entries: make(map[string]*Entry),
		mu:      sync.Mutex{},
	}
}

// generateKey creates a unique key from chatID and messageHash.
func generateKey(chatID int64, messageHash string) string {
	return strconv.FormatInt(chatID, 10) + ":" + messageHash
}

// Record records a duplicate message and returns the current entry state.
func (s *Storage) Record(chatID int64, messageHash string) Entry {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	key := generateKey(chatID, messageHash)
	entry, exists := s.entries[key]

	if !exists {
		// Create new entry
		entry = &Entry{
			Count:     1,
			FirstSeen: now,
			LastSeen:  now,
		}
		s.entries[key] = entry
		return *entry
	}

	// Check if the entry has expired
	if now.Sub(entry.FirstSeen) > s.window {
		// Reset entry as it's outside the window
		entry.Count = 1
		entry.FirstSeen = now
		entry.LastSeen = now
		return *entry
	}

	// Increment existing entry
	entry.Count++
	entry.LastSeen = now

	// Return current entry for limit check by caller
	return *entry
}

// Cleanup removes entries that are older than the specified window.
func (s *Storage) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.entries {
		if now.Sub(entry.FirstSeen) > s.window {
			delete(s.entries, key)
		}
	}
}
