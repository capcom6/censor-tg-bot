package censor

import (
	"fmt"

	appcache "github.com/capcom6/censor-tg-bot/internal/cache"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins"
	"github.com/go-core-fx/fxutil"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

func Module() fx.Option {
	return fx.Module(
		"censor",
		logger.WithNamedLogger("censor"),

		// Provide metrics
		fx.Provide(NewMetrics, fx.Private),

		// Provide plugins
		plugins.Module(),
		fx.Provide(fx.Annotate(
			newPlugins,
			fx.ParamTags(`group:"metadata"`),
		)),

		// Provide service
		fx.Provide(New),
		fx.Invoke(fxutil.RegisterRunnable[*Service]()),
	)
}

// newPlugins builds every registered plugin from its config block, providing
// the shared deps: the app-scoped cache named after the plugin (creation
// failure fails startup) and the module logger.
func newPlugins(
	metadata []plugin.Metadata,
	config Config,
	factory appcache.Factory,
	logger *zap.Logger,
) ([]plugin.Plugin, error) {
	plugins := make([]plugin.Plugin, 0, len(metadata))
	for _, m := range metadata {
		configMap := map[string]any{}
		if v, ok := config.Plugins[m.Name]; ok {
			configMap = v.Config
		}

		c, err := factory.New(m.Name)
		if err != nil {
			return nil, fmt.Errorf("failed to create cache for plugin %s: %w", m.Name, err)
		}

		p, err := m.Factory(configMap, plugin.Deps{Cache: c, Logger: logger})
		if err != nil {
			return nil, fmt.Errorf("failed to create plugin %s: %w", m.Name, err)
		}

		plugins = append(plugins, p)
	}

	return plugins, nil
}
