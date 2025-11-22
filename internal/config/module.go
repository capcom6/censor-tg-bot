package config

import "go.uber.org/fx"

func Module() fx.Option {
	return fx.Module(
		"config",
		fx.Provide(Get),
		fx.Provide(func(cfg Config) Telegram {
			return cfg.Telegram
		}),
		fx.Provide(func(cfg Config) Censor {
			return cfg.Censor
		}),
		fx.Provide(func(cfg Config) Storage {
			return cfg.Storage
		}),
	)
}
