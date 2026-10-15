package bot_test

import (
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/bot"
	"github.com/stretchr/testify/require"
)

func TestConfigValidate(t *testing.T) {
	valid := func() bot.Config {
		return bot.Config{
			AdminID:      1,
			BanThreshold: 3,
			Media: bot.MediaConfig{
				MaxBytes: 5 * 1024 * 1024,
				Timeout:  5 * time.Second,
			},
		}
	}

	t.Run("positive media values pass", func(t *testing.T) {
		require.NoError(t, valid().Validate())
	})

	t.Run("a non-positive timeout is rejected", func(t *testing.T) {
		cfg := valid()
		cfg.Media.Timeout = 0

		require.ErrorIs(t, cfg.Validate(), bot.ErrInvalidMediaConfig)
	})

	t.Run("a non-positive max_bytes is rejected", func(t *testing.T) {
		cfg := valid()
		cfg.Media.MaxBytes = 0

		require.ErrorIs(t, cfg.Validate(), bot.ErrInvalidMediaConfig)
	})
}
