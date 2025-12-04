package config

import (
	"os"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/bot"
	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	"github.com/go-core-fx/fiberfx"
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
			return censor.Config{
				Blacklist: cfg.Censor.Blacklist,
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
