package ratelimit

import (
	"fmt"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

const (
	DefaultMaxMessages = 5
	DefaultWindow      = time.Minute
)

type Config struct {
	MaxMessages int
	Window      time.Duration
}

func NewConfig(config map[string]any) (Config, error) {
	c := Config{
		MaxMessages: DefaultMaxMessages,
		Window:      DefaultWindow,
	}

	if maxMessages, ok := config["max_messages"]; ok {
		if c.MaxMessages, ok = maxMessages.(int); !ok {
			return Config{}, fmt.Errorf(
				"%w: failed to parse max_messages: expected int, got %T",
				plugin.ErrInvalidConfig,
				maxMessages,
			)
		}
	}

	if window, keyOk := config["window"]; keyOk {
		str, ok := window.(string)
		if !ok {
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

	return c, nil
}
