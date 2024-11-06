package storage_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/config"
	"github.com/capcom6/censor-tg-bot/internal/storage"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		config  config.Storage
		wantErr bool
	}{
		{
			name: "valid memory scheme URL",
			config: config.Storage{
				URL: "memory://storage?ttl=5m",
			},
			wantErr: false,
		},
		{
			name: "invalid scheme URL",
			config: config.Storage{
				URL: "http://storage?ttl=5m",
			},
			wantErr: true,
		},
		{
			name: "URL with invalid TTL query parameter",
			config: config.Storage{
				URL: "memory://storage?ttl=abc",
			},
			wantErr: true,
		},
		{
			name: "URL with missing TTL query parameter",
			config: config.Storage{
				URL: "memory://storage",
			},
			wantErr: true,
		},
		{
			name: "URL with valid TTL query parameter",
			config: config.Storage{
				URL: "memory://storage?ttl=10s",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := storage.New(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func TestStorage_GetOrSet(t *testing.T) {
	s, err := storage.New(config.Storage{URL: "memory://storage?ttl=1h"})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	t.Run("Set new key", func(t *testing.T) {
		count, err := s.GetOrSet("key1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected count 1, got %d", count)
		}
	})

	t.Run("Get existing key", func(t *testing.T) {
		count, err := s.GetOrSet("key1")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if count != 2 {
			t.Errorf("Expected count 2, got %d", count)
		}
	})

	t.Run("Multiple increments", func(t *testing.T) {
		for i := 3; i <= 5; i++ {
			count, err := s.GetOrSet("key1")
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if count != i {
				t.Errorf("Expected count %d, got %d", i, count)
			}
		}
	})

	t.Run("Different keys", func(t *testing.T) {
		count1, _ := s.GetOrSet("key2")
		count2, _ := s.GetOrSet("key3")

		if count1 != 1 || count2 != 1 {
			t.Errorf("Expected both counts to be 1, got %d and %d", count1, count2)
		}
	})
}

func TestStorage_GetOrSet_Expiration(t *testing.T) {
	s, err := storage.New(config.Storage{URL: "memory://storage?ttl=1ms"})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	t.Run("Expired key", func(t *testing.T) {
		_, _ = s.GetOrSet("expiring_key")
		time.Sleep(150 * time.Millisecond) // Wait for the key to expire
		count, err := s.GetOrSet("expiring_key")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected count 1 for expired key, got %d", count)
		}
	})
}

func BenchmarkStorage_GetOrSet(b *testing.B) {
	s, err := storage.New(config.Storage{URL: "memory://storage?ttl=1h"})
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}

	b.Run("Single Key", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_, err := s.GetOrSet("key")
			if err != nil {
				b.Fatalf("Error in GetOrSet: %v", err)
			}
		}
	})

	b.Run("Multiple Keys", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key%d", i%100) // Use 100 different keys
			_, err := s.GetOrSet(key)
			if err != nil {
				b.Fatalf("Error in GetOrSet: %v", err)
			}
		}
	})
}
