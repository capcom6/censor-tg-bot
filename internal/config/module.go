package config

import (
	"os"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/bot"
	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	"github.com/go-core-fx/cachefx"
	"github.com/go-core-fx/fiberfx"
	"github.com/samber/lo"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"config",
		fx.Provide(New),
		fx.Provide(func(cfg Config) tgbotapifx.Config {
			return tgbotapifx.Config{
				Token:           cfg.Telegram.Token,
				LongPollTimeout: time.Minute,
				Debug:           os.Getenv("DEBUG") != "",
				ProxyURL:        cfg.Telegram.ProxyURL,
				Timeout:         max(cfg.Telegram.Timeout, time.Minute+10*time.Second),
			}
		}),
		fx.Provide(func(cfg Config) bot.Config {
			return bot.Config{
				AdminID:      cfg.Bot.AdminID,
				BanThreshold: cfg.Bot.BanThreshold,

				Media: bot.MediaConfig{
					MaxBytes: cfg.Bot.Media.MaxBytes,
					Timeout:  cfg.Bot.Media.Timeout,
				},
			}
		}),
		fx.Provide(newCensorConfig),
		fx.Provide(func(cfg Config) storage.Config {
			return storage.Config{
				TTL: cfg.Storage.TTL,
			}
		}),
		fx.Provide(func(cfg Config) cachefx.Config {
			return cachefx.Config{
				URL:           cfg.Cache.URL,
				SweepInterval: cfg.Cache.SweepInterval,
			}
		}),
		fx.Provide(func(cfg Config) fiberfx.Config {
			return fiberfx.Config{
				Address:     cfg.HTTP.Address,
				ProxyHeader: cfg.HTTP.ProxyHeader,
				Proxies:     cfg.HTTP.Proxies,
			}
		}),
	)
}

// newCensorConfig maps the app config to the censor service config, seeding
// the built-in keyword plugin when no plugins are configured. Its blacklist
// must arrive as []any - the shape the YAML loader produces and the plugin
// parser expects.
func newCensorConfig(cfg Config) censor.Config {
	if len(cfg.Censor.Plugins) == 0 {
		cfg.Censor.Plugins = map[string]plugin{
			"keyword": {
				Enabled:  true,
				Priority: 1,
				Config: map[string]any{
					"blacklist": lo.ToAnySlice(cfg.Censor.Blacklist),
				},
			},
		}
	}

	return censor.Config{
		Strategy:    cfg.Censor.Strategy,
		Timeout:     cfg.Censor.Timeout,
		EnabledOnly: cfg.Censor.EnabledOnly,
		Plugins: lo.MapValues(
			cfg.Censor.Plugins,
			func(p plugin, _ string) censor.PluginConfig {
				return censor.PluginConfig{
					Enabled:  p.Enabled,
					Priority: p.Priority,
					Config:   p.Config,
				}
			},
		),
		ErrorAction: cfg.Censor.ErrorAction,
		SkipAction:  cfg.Censor.SkipAction,
	}
}
