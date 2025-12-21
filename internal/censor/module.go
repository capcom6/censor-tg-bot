package censor

import (
	"context"
	"fmt"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins"
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
		fx.Provide(fx.Annotate(
			func(metadata []plugin.Metadata, config Config) ([]plugin.Plugin, error) {
				plugins := make([]plugin.Plugin, 0, len(metadata))
				for _, m := range metadata {
					configMap := map[string]any{}
					if v, ok := config.Plugins[m.Name]; ok {
						configMap = v.Config
					}

					p, err := m.Factory(configMap)
					if err != nil {
						return nil, fmt.Errorf("failed to create plugin %s: %w", m.Name, err)
					}

					plugins = append(plugins, p)
				}
				return plugins, nil
			},
			fx.ParamTags(`group:"metadata"`),
		)),

		// Provide service
		fx.Provide(New),
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
