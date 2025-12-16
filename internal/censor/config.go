package censor

import (
	"fmt"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

// Config for the plugin manager.
type Config struct {
	Strategy    ExecutionStrategy
	Timeout     time.Duration
	Plugins     map[string]PluginConfig
	EnabledOnly bool

	ErrorAction plugin.Action
	SkipAction  plugin.Action
}

// PluginConfig for individual plugin configuration.
type PluginConfig struct {
	Enabled  bool
	Priority int
	Config   map[string]any
}

// Validate checks if the configuration is valid.
func (c Config) Validate() error {
	// Check strategy
	if !c.Strategy.IsValid() {
		return fmt.Errorf("%w: invalid execution strategy: %s", ErrInvalidConfig, c.Strategy)
	}

	// Check timeout
	if c.Timeout <= 0 {
		return fmt.Errorf("%w: invalid timeout: %s", ErrInvalidConfig, c.Timeout)
	}

	// Check action fields
	validActions := map[plugin.Action]bool{
		plugin.ActionSkip:  false,
		plugin.ActionAllow: true,
		plugin.ActionBlock: true,
	}
	if !validActions[c.ErrorAction] {
		return fmt.Errorf("%w: invalid error action: %s", ErrInvalidConfig, c.ErrorAction)
	}
	if !validActions[c.SkipAction] {
		return fmt.Errorf("%w: invalid skip action: %s", ErrInvalidConfig, c.SkipAction)
	}

	// Check plugin configurations
	for name, config := range c.Plugins {
		if config.Priority < 0 {
			return fmt.Errorf("%w: invalid priority for plugin %s: %d", ErrInvalidConfig, name, config.Priority)
		}
	}

	return nil
}
