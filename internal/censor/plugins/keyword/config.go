package keyword

import (
	"fmt"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/samber/lo"
)

type Config struct {
	Blacklist []string
}

func NewConfig(config map[string]any) (Config, error) {
	blacklist, ok := config["blacklist"]
	if !ok {
		return Config{}, nil
	}

	blacklistSlice, ok := blacklist.([]any)
	if !ok {
		return Config{}, fmt.Errorf("%w: failed to parse blacklist", plugin.ErrInvalidConfig)
	}

	blacklistStrings, ok := lo.FromAnySlice[string](blacklistSlice)
	if !ok {
		return Config{}, fmt.Errorf("%w: failed to parse blacklist", plugin.ErrInvalidConfig)
	}

	return Config{
		Blacklist: blacklistStrings,
	}, nil
}
