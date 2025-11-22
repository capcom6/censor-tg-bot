package censor

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"censor",
		fx.Decorate(func(logger *zap.Logger) *zap.Logger {
			return logger.Named("censor")
		}),
		fx.Provide(New),
	)
}
