package bot

import (
	"context"

	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module(
	"bot",
	fx.Decorate(func(logger *zap.Logger) *zap.Logger {
		return logger.Named("bot")
	}),
	fx.Provide(newApi, fx.Private),
	fx.Provide(new),
	fx.Invoke(func(lc fx.Lifecycle, bot *bot, log *zap.Logger) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				bot.Start()
				log.Info("bot started")
				return nil
			},
			OnStop: func(ctx context.Context) error {
				bot.Stop()
				log.Info("bot stopped")
				return nil
			},
		})
	}),
)
