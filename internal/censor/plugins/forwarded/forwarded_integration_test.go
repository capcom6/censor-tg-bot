package forwarded_test

import (
	"context"
	"testing"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/forwarded"
	"github.com/go-core-fx/logger"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

func TestPluginIntegration_DICompatibility(t *testing.T) {
	// Test that the plugin can be created and used within the fx DI framework
	var config forwarded.Config
	var pluginInstance plugin.Plugin

	app := fxtest.New(
		t,
		logger.WithNamedLogger("test"),
		fx.Provide(
			func() map[string]any {
				return map[string]any{
					"allowed_user_ids": []any{int64(12345)},
				}
			},
			forwarded.NewConfig,
			forwarded.New,
		),
		fx.Populate(&config),
		fx.Populate(&pluginInstance),
	)

	app.RequireStart()
	defer app.RequireStop()

	// Verify config was populated correctly
	require.Equal(t, []int64{12345}, config.AllowedUserIDs)

	// Verify plugin was created correctly
	require.NotNil(t, pluginInstance)
	require.Equal(t, "forwarded", pluginInstance.Name())

	// Test plugin functionality
	result, err := pluginInstance.Evaluate(context.Background(), plugin.Message{
		Text:                "Test message",
		ForwardedFromUserID: ptr(int64(12345)),
	})
	require.NoError(t, err)
	require.Equal(t, plugin.ActionAllow, result.Action)
}

func ptr[T any](value T) *T {
	return &value
}
