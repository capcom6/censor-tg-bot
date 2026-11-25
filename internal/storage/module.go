package storage

import (
	appcache "github.com/capcom6/censor-tg-bot/internal/cache"
	"github.com/go-core-fx/cachefx/cache"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// Module provides the violation counter store over the app-scoped cache.
// The cachefx sweeper reclaims idle counters.
func Module() fx.Option {
	return fx.Module(
		"storage",
		fx.Decorate(func(logger *zap.Logger) *zap.Logger {
			return logger.Named("storage")
		}),
		fx.Provide(
			func(factory appcache.Factory) (cache.Cache, error) {
				return factory.New("storage")
			},
			fx.Private,
		),
		fx.Provide(New),
	)
}
