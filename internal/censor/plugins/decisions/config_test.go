package decisions_test

import (
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/decisions"
	"github.com/stretchr/testify/require"
)

func TestNewConfigDefaults(t *testing.T) {
	t.Parallel()

	c, err := decisions.NewConfig(map[string]any{})
	require.NoError(t, err)
	require.Equal(t, decisions.DefaultConfig(), c)
}

func TestNewConfigAllFields(t *testing.T) {
	t.Parallel()

	c, err := decisions.NewConfig(map[string]any{
		"base_url":        "https://example.test/v1",
		"api_key":         "test-key",
		"model":           "decisions-model",
		"block_threshold": 0.9,
		"timeout":         "45s",
		"context":         "a group about cats",
		"cache_ttl":       "30m",
		"cache_enabled":   false,
	})
	require.NoError(t, err)

	require.Equal(t, "https://example.test/v1", c.BaseURL)
	require.Equal(t, "test-key", c.APIKey)
	require.Equal(t, "decisions-model", c.Model)
	require.InDelta(t, 0.9, c.BlockThreshold, 1e-9)
	require.Equal(t, 45*time.Second, c.Timeout)
	require.Equal(t, "a group about cats", c.Context)
	require.Equal(t, 30*time.Minute, c.CacheTTL)
	require.False(t, c.CacheEnabled)
	require.Equal(t, decisions.DefaultConfig().Questions, c.Questions)
}

func TestNewConfigInvalidValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config map[string]any
	}{
		{"invalid api_key type", map[string]any{"api_key": 123}},
		{"invalid base_url type", map[string]any{"base_url": 1}},
		{"empty model", map[string]any{"model": ""}},
		{"invalid block_threshold type", map[string]any{"block_threshold": "high"}},
		{"block_threshold above range", map[string]any{"block_threshold": 1.5}},
		{"block_threshold below range", map[string]any{"block_threshold": -0.1}},
		{"invalid timeout format", map[string]any{"timeout": "abc"}},
		{"timeout below range", map[string]any{"timeout": "1s"}},
		{"timeout above range", map[string]any{"timeout": "10m"}},
		{"invalid cache_ttl format", map[string]any{"cache_ttl": "nope"}},
		{"cache_ttl below range", map[string]any{"cache_ttl": "10s"}},
		{"invalid context type", map[string]any{"context": 42}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := decisions.NewConfig(tt.config)
			require.Error(t, err)
		})
	}
}

func TestNewConfigQuestionsOverrides(t *testing.T) {
	t.Parallel()

	want := decisions.DefaultConfig().Questions
	want.Spam.Instructions = "Is this an advertisement?"
	want.Spam.Criteria.True = "paid promotion"

	c, err := decisions.NewConfig(map[string]any{
		"questions": map[string]any{
			"spam": map[string]any{
				"instructions": "Is this an advertisement?",
				"criteria": map[string]any{
					"true": "paid promotion",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, want, c.Questions)
}

func TestNewConfigQuestionsBoolKeysNormalize(t *testing.T) {
	t.Parallel()

	want := decisions.DefaultConfig().Questions
	want.Abuse.Criteria.True = "attack"
	want.Abuse.Criteria.False = "friendly"

	// Unquoted YAML `true:` keys decode as booleans and arrive as map[any]any.
	c, err := decisions.NewConfig(map[string]any{
		"questions": map[string]any{
			"abuse": map[string]any{
				"criteria": map[any]any{
					true:  "attack",
					false: "friendly",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, want, c.Questions)
}

func TestNewConfigQuestionsInvalid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		config map[string]any
	}{
		{"unknown question", map[string]any{
			"questions": map[string]any{"toxic": map[string]any{"instructions": "x"}},
		}},
		{"unknown field", map[string]any{
			"questions": map[string]any{"spam": map[string]any{"hint": "x"}},
		}},
		{"empty instructions", map[string]any{
			"questions": map[string]any{"category": map[string]any{"instructions": "  "}},
		}},
		{"instructions not a string", map[string]any{
			"questions": map[string]any{"spam": map[string]any{"instructions": 7}},
		}},
		{"unknown category criterion", map[string]any{
			"questions": map[string]any{
				"category": map[string]any{"criteria": map[string]any{"politics": "x"}},
			},
		}},
		{"unknown noul criterion", map[string]any{
			"questions": map[string]any{
				"spam": map[string]any{"criteria": map[string]any{"maybe": "x"}},
			},
		}},
		{"questions not an object", map[string]any{"questions": "oops"}},
		{"question not an object", map[string]any{
			"questions": map[string]any{"spam": "oops"},
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := decisions.NewConfig(tt.config)
			require.Error(t, err)
		})
	}
}
