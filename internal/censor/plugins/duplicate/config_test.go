package duplicate_test

import (
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/duplicate"
	"github.com/stretchr/testify/require"
)

func TestConfig_NewConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		want    duplicate.Config
		wantErr bool
	}{
		{
			name: "valid configuration with all fields",
			config: map[string]any{
				"max_duplicates": 5,
				"window":         "10m",
			},
			want: duplicate.Config{
				MaxDuplicates: 5,
				Window:        10 * time.Minute,
			},
			wantErr: false,
		},
		{
			name:   "valid configuration with defaults",
			config: map[string]any{},
			want: duplicate.Config{
				MaxDuplicates: 1,
				Window:        5 * time.Minute,
				// default
			},
			wantErr: false,
		},
		{
			name: "invalid max_duplicates type",
			config: map[string]any{
				"max_duplicates": "3",
			},
			wantErr: true,
		},
		{
			name: "invalid window type",
			config: map[string]any{
				"window": 300,
			},
			wantErr: true,
		},
		{
			name: "invalid window format",
			config: map[string]any{
				"window": "invalid",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := duplicate.NewConfig(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestConfig_DefaultConfig(t *testing.T) {
	config := duplicate.DefaultConfig()

	require.Equal(t, 1, config.MaxDuplicates)
	require.Equal(t, 5*time.Minute, config.Window)

	// Should be valid by default
	require.NoError(t, config.Validate())
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  duplicate.Config
		wantErr bool
	}{
		{
			name: "valid configuration",
			config: duplicate.Config{
				MaxDuplicates: 3,
				Window:        5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "max_duplicates = 0",
			config: duplicate.Config{
				MaxDuplicates: 0,
				Window:        5 * time.Minute,
			},
			wantErr: false,
		},
		{
			name: "negative max_duplicates",
			config: duplicate.Config{
				MaxDuplicates: -1,
				Window:        5 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "zero window",
			config: duplicate.Config{
				MaxDuplicates: 3,
				Window:        0,
			},
			wantErr: true,
		},
		{
			name: "negative window",
			config: duplicate.Config{
				MaxDuplicates: 3,
				Window:        -1 * time.Minute,
			},
			wantErr: true,
		},
		{
			name: "window too small",
			config: duplicate.Config{
				MaxDuplicates: 3,
				Window:        5 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "window too large",
			config: duplicate.Config{
				MaxDuplicates: 3,
				Window:        48 * time.Hour,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}
