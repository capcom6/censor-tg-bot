package censor_test

import (
	"context"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// stubPlugin is a media-capable or plain plugin used to drive the gate.
type stubPlugin struct {
	name     string
	priority int
	needs    bool
}

func (s *stubPlugin) Name() string {
	return s.name
}

func (s *stubPlugin) Evaluate(_ context.Context, _ plugin.Message) (plugin.Result, error) {
	return plugin.Result{
		Action:   plugin.ActionSkip,
		Reason:   "stub",
		Metadata: nil,
		Plugin:   s.name,
	}, nil
}

func (s *stubPlugin) Priority() int {
	return s.priority
}

func (s *stubPlugin) Cleanup(_ context.Context) {}

// NeedsMedia implements plugin.MediaConsumer on purpose: every stub is media-capable,
// so only the NeedsMedia answer decides the gate.
func (s *stubPlugin) NeedsMedia(_ plugin.Message) bool {
	return s.needs
}

func TestRequiresMedia(t *testing.T) {
	// promauto registers process-globally, so the metrics must be built exactly once.
	metrics := censor.NewMetrics()
	logger := zap.NewNop()

	newService := func(t *testing.T, enabledOnly bool, enabled bool, plugins ...plugin.Plugin) *censor.Service {
		t.Helper()

		pluginConfigs := make(map[string]censor.PluginConfig, len(plugins))
		for _, p := range plugins {
			pluginConfigs[p.Name()] = censor.PluginConfig{
				Enabled:  enabled,
				Priority: p.Priority(),
			}
		}

		return censor.New(plugins, censor.Config{
			Strategy:    censor.StrategySequential,
			Timeout:     5 * time.Second,
			Plugins:     pluginConfigs,
			EnabledOnly: enabledOnly,
			ErrorAction: plugin.ActionBlock,
			SkipAction:  plugin.ActionSkip,
		}, metrics, logger)
	}

	mediaPlugin := &stubPlugin{name: "media", priority: 100, needs: true}
	textPlugin := &stubPlugin{name: "text", priority: 200, needs: false}

	t.Run("no media-capable plugin", func(t *testing.T) {
		svc := newService(t, true, true, textPlugin)

		require.False(t, svc.RequiresMedia())
	})

	t.Run("media-capable plugin among the enabled set", func(t *testing.T) {
		svc := newService(t, true, true, mediaPlugin, textPlugin)

		require.True(t, svc.RequiresMedia())
	})

	t.Run("media-capable plugin filtered out by EnabledOnly", func(t *testing.T) {
		svc := newService(t, true, false, mediaPlugin, textPlugin)

		// The gate must share the filter with GetPlugins, otherwise a disabled
		// media-capable plugin would still cause a download.
		require.Empty(t, svc.GetPlugins())
		require.False(t, svc.RequiresMedia())
	})

	t.Run("media-capable plugin absent from the plugin config", func(t *testing.T) {
		svc := newService(t, true, false, textPlugin)
		require.Empty(t, svc.GetPlugins())
		require.False(t, svc.RequiresMedia())

		// The same registry without EnabledOnly keeps every registered plugin.
		openSvc := censor.New([]plugin.Plugin{mediaPlugin}, censor.Config{
			Strategy:    censor.StrategySequential,
			Timeout:     5 * time.Second,
			Plugins:     map[string]censor.PluginConfig{},
			EnabledOnly: false,
			ErrorAction: plugin.ActionBlock,
			SkipAction:  plugin.ActionSkip,
		}, metrics, logger)
		require.Len(t, openSvc.GetPlugins(), 1)
		require.True(t, openSvc.RequiresMedia())
	})

	t.Run("empty registry", func(t *testing.T) {
		svc := newService(t, true, true)

		require.Empty(t, svc.GetPlugins())
		require.False(t, svc.RequiresMedia())
	})
}
