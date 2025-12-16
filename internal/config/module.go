package config

import (
	"os"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/bot"
	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
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
			}
		}),
		fx.Provide(func(cfg Config) bot.Config {
			return bot.Config{
				AdminID:      cfg.Bot.AdminID,
				BanThreshold: cfg.Bot.BanThreshold,
			}
		}),
		fx.Provide(func(cfg Config) censor.Config {
			if len(cfg.Censor.Plugins) == 0 {
				cfg.Censor.Plugins = map[string]plugin{
					"keyword": {
						Enabled:  true,
						Priority: 1,
						Config: map[string]any{
							"blacklist": cfg.Censor.Blacklist,
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
		}),
		fx.Provide(func(cfg Config) storage.Config {
			return storage.Config{
				URL: cfg.Storage.URL,
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
