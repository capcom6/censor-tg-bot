package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

type Bot struct {
	AdminID      int64 `envconfig:"BOT__ADMIN_ID"      required:"true"`
	BanThreshold int   `envconfig:"BOT__BAN_THRESHOLD"                 default:"3"`
}

type Telegram struct {
	Bot

	Token string `envconfig:"TELEGRAM__TOKEN" required:"true"`
}

type Censor struct {
	Blacklist []string `envconfig:"CENSOR__BLACKLIST"`
}

type Storage struct {
	URL string `envconfig:"STORAGE__URL"`
}

type Config struct {
	Telegram Telegram
	Censor   Censor
	Storage  Storage
}

func Default() Config {
	//nolint:exhaustruct // default values
	return Config{
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
	}
}

func Get(logger *zap.Logger) Config {
	config := Default()

	if err := godotenv.Load(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			logger.Error("error loading .env file", zap.Error(err))
		}
	}

	if err := envconfig.Process("", &config); err != nil {
		logger.Error("error loading environment variables", zap.Error(err))
	}

	return config
}
