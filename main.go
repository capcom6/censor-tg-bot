package main

import (
	"context"

	"github.com/capcom6/censor-tg-bot/internal/bot"
	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/config"
	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/capcom6/go-infra-fx/logger"
	"go.uber.org/fx"
	"go.uber.org/fx/fxevent"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var module = fx.Module(
	"main",
	fx.Invoke(func(lc fx.Lifecycle) {
		lc.Append(fx.Hook{
			OnStart: func(ctx context.Context) error {
				return nil
			},
			OnStop: func(ctx context.Context) error {
				return nil
			},
		})

		// log.Debug("config", zap.Any("config", cfg))
	}),
)

func main() {
	fx.New(
		fx.WithLogger(func(logger *zap.Logger) fxevent.Logger {
			logOption := fxevent.ZapLogger{Logger: logger}
			logOption.UseLogLevel(zapcore.DebugLevel)
			return &logOption
		}),
		logger.Module,
		config.Module,
		censor.Module,
		storage.Module,
		bot.Module,
		module,
	).Run()
}
