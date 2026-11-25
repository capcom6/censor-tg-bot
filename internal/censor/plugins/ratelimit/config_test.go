package ratelimit_test

import (
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/ratelimit"
	"github.com/stretchr/testify/require"
)

func TestNewConfigWindowValidation(t *testing.T) {
	t.Parallel()

	t.Run("rejects zero window", func(t *testing.T) {
		t.Parallel()

		_, err := ratelimit.NewConfig(map[string]any{"window": "0s"})
		require.ErrorIs(t, err, plugin.ErrInvalidConfig)
		require.Contains(t, err.Error(), "positive")
	})

	t.Run("rejects negative window", func(t *testing.T) {
		t.Parallel()

		_, err := ratelimit.NewConfig(map[string]any{"window": "-5s"})
		require.ErrorIs(t, err, plugin.ErrInvalidConfig)
	})

	t.Run("accepts positive window", func(t *testing.T) {
		t.Parallel()

		c, err := ratelimit.NewConfig(map[string]any{"window": "30s"})
		require.NoError(t, err)
		require.Equal(t, 30*time.Second, c.Window)
	})

	t.Run("default window is positive", func(t *testing.T) {
		t.Parallel()

		c, err := ratelimit.NewConfig(nil)
		require.NoError(t, err)
		require.Equal(t, ratelimit.DefaultWindow, c.Window)
	})
}
