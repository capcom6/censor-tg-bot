package tgbotapifx

import (
	"context"

	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"tgbotapifx",
		logger.WithNamedLogger("tgbotapifx"),
		fx.Provide(New),
		fx.Invoke(func(lc fx.Lifecycle, bot *Bot, logger *zap.Logger) {
			ctx, cancel := context.WithCancel(context.Background())
			waitCh := make(chan struct{})

			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					go func() {
						bot.Run(ctx)
						close(waitCh)
					}()

					logger.Info("bot started")
					return nil
				},
				OnStop: func(ctx context.Context) error {
					cancel()
					select {
					case <-waitCh:
					case <-ctx.Done():
						logger.Warn("bot stop timed out")
						return ctx.Err()
					}
					logger.Info("bot stopped")
					return nil
				},
			})
		}),
	)
}
