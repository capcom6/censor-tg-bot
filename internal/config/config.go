package config

import (
	"errors"
	"os"
	"sync"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
)

type Telegram struct {
	Token   string `envconfig:"TELEGRAM__TOKEN" required:"true"`
	AdminID int64  `envconfig:"TELEGRAM__ADMIN_ID" required:"true"`
}

type Censor struct {
	Blacklist []string `envconfig:"CENSOR__BLACKLIST"`
}

type Config struct {
	Telegram Telegram
	Censor   Censor
}

var instance = Config{
	Telegram: Telegram{},
	Censor: Censor{
		Blacklist: []string{
			"$",
			"долл",
		},
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
