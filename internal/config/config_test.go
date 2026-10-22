package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/config"
	"github.com/stretchr/testify/require"
)

// writeConfig writes a YAML configuration file and returns its path.
func writeConfig(t *testing.T, body string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yml")
	require.NoError(t, os.WriteFile(path, []byte(body), 0o600))

	return path
}

// mediaEnvIsSet reports whether the environment overrides the media limits. The
// environment is loaded after the YAML file, so it wins over the file.
func mediaEnvIsSet() bool {
	return os.Getenv("BOT__MEDIA__MAX_BYTES") != "" || os.Getenv("BOT__MEDIA__TIMEOUT") != ""
}

func TestDefault_Media(t *testing.T) {
	t.Parallel()

	defaults := config.Default()

	require.Equal(t, int64(10*1024*1024), defaults.Bot.Media.MaxBytes)
	require.Equal(t, 5*time.Second, defaults.Bot.Media.Timeout)
}

func TestNew_MediaDefaultsSurviveTheOverlay(t *testing.T) {
	// The environment is loaded after the YAML file and wins over it, so the
	// defaults can only be observed when the media limits are not set.
	if mediaEnvIsSet() {
		t.Skip("the media limits are set in the environment")
	}

	// The environment of the test cannot be changed in a parallel test, and
	// paralleltest skips the functions that set it.
	t.Setenv("CONFIG_PATH", writeConfig(t, "telegram:\n  token: \"test-token\"\n"))

	cfg, err := config.New()
	require.NoError(t, err)

	require.Equal(t, int64(config.DefaultMediaMaxBytes), cfg.Bot.Media.MaxBytes)
	require.Equal(t, config.DefaultMediaTimeout, cfg.Bot.Media.Timeout)
}

func TestNew_MediaOverride(t *testing.T) {
	if mediaEnvIsSet() {
		t.Skip("the media limits are set in the environment")
	}

	t.Setenv("CONFIG_PATH", writeConfig(t, "bot:\n  media:\n    max_bytes: 1024\n    timeout: 2s\n"))

	cfg, err := config.New()
	require.NoError(t, err)

	require.Equal(t, int64(1024), cfg.Bot.Media.MaxBytes)
	require.Equal(t, 2*time.Second, cfg.Bot.Media.Timeout)
}
