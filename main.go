package main

import (
	"context"

	"github.com/capcom6/censor-tg-bot/internal/bot"
	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/config"
	"github.com/capcom6/censor-tg-bot/internal/server"
	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	"github.com/go-core-fx/fiberfx"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func module() fx.Option {
	return fx.Module(
		"main",
		fx.Invoke(func(lc fx.Lifecycle, logger *zap.Logger) {
			lc.Append(fx.Hook{
				OnStart: func(_ context.Context) error {
					logger.Info("app started")
					return nil
				},
				OnStop: func(_ context.Context) error {
					logger.Info("app stopped")
					return nil
				},
			})
		}),
	)
}

func main() {
	fx.New(
		logger.WithFxDefaultLogger(),
		logger.Module(),
		tgbotapifx.Module(),
		fiberfx.Module(),
		//
		config.Module(),
		censor.Module(),
		storage.Module(),
		bot.Module(),
		server.Module(),
		module(),
	).Run()
}
