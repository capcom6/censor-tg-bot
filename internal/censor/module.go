package censor

import (
	"context"
	"fmt"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugins"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/duplicate"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/forwarded"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/keyword"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/ratelimit"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/regex"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

//nolint:gocognit //will be fixed
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
		fx.Provide(
			func(config Config) (forwarded.Config, error) {
				configMap := map[string]any{}
				if v, ok := config.Plugins["forwarded"]; ok {
					configMap = v.Config
				}

				c, err := forwarded.NewConfig(configMap)
				if err != nil {
					return c, fmt.Errorf("failed to create forwarded config: %w", err)
				}

				return c, nil
			},
		),
		fx.Provide(
			func(config Config) (duplicate.Config, error) {
				configMap := map[string]any{}
				if v, ok := config.Plugins["duplicate"]; ok {
					configMap = v.Config
				}

				c, err := duplicate.NewConfig(configMap)
				if err != nil {
					return c, fmt.Errorf("failed to create duplicate config: %w", err)
				}

				return c, nil
			},
		),

		// Provide service
		fx.Provide(fx.Annotate(New, fx.ParamTags(`group:"plugins"`))),
		fx.Invoke(func(svc *Service, lc fx.Lifecycle) {
			ctx, cancel := context.WithCancel(context.Background())
			waitCh := make(chan struct{})
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					go func() {
						defer close(waitCh)

						ticker := time.NewTicker(1 * time.Minute)
						defer ticker.Stop()
						for {
							select {
							case <-ticker.C:
								svc.Cleanup(ctx)
							case <-ctx.Done():
								return
							}
						}
					}()
					return nil
				},
				OnStop: func(ctx context.Context) error {
					cancel()
					select {
					case <-waitCh:
					case <-ctx.Done():
					}
					return nil
				},
			})
		}),
	)
}
