package censor_test

import (
	"testing"

	"github.com/capcom6/censor-tg-bot/internal/cache"
	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/config"
	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/go-core-fx/cachefx"
	"github.com/go-core-fx/logger"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
)

// TestWiringProbe resolves the cachefx -> app cache -> plugin deps chain, the
// storage counter chain and the cleanup runnable without the Telegram bot,
// proving module wiring end to end.
func TestWiringProbe(t *testing.T) {
	t.Setenv("CONFIG_PATH", "../../configs/config.example.yml")

	app := fx.New(
		logger.Module(),
		logger.WithFxDefaultLogger(),
		cachefx.Module(cachefx.WithSweeper()),
		config.Module(),
		cache.Module(),
		censor.Module(),
		storage.Module(),
		// The app-wide provide lives in main.go, which this probe does not
		// build; a fresh registry per invocation keeps repeated runs
		// (-count=2) from panicking on duplicate registration.
		fx.Provide(func() prometheus.Registerer {
			return prometheus.NewRegistry()
		}),
		// Providers are lazy: demand the store so its constructors run.
		fx.Invoke(func(*storage.Storage) {}),
	)
	require.NoError(t, app.Err())
}
