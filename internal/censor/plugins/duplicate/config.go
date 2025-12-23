package duplicate

import (
	"fmt"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

const (
	// DefaultMaxDuplicates is the default maximum number of duplicate messages allowed before blocking.
	DefaultMaxDuplicates = 1
	// DefaultWindow is the default time window to consider messages as duplicates.
	DefaultWindow = 5 * time.Minute
	// MinWindow is the minimum reasonable window duration.
	MinWindow = 10 * time.Second
	// MaxWindow is the maximum reasonable window duration.
	MaxWindow = 24 * time.Hour
)

// Config represents the configuration for the duplicate detection plugin.
type Config struct {
	MaxDuplicates int           // Maximum number of duplicate messages allowed before blocking
	Window        time.Duration // Time window to consider messages as duplicates
}

// NewConfig creates a new configuration from the provided map.
func NewConfig(config map[string]any) (Config, error) {
	c := DefaultConfig()

	// Parse MaxDuplicates
	if maxDuplicates, ok := config["max_duplicates"]; ok {
		if c.MaxDuplicates, ok = maxDuplicates.(int); !ok {
			return Config{}, fmt.Errorf(
				"%w: failed to parse max_duplicates: expected int, got %T",
				plugin.ErrInvalidConfig,
				maxDuplicates,
			)
		}
	}

	// Parse Window
	if window, ok := config["window"]; ok {
		str, windowStrOk := window.(string)
		if !windowStrOk {
			return Config{}, fmt.Errorf(
				"%w: failed to parse window: expected string, got %T",
				plugin.ErrInvalidConfig,
				window,
			)
		}

		var err error
		if c.Window, err = time.ParseDuration(str); err != nil {
			return Config{}, fmt.Errorf("%w: failed to parse window: %w", plugin.ErrInvalidConfig, err)
		}
	}

	// Validate the configuration
	if err := c.Validate(); err != nil {
		return Config{}, err
	}

	return c, nil
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() Config {
	return Config{
		MaxDuplicates: DefaultMaxDuplicates,
		Window:        DefaultWindow,
	}
}

// Validate checks if the configuration values are valid.
func (c Config) Validate() error {
	// Check MaxDuplicates
	if c.MaxDuplicates < 0 {
		return fmt.Errorf(
			"%w: max_duplicates must be >= 0, got: %d",
			plugin.ErrInvalidConfig,
			c.MaxDuplicates,
		)
	}

	// Check Window
	if c.Window < MinWindow {
		return fmt.Errorf("%w: window must be at least %s, got: %s", plugin.ErrInvalidConfig, MinWindow, c.Window)
	}

	if c.Window > MaxWindow {
		return fmt.Errorf("%w: window must not exceed %s, got: %s", plugin.ErrInvalidConfig, MaxWindow, c.Window)
	}

	return nil
}
