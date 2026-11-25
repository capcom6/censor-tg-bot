package ratelimit_test

import (
	"context"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/ratelimit"
	"github.com/go-core-fx/cachefx/cache"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func newPlugin(maxMessages int, window time.Duration) plugin.Plugin {
	deps := plugin.Deps{
		Cache:  cache.NewMemory(time.Hour),
		Logger: zap.NewNop(),
	}

	return ratelimit.New(ratelimit.Config{MaxMessages: maxMessages, Window: window}, deps)
}

func TestPlugin_BlocksAfterLimit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := newPlugin(2, time.Minute)
	msg := plugin.Message{UserID: 42, ChatID: 1}

	// Two messages fit the window.
	for range 2 {
		result, err := p.Evaluate(ctx, msg)
		require.NoError(t, err)
		require.Equal(t, plugin.ActionSkip, result.Action)
	}

	// The third exceeds it.
	result, err := p.Evaluate(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, plugin.ActionBlock, result.Action)
	require.EqualValues(t, 3, result.Metadata["count"])
}

func TestPlugin_WindowReset(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := newPlugin(1, 50*time.Millisecond)
	msg := plugin.Message{UserID: 42, ChatID: 1}

	// First message fits, second exceeds within the window.
	result, err := p.Evaluate(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)

	result, err = p.Evaluate(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, plugin.ActionBlock, result.Action)

	// After the window expires the counter restarts.
	time.Sleep(100 * time.Millisecond)

	result, err = p.Evaluate(ctx, msg)
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)
}

func TestPlugin_UsersAreIndependent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	p := newPlugin(1, time.Minute)

	result, err := p.Evaluate(ctx, plugin.Message{UserID: 1, ChatID: 1})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)

	result, err = p.Evaluate(ctx, plugin.Message{UserID: 1, ChatID: 1})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionBlock, result.Action)

	result, err = p.Evaluate(ctx, plugin.Message{UserID: 2, ChatID: 1})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action)
}
