package decisions_test

import (
	"context"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/decisions"
	"github.com/go-core-fx/cachefx/cache"
	"github.com/stretchr/testify/require"
)

func testResponse() *decisions.Response {
	return &decisions.Response{
		Model: "r1",
		Answers: map[string]decisions.Answer{
			"category": {Type: "choice", Choice: "other", Confidence: 0.9},
			"spam":     {Type: "noul", Noul: new(0.1)},
			"abuse":    {Type: "noul", Noul: new(0.1)},
		},
		Usage: decisions.Usage{InputTokens: 10, OutputTokens: 5},
	}
}

func TestStorage_GetSet(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	storage := decisions.NewStorage(cache.NewMemory(time.Hour), 10*time.Second)

	_, found, err := storage.GetBy(ctx, "text", "model", "ctx", "questions-v1")
	require.NoError(t, err)
	require.False(t, found)

	require.NoError(t, storage.SetBy(ctx, "text", "model", "ctx", "questions-v1", testResponse()))

	cached, found, err := storage.GetBy(ctx, "text", "model", "ctx", "questions-v1")
	require.NoError(t, err)
	require.True(t, found)
	require.NotNil(t, cached)
	require.Equal(t, "r1", cached.Model)
}

func TestStorage_ExpiredEntry(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	storage := decisions.NewStorage(cache.NewMemory(time.Hour), 1*time.Millisecond)
	require.NoError(t, storage.SetBy(ctx, "text", "model", "ctx", "questions-v1", testResponse()))

	time.Sleep(2 * time.Millisecond)

	cached, found, err := storage.GetBy(ctx, "text", "model", "ctx", "questions-v1")
	require.NoError(t, err)
	require.False(t, found)
	require.Nil(t, cached)
}

func TestStorage_KeyMaterial(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	storage := decisions.NewStorage(cache.NewMemory(time.Hour), 10*time.Second)
	require.NoError(t, storage.SetBy(ctx, "text", "model", "ctx", "questions-v1", testResponse()))

	// Same material hits.
	_, found, err := storage.GetBy(ctx, "text", "model", "ctx", "questions-v1")
	require.NoError(t, err)
	require.True(t, found)

	// Editing the question text in config changes the key.
	_, found, err = storage.GetBy(ctx, "text", "model", "ctx", "questions-v2")
	require.NoError(t, err)
	require.False(t, found)

	// Different model, context or text is a different entry.
	for _, tuple := range [][4]string{
		{"text", "other-model", "ctx", "questions-v1"},
		{"text", "model", "other-ctx", "questions-v1"},
		{"other-text", "model", "ctx", "questions-v1"},
	} {
		_, found, err = storage.GetBy(ctx, tuple[0], tuple[1], tuple[2], tuple[3])
		require.NoError(t, err)
		require.False(t, found, "expected miss for %v", tuple)
	}
}
