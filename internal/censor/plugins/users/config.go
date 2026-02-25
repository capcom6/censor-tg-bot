package users

import (
	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

// Config holds the plugin configuration.
type Config struct {
	Blacklist []int
	Whitelist []int
}

// NewConfig parses the plugin configuration.
func NewConfig(params map[string]any) (Config, error) {
	var err error
	cfg := Config{
		Blacklist: []int{},
		Whitelist: []int{},
	}

	if cfg.Blacklist, err = plugin.SliceFromAnyOrDefault(params, "blacklist", []int{}); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	if cfg.Whitelist, err = plugin.SliceFromAnyOrDefault(params, "whitelist", []int{}); err != nil {
		return Config{}, err //nolint:wrapcheck // no need
	}

	return cfg, nil
}
