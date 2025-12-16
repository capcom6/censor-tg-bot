package censor

import (
	"fmt"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugins"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/keyword"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/ratelimit"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/regex"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"censor",
		logger.WithNamedLogger("censor"),

		// Provide metrics
		fx.Provide(NewMetrics, fx.Private),

		// Provide plugins
		plugins.Module(),
		fx.Provide(
			func(config Config) (keyword.Config, error) {
				configMap := map[string]any{}
				if v, ok := config.Plugins["keyword"]; ok {
					configMap = v.Config
				}

				c, err := keyword.NewConfig(configMap)
				if err != nil {
					return c, fmt.Errorf("failed to create keyword config: %w", err)
				}

				return c, nil
			},
		),
		fx.Provide(
			func(config Config) (regex.Config, error) {
				configMap := map[string]any{}
				if v, ok := config.Plugins["regex"]; ok {
					configMap = v.Config
				}

				c, err := regex.NewConfig(configMap)
				if err != nil {
					return c, fmt.Errorf("failed to create regex config: %w", err)
				}

				return c, nil
			},
		),
		fx.Provide(
			func(config Config) (ratelimit.Config, error) {
				configMap := map[string]any{}
				if v, ok := config.Plugins["ratelimit"]; ok {
					configMap = v.Config
				}

				c, err := ratelimit.NewConfig(configMap)
				if err != nil {
					return c, fmt.Errorf("failed to create ratelimit config: %w", err)
				}

				return c, nil
			},
		),

		// Provide service
		fx.Provide(fx.Annotate(New, fx.ParamTags(`group:"plugins"`))),
	)
}
