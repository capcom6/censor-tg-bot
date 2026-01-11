package llm_test

import (
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/llm"
	"github.com/stretchr/testify/require"
)

func TestNewConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]any
		want    llm.Config
		wantErr bool
	}{
		{
			name: "valid minimal config",
			config: map[string]any{
				"api_key": "test-key",
				"model":   "gpt-3.5-turbo",
				"prompt":  "Test prompt",
			},
			want: llm.Config{
				APIKey:              "test-key",
				Model:               "gpt-3.5-turbo",
				Prompt:              "Test prompt",
				ConfidenceThreshold: llm.DefaultConfidenceThreshold,
				Timeout:             llm.DefaultTimeout,
				Temperature:         llm.DefaultTemperature,

				CacheTTL:     llm.DefaultCacheTTL,
				CacheMaxSize: llm.DefaultCacheMaxSize,
				CacheEnabled: true,
			},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			config: map[string]any{
				"api_key":              "test-key",
				"model":                "gpt-4",
				"confidence_threshold": 0.9,
				"timeout":              "45s",
				"prompt":               "Custom prompt",
				"temperature":          0.7,
			},
			want: llm.Config{
				APIKey:              "test-key",
				Model:               "gpt-4",
				ConfidenceThreshold: 0.9,
				Timeout:             45 * time.Second,
				Prompt:              "Custom prompt",
				Temperature:         0.7,

				CacheTTL:     llm.DefaultCacheTTL,
				CacheMaxSize: llm.DefaultCacheMaxSize,
				CacheEnabled: true,
			},
			wantErr: false,
		},
		{
			name:    "missing api_key",
			config:  map[string]any{},
			want:    llm.DefaultConfig(),
			wantErr: false,
		},
		{
			name: "invalid api_key type",
			config: map[string]any{
				"api_key": 123,
			},
			wantErr: true,
		},
		{
			name: "invalid model type",
			config: map[string]any{
				"api_key": "test-key",
				"model":   123,
			},
			wantErr: true,
		},
		{
			name: "invalid confidence_threshold type",
			config: map[string]any{
				"api_key":              "test-key",
				"model":                "gpt-3.5-turbo",
				"confidence_threshold": "0.8",
			},
			wantErr: true,
		},
		{
			name: "invalid timeout format",
			config: map[string]any{
				"api_key": "test-key",
				"model":   "gpt-3.5-turbo",
				"timeout": "invalid",
			},
			wantErr: true,
		},
		{
			name: "missing prompt",
			config: map[string]any{
				"api_key": "test-key",
				"model":   "gpt-3.5-turbo",
				"prompt":  "", // Explicitly set to empty to test missing prompt
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := llm.NewConfig(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  llm.Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: llm.Config{
				APIKey:              "test-key",
				Model:               "gpt-3.5-turbo",
				ConfidenceThreshold: 0.8,
				Timeout:             30 * time.Second,
				Prompt:              "Test prompt",
			},
			wantErr: false,
		},
		{
			name: "missing api_key",
			config: llm.Config{
				Model:               "gpt-3.5-turbo",
				ConfidenceThreshold: 0.8,
				Timeout:             30 * time.Second,
				Prompt:              "Test prompt",
			},
			wantErr: false,
		},
		{
			name: "missing model",
			config: llm.Config{
				APIKey:              "test-key",
				ConfidenceThreshold: 0.8,
				Timeout:             30 * time.Second,
				Prompt:              "Test prompt",
			},
			wantErr: true,
		},
		{
			name: "missing prompt",
			config: llm.Config{
				APIKey:              "test-key",
				Model:               "gpt-3.5-turbo",
				ConfidenceThreshold: 0.8,
				Timeout:             30 * time.Second,
			},
			wantErr: true,
		},
		{
			name: "confidence_threshold too low",
			config: llm.Config{
				APIKey:              "test-key",
				Model:               "gpt-3.5-turbo",
				ConfidenceThreshold: -0.1,
				Timeout:             30 * time.Second,
				Prompt:              "Test prompt",
			},
			wantErr: true,
		},
		{
			name: "confidence_threshold too high",
			config: llm.Config{
				APIKey:              "test-key",
				Model:               "gpt-3.5-turbo",
				ConfidenceThreshold: 1.1,
				Timeout:             30 * time.Second,
				Prompt:              "Test prompt",
			},
			wantErr: true,
		},
		{
			name: "timeout too short",
			config: llm.Config{
				APIKey:              "test-key",
				Model:               "gpt-3.5-turbo",
				ConfidenceThreshold: 0.8,
				Timeout:             1 * time.Second,
				Prompt:              "Test prompt",
			},
			wantErr: true,
		},
		{
			name: "timeout too long",
			config: llm.Config{
				APIKey:              "test-key",
				Model:               "gpt-3.5-turbo",
				ConfidenceThreshold: 0.8,
				Timeout:             10 * time.Minute,
				Prompt:              "Test prompt",
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

func TestDefaultConfig(t *testing.T) {
	config := llm.DefaultConfig()

	// Check that defaults are set correctly
	require.InDelta(t, llm.DefaultConfidenceThreshold, config.ConfidenceThreshold, 0.0001)
	require.Equal(t, llm.DefaultTimeout, config.Timeout)
	require.Equal(t, llm.DefaultModel, config.Model)
	require.InDelta(t, llm.DefaultTemperature, config.Temperature, 0.0001)
	require.NotEmpty(t, config.Prompt)
}

func TestConstants(t *testing.T) {
	// Test that constants are properly defined
	require.InDelta(t, float64(0.8), llm.DefaultConfidenceThreshold, 0.0001)
	require.Equal(t, 30*time.Second, llm.DefaultTimeout)
	require.InDelta(t, float64(0.1), llm.DefaultTemperature, 0.0001)

	require.InDelta(t, float64(0.0), llm.MinConfidenceThreshold, 0.0001)
	require.InDelta(t, float64(1.0), llm.MaxConfidenceThreshold, 0.0001)
	require.Equal(t, 5*time.Second, llm.MinTimeout)
	require.Equal(t, 5*time.Minute, llm.MaxTimeout)
}
