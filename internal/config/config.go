package config

import (
	"fmt"

	"github.com/go-core-fx/config"
)

type Bot struct {
	AdminID      int64 `koanf:"admin_id"`
	BanThreshold uint8 `koanf:"ban_threshold"`
}

type Telegram struct {
	Token string `koanf:"token"`
}

type Censor struct {
	Blacklist []string `koanf:"blacklist"`
}

type Storage struct {
	URL string `koanf:"url"`
}

type HTTP struct {
	Address     string   `koanf:"address"`
	ProxyHeader string   `koanf:"proxy_header"`
	Proxies     []string `koanf:"proxies"`
}

type Config struct {
	Bot      Bot      `koanf:"bot"`
	Telegram Telegram `koanf:"telegram"`
	Censor   Censor   `koanf:"censor"`
	Storage  Storage  `koanf:"storage"`
	HTTP     HTTP     `koanf:"http"`
}

func Default() Config {
	//nolint:exhaustruct,mnd // default values
	return Config{
		Bot: Bot{
			BanThreshold: 3,
		},
		Telegram: Telegram{},
		Censor: Censor{
			Blacklist: []string{
				"$",
				"долл",
			},
		},
		Storage: Storage{
			URL: "memory://storage?ttl=5m",
		},
		HTTP: HTTP{
			Address:     "127.0.0.1:3000",
			ProxyHeader: "X-Forwarded-For",
			Proxies:     []string{},
		},
	}
}

func New() (Config, error) {
	cfg := Default()

	if err := config.Load(&cfg); err != nil {
		return Config{}, fmt.Errorf("failed to load config: %w", err)
	}

	return cfg, nil
}
