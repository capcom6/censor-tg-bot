package config

import "go.uber.org/fx"

var Module = fx.Module(
	"config",
	fx.Provide(Get),
	fx.Provide(func(cfg Config) Telegram {
		return cfg.Telegram
	}),
	fx.Provide(func(cfg Config) Censor {
		return cfg.Censor
	}),
)
