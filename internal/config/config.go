package config

import (
	"errors"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

type Bot struct {
	AdminID      int64 `envconfig:"BOT__ADMIN_ID" required:"true"`
	BanThreshold int   `envconfig:"BOT__BAN_THRESHOLD" default:"3"`
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

var instance = Config{
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
var once = &sync.Once{}

func Get(logger *zap.Logger) Config {
	once.Do(func() {
		if err := godotenv.Load(); err != nil {
			if !errors.Is(err, os.ErrNotExist) {
				logger.Error("error loading .env file", zap.Error(err))
			}
		}
		envconfig.MustProcess("", &instance)
	})
	return instance
}
