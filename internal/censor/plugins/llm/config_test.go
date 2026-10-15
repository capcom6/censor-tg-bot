package llm_test

import (
	"strings"
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
				BaseURL:             llm.DefaultBaseURL,
				APIKey:              "test-key",
				Model:               "gpt-3.5-turbo",
				Prompt:              "Test prompt",
				ConfidenceThreshold: llm.DefaultConfidenceThreshold,
				Timeout:             llm.DefaultTimeout,
				Temperature:         llm.DefaultTemperature,

				CacheTTL:     llm.DefaultCacheTTL,
				CacheMaxSize: llm.DefaultCacheMaxSize,
				CacheEnabled: true,

				ImageEnabled: false,
				VisionModel:  "",
				ImagePrompt:  llm.DefaultImagePrompt,
				// The inline budget has no default: bot.media.max_bytes is already the
				// download bound and a capping default would silently disable photo
				// moderation.
				MaxInlineImageBytes: 0,
			},
			wantErr: false,
		},
		{
			name: "valid config with all fields",
			config: map[string]any{
				"api_key":              "test-key",
				"model":                "gpt-4",
				"base_url":             "https://custom.example/v1",
				"confidence_threshold": 0.9,
				"timeout":              "45s",
				"prompt":               "Custom prompt",
				"temperature":          0.7,
			},
			want: llm.Config{
				BaseURL:             "https://custom.example/v1",
				APIKey:              "test-key",
				Model:               "gpt-4",
				ConfidenceThreshold: 0.9,
				Timeout:             45 * time.Second,
				Prompt:              "Custom prompt",
				Temperature:         0.7,

				CacheTTL:     llm.DefaultCacheTTL,
				CacheMaxSize: llm.DefaultCacheMaxSize,
				CacheEnabled: true,

				ImageEnabled: false,
				VisionModel:  "",
				ImagePrompt:  llm.DefaultImagePrompt,

				MaxInlineImageBytes: 0,
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
		{
			// Turning photo moderation on without saying which model judges the photos
			// would otherwise ship every photo to the TEXT model and block on a text
			// verdict, so the deployment must fail at startup instead.
			name: "image moderation without a vision model",
			config: map[string]any{
				"api_key":       "test-key",
				"image_enabled": true,
			},
			wantErr: true,
		},
		{
			name: "image moderation with a vision model",
			config: map[string]any{
				"api_key":       "test-key",
				"image_enabled": true,
				"vision_model":  "vision/test-model",
			},
			want: llm.Config{
				BaseURL:             llm.DefaultBaseURL,
				APIKey:              "test-key",
				Model:               llm.DefaultModel,
				Prompt:              llm.DefaultPrompt,
				ConfidenceThreshold: llm.DefaultConfidenceThreshold,
				Timeout:             llm.DefaultTimeout,
				Temperature:         llm.DefaultTemperature,
				CacheTTL:            llm.DefaultCacheTTL,
				CacheMaxSize:        llm.DefaultCacheMaxSize,
				CacheEnabled:        true,
				ImageEnabled:        true,
				VisionModel:         "vision/test-model",
				ImagePrompt:         llm.DefaultImagePrompt,
			},
			wantErr: false,
		},
		{
			name: "image moderation with its own prompt and inline budget",
			config: map[string]any{
				"api_key":                "test-key",
				"image_enabled":          true,
				"vision_model":           "vision/test-model",
				"image_prompt":           "describe the photo",
				"max_inline_image_bytes": 2_000_000,
			},
			want: llm.Config{
				BaseURL:             llm.DefaultBaseURL,
				APIKey:              "test-key",
				Model:               llm.DefaultModel,
				Prompt:              llm.DefaultPrompt,
				ConfidenceThreshold: llm.DefaultConfidenceThreshold,
				Timeout:             llm.DefaultTimeout,
				Temperature:         llm.DefaultTemperature,
				CacheTTL:            llm.DefaultCacheTTL,
				CacheMaxSize:        llm.DefaultCacheMaxSize,
				CacheEnabled:        true,
				ImageEnabled:        true,
				VisionModel:         "vision/test-model",
				ImagePrompt:         "describe the photo",
				MaxInlineImageBytes: 2_000_000,
			},
			wantErr: false,
		},
		{
			name: "a negative inline budget is refused",
			config: map[string]any{
				"api_key":                "test-key",
				"max_inline_image_bytes": -1,
			},
			wantErr: true,
		},
		{
			name: "an oversized vision model id is refused",
			config: map[string]any{
				"api_key":      "test-key",
				"vision_model": strings.Repeat("m", llm.MaxVisionModelLength+1),
			},
			wantErr: true,
		},
		{
			name: "an oversized image prompt is refused",
			config: map[string]any{
				"api_key":      "test-key",
				"image_prompt": strings.Repeat("p", llm.MaxImagePromptLength+1),
			},
			wantErr: true,
		},
		{
			name: "a mistyped image_enabled is refused",
			config: map[string]any{
				"api_key":       "test-key",
				"image_enabled": "yes",
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

	// Photo moderation is off by default because it ships every photo to a third-party
	// provider and costs money per call.
	require.False(t, config.ImageEnabled)
	require.Empty(t, config.VisionModel, "no vision model is shipped as a default")
	require.Equal(t, llm.DefaultImagePrompt, config.ImagePrompt)

	// The inline budget has no default: bot.media.max_bytes is already the download
	// bound, and a capping default would silently kill photo moderation.
	require.Zero(t, config.MaxInlineImageBytes)

	require.NoError(t, config.Validate())
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
