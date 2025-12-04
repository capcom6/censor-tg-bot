package censor

import (
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"censor",
		logger.WithNamedLogger("censor"),
		fx.Provide(NewMetrics, fx.Private),
		fx.Provide(New),
	)
}
