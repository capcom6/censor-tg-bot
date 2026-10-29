package bot_test

import (
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/bot"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		config  bot.Config
		wantErr string
	}{
		{
			name: "valid config",
			config: bot.Config{
				AdminID:      1,
				BanThreshold: 3,
				Media: bot.MediaConfig{
					MaxBytes: 10 * 1024 * 1024,
					Timeout:  5 * time.Second,
				},
			},
		},
		{
			name: "zero max bytes",
			config: bot.Config{
				Media: bot.MediaConfig{
					MaxBytes: 0,
					Timeout:  5 * time.Second,
				},
			},
			wantErr: "media.max_bytes must be positive",
		},
		{
			name: "negative max bytes",
			config: bot.Config{
				Media: bot.MediaConfig{
					MaxBytes: -1,
					Timeout:  5 * time.Second,
				},
			},
			wantErr: "media.max_bytes must be positive",
		},
		{
			name: "zero timeout",
			config: bot.Config{
				Media: bot.MediaConfig{
					MaxBytes: 10 * 1024 * 1024,
					Timeout:  0,
				},
			},
			wantErr: "media.timeout must be positive",
		},
		{
			name: "negative timeout",
			config: bot.Config{
				Media: bot.MediaConfig{
					MaxBytes: 10 * 1024 * 1024,
					Timeout:  -time.Second,
				},
			},
			wantErr: "media.timeout must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := tt.config.Validate()

			if tt.wantErr == "" {
				require.NoError(t, err)

				return
			}

			require.ErrorIs(t, err, bot.ErrInvalidConfig)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}
