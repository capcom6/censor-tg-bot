package bot

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"bot",
		fx.Decorate(func(logger *zap.Logger) *zap.Logger {
			return logger.Named("bot")
		}),
		fx.Provide(newAPI, fx.Private),
		fx.Provide(New),
		fx.Invoke(func(lc fx.Lifecycle, bot *Bot, log *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					bot.Start()
					log.Info("bot started")
					return nil
				},
				OnStop: func(_ context.Context) error {
					bot.Stop()
					log.Info("bot stopped")
					return nil
				},
			})
		}),
	)
}
