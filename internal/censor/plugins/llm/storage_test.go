package llm_test

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/llm"
)

func TestStorage_GetSet(t *testing.T) {
	// Create a new storage with 10s TTL and max size of 10
	storage := llm.NewStorage(10*time.Second, 10)

	// Test cache miss
	key := "test-key"
	response := &llm.Response{
		Inappropriate: true,
		Confidence:    0.9,
		Reason:        "test reason",
	}

	// Get non-existent key
	cachedResp, found := storage.Get(key)
	if found {
		t.Errorf("Expected cache miss, got cache hit")
	}
	if cachedResp != nil {
		t.Errorf("Expected nil response, got %v", cachedResp)
	}

	// Set a response
	storage.Set(key, response)

	// Get the same key - should be a cache hit
	cachedResp, found = storage.Get(key)
	if !found {
		t.Errorf("Expected cache hit, got cache miss")
	}
	if cachedResp == nil {
		t.Errorf("Expected non-nil response, got nil")
		return
	}
	if cachedResp.Inappropriate != response.Inappropriate ||
		cachedResp.Confidence != response.Confidence ||
		cachedResp.Reason != response.Reason {
		t.Errorf("Expected response %v, got %v", response, cachedResp)
	}
}

func TestStorage_ExpiredEntry(t *testing.T) {
	// Create a new storage with 1ms TTL and max size of 10
	storage := llm.NewStorage(1*time.Millisecond, 10)

	key := "test-key"
	response := &llm.Response{
		Inappropriate: true,
		Confidence:    0.9,
		Reason:        "test reason",
	}

	// Set a response
	storage.Set(key, response)

	// Wait for expiration
	time.Sleep(2 * time.Millisecond)

	// Get the key - should be expired
	cachedResp, found := storage.Get(key)
	if found {
		t.Errorf("Expected cache miss due to expiration, got cache hit")
	}
	if cachedResp != nil {
		t.Errorf("Expected nil response, got %v", cachedResp)
	}
}

func TestStorage_LRU(t *testing.T) {
	// Create a new storage with 10s TTL and max size of 2
	storage := llm.NewStorage(10*time.Second, 2)

	// Set three entries - the first one should be evicted
	key1 := "key1"
	key2 := "key2"
	key3 := "key3"

	response1 := &llm.Response{Inappropriate: true, Confidence: 0.9, Reason: "reason1"}
	response2 := &llm.Response{Inappropriate: false, Confidence: 0.7, Reason: "reason2"}
	response3 := &llm.Response{Inappropriate: true, Confidence: 0.8, Reason: "reason3"}

	storage.Set(key1, response1)
	storage.Set(key2, response2)
	storage.Set(key3, response3)

	// key1 should be evicted (LRU)
	cachedResp, found := storage.Get(key1)
	if found {
		t.Errorf("Expected key1 to be evicted, got cache hit")
	}
	if cachedResp != nil {
		t.Errorf("Expected nil response, got %v", cachedResp)
	}

	// key2 and key3 should still be there
	cachedResp, found = storage.Get(key2)
	if !found {
		t.Errorf("Expected key2 to be present, got cache miss")
	}
	if cachedResp == nil {
		t.Errorf("Expected non-nil response, got nil")
	}

	cachedResp, found = storage.Get(key3)
	if !found {
		t.Errorf("Expected key3 to be present, got cache miss")
	}
	if cachedResp == nil {
		t.Errorf("Expected non-nil response, got nil")
	}
}

func TestStorage_LRU_AccessOrder(t *testing.T) {
	// Create a new storage with 10s TTL and max size of 2
	storage := llm.NewStorage(10*time.Second, 2)

	// Set three entries
	key1 := "key1"
	key2 := "key2"
	key3 := "key3"

	response1 := &llm.Response{Inappropriate: true, Confidence: 0.9, Reason: "reason1"}
	response2 := &llm.Response{Inappropriate: false, Confidence: 0.7, Reason: "reason2"}
	response3 := &llm.Response{Inappropriate: true, Confidence: 0.8, Reason: "reason3"}

	storage.Set(key1, response1)
	storage.Set(key2, response2)

	// Access key1 - it becomes most recently used
	storage.Get(key1)

	// Add key3 - key2 should be evicted (least recently used)
	storage.Set(key3, response3)

	// key2 should be evicted, key1 and key3 should remain
	cachedResp, found := storage.Get(key2)
	if found {
		t.Errorf("Expected key2 to be evicted, got cache hit")
	}
	if cachedResp != nil {
		t.Errorf("Expected nil response, got %v", cachedResp)
	}

	cachedResp, found = storage.Get(key1)
	if !found {
		t.Errorf("Expected key1 to be present, got cache miss")
	}
	if cachedResp == nil {
		t.Errorf("Expected non-nil response, got nil")
	}

	cachedResp, found = storage.Get(key3)
	if !found {
		t.Errorf("Expected key3 to be present, got cache miss")
	}
	if cachedResp == nil {
		t.Errorf("Expected non-nil response, got nil")
	}
}

func TestStorage_Cleanup(t *testing.T) {
	// Create a new storage with 1ms TTL and max size of 10
	storage := llm.NewStorage(1*time.Millisecond, 10)

	key1 := "key1"
	key2 := "key2"
	key3 := "key3"

	response1 := &llm.Response{Inappropriate: true, Confidence: 0.9, Reason: "reason1"}
	response2 := &llm.Response{Inappropriate: false, Confidence: 0.7, Reason: "reason2"}
	response3 := &llm.Response{Inappropriate: true, Confidence: 0.8, Reason: "reason3"}

	storage.Set(key1, response1)
	storage.Set(key2, response2)
	storage.Set(key3, response3)

	// Wait for expiration
	time.Sleep(2 * time.Millisecond)

	// Cleanup should remove all expired entries
	storage.Cleanup()

	// All entries should be gone
	cachedResp, found := storage.Get(key1)
	if found {
		t.Errorf("Expected key1 to be cleaned up, got cache hit")
	}
	if cachedResp != nil {
		t.Errorf("Expected nil response, got %v", cachedResp)
	}

	cachedResp, found = storage.Get(key2)
	if found {
		t.Errorf("Expected key2 to be cleaned up, got cache hit")
	}
	if cachedResp != nil {
		t.Errorf("Expected nil response, got %v", cachedResp)
	}

	cachedResp, found = storage.Get(key3)
	if found {
		t.Errorf("Expected key3 to be cleaned up, got cache hit")
	}
	if cachedResp != nil {
		t.Errorf("Expected nil response, got %v", cachedResp)
	}
}

func TestStorage_MaxSizeValidation(t *testing.T) {
	// Test with zero max size - should be handled by config validation
	// But we should still handle it gracefully
	storage := llm.NewStorage(10*time.Second, 0)

	key := "test-key"
	response := &llm.Response{
		Inappropriate: true,
		Confidence:    0.9,
		Reason:        "test reason",
	}

	// Try to set a response - should still work (but won't evict)
	storage.Set(key, response)

	// Get the same key - should be a cache hit
	cachedResp, found := storage.Get(key)
	if !found {
		t.Errorf("Expected cache hit, got cache miss")
	}
	if cachedResp == nil {
		t.Errorf("Expected non-nil response, got nil")
	}
}

func TestStorage_ConcurrentStress(_ *testing.T) {
	storage := llm.NewStorage(10*time.Second, 100)

	var wg sync.WaitGroup
	const numGoroutines = 10
	const numOps = 100

	for i := range numGoroutines {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := range numOps {
				key := fmt.Sprintf("key-%d-%d", id, j)
				resp := &llm.Response{Inappropriate: true, Confidence: 0.9, Reason: "test"}
				storage.Set(key, resp)
				storage.Get(key)
			}
		}(i)
	}

	wg.Wait()
}
