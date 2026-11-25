package storage_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/go-core-fx/cachefx/cache"
	"github.com/stretchr/testify/require"
)

func newStorage(t *testing.T, ttl time.Duration) *storage.Storage {
	t.Helper()

	s, err := storage.New(cache.NewMemory(time.Hour), storage.Config{TTL: ttl})
	require.NoError(t, err)

	return s
}

func TestNew(t *testing.T) {
	t.Parallel()

	_, err := storage.New(cache.NewMemory(time.Hour), storage.Config{TTL: 0})
	require.ErrorIs(t, err, storage.ErrInvalidTTL)

	_, err = storage.New(cache.NewMemory(time.Hour), storage.Config{TTL: -time.Second})
	require.ErrorIs(t, err, storage.ErrInvalidTTL)

	_, err = storage.New(cache.NewMemory(time.Hour), storage.Config{TTL: time.Minute})
	require.NoError(t, err)
}

func TestStorage_Increment(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newStorage(t, time.Hour)

	// Each subtest uses its own key so the subtests stay independent.
	t.Run("new key", func(t *testing.T) {
		t.Parallel()

		count, err := s.Increment(ctx, "key1")
		require.NoError(t, err)
		require.EqualValues(t, 1, count)
	})

	t.Run("existing key", func(t *testing.T) {
		t.Parallel()

		count, err := s.Increment(ctx, "key2")
		require.NoError(t, err)
		require.EqualValues(t, 1, count)

		count, err = s.Increment(ctx, "key2")
		require.NoError(t, err)
		require.EqualValues(t, 2, count)
	})

	t.Run("multiple increments", func(t *testing.T) {
		t.Parallel()

		for i := 1; i <= 3; i++ {
			count, err := s.Increment(ctx, "key3")
			require.NoError(t, err)
			require.EqualValues(t, i, count)
		}
	})

	t.Run("different keys", func(t *testing.T) {
		t.Parallel()

		count1, err := s.Increment(ctx, "key4")
		require.NoError(t, err)
		count2, err := s.Increment(ctx, "key5")
		require.NoError(t, err)

		require.EqualValues(t, 1, count1)
		require.EqualValues(t, 1, count2)
	})
}

func TestStorage_SlidingWindow(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newStorage(t, 2*time.Second)

	count, err := s.Increment(ctx, "key")
	require.NoError(t, err)
	require.EqualValues(t, 1, count)

	time.Sleep(time.Second)
	count, err = s.Increment(ctx, "key")
	require.NoError(t, err)
	require.EqualValues(t, 2, count)

	// The first violation is now older than the ttl, but the second one
	// slid the window forward: a fixed window would have reset to 1 here.
	time.Sleep(1100 * time.Millisecond)
	count, err = s.Increment(ctx, "key")
	require.NoError(t, err)
	require.EqualValues(t, 3, count)
}

func TestStorage_Increment_Expiration(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newStorage(t, 100*time.Millisecond)

	_, err := s.Increment(ctx, "expiring_key")
	require.NoError(t, err)

	// An idle gap longer than the ttl resets the counter, window and all.
	time.Sleep(300 * time.Millisecond)

	count, err := s.Increment(ctx, "expiring_key")
	require.NoError(t, err)
	require.EqualValues(t, 1, count)
}

func BenchmarkStorage_Increment(b *testing.B) {
	s, err := storage.New(cache.NewMemory(time.Hour), storage.Config{TTL: time.Hour})
	if err != nil {
		b.Fatalf("failed to create storage: %v", err)
	}
	ctx := context.Background()

	b.Run("Single Key", func(b *testing.B) {
		b.ResetTimer()
		for range b.N {
			if _, incErr := s.Increment(ctx, "key"); incErr != nil {
				b.Fatalf("error in Increment: %v", incErr)
			}
		}
	})

	b.Run("Multiple Keys", func(b *testing.B) {
		b.ResetTimer()
		for i := range b.N {
			key := fmt.Sprintf("key%d", i%100) // Use 100 different keys
			if _, incErr := s.Increment(ctx, key); incErr != nil {
				b.Fatalf("error in Increment: %v", incErr)
			}
		}
	})
}
