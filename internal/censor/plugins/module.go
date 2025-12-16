package plugins

import (
	"context"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/keyword"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/ratelimit"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/regex"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"plugins",
		logger.WithNamedLogger("plugins"),
		fx.Provide(
			ratelimit.NewStorage,
			fx.Private,
		),
		fx.Provide(
			fx.Annotate(keyword.New, fx.ResultTags(`group:"plugins"`)),
			fx.Annotate(ratelimit.New, fx.ResultTags(`group:"plugins"`)),
			fx.Annotate(regex.New, fx.ResultTags(`group:"plugins"`)),
		),
		fx.Invoke(func(storage *ratelimit.Storage, lc fx.Lifecycle) {
			ctx, cancel := context.WithCancel(context.Background())
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					go func() {
						ticker := time.NewTicker(1 * time.Minute)
						defer ticker.Stop()
						for {
							select {
							case <-ticker.C:
								storage.Cleanup()
							case <-ctx.Done():
								return
							}
						}
					}()
					return nil
				},
				OnStop: func(_ context.Context) error {
					cancel()
					return nil
				},
			})
		}),
	)
}
