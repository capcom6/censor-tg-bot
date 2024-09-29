package storage

import (
	"go.uber.org/fx"
	"go.uber.org/zap"
)

var Module = fx.Module(
	"storage",
	fx.Decorate(func(logger *zap.Logger) *zap.Logger {
		return logger.Named("storage")
	}),
	fx.Provide(New),
)
