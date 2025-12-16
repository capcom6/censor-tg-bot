package regex

import (
	"fmt"
	"regexp"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/samber/lo"
)

type Config struct {
	Patterns []*regexp.Regexp
}

func NewConfig(config map[string]any) (Config, error) {
	patterns, ok := config["patterns"]
	if !ok {
		return Config{}, nil
	}

	patternsSlice, ok := patterns.([]any)
	if !ok {
		return Config{}, fmt.Errorf("%w: failed to parse patterns", plugin.ErrInvalidConfig)
	}

	patternsStrings, ok := lo.FromAnySlice[string](patternsSlice)
	if !ok {
		return Config{}, fmt.Errorf("%w: failed to parse patterns", plugin.ErrInvalidConfig)
	}

	var patternsRegexp []*regexp.Regexp
	for _, pattern := range patternsStrings {
		patternRegexp, err := regexp.Compile(pattern)
		if err != nil {
			return Config{}, fmt.Errorf("%w: failed to compile pattern %q: %w", plugin.ErrInvalidConfig, pattern, err)
		}

		patternsRegexp = append(patternsRegexp, patternRegexp)
	}

	return Config{
		Patterns: patternsRegexp,
	}, nil
}
