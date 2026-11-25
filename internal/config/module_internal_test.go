package config

import (
	"testing"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/keyword"
	"github.com/stretchr/testify/require"
)

// TestKeywordBlacklistShape covers both producers of the built-in keyword
// config: the default config and the no-plugins fallback. Both must hand the
// parser []any (the YAML-decoded shape), not the struct-typed []string.
func TestKeywordBlacklistShape(t *testing.T) {
	t.Parallel()

	t.Run("default config", func(t *testing.T) {
		t.Parallel()

		cfg := Default()
		keywordConfig := cfg.Censor.Plugins["keyword"].Config

		require.IsType(t, []any{}, keywordConfig["blacklist"])
		_, err := keyword.NewConfig(keywordConfig)
		require.NoError(t, err)
	})

	t.Run("fallback when no plugins configured", func(t *testing.T) {
		t.Parallel()

		cfg := Default()
		cfg.Censor.Plugins = nil
		cfg.Censor.Blacklist = []string{"$"}

		keywordConfig := newCensorConfig(cfg).Plugins["keyword"].Config

		require.IsType(t, []any{}, keywordConfig["blacklist"])
		_, err := keyword.NewConfig(keywordConfig)
		require.NoError(t, err)
	})
}
