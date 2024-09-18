package config

import (
	"log"
	"sync"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
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

func Get() Config {
	once.Do(func() {
		if err := godotenv.Load(); err != nil {
			log.Printf("Error loading .env file: %s", err)
		}
		envconfig.MustProcess("", &instance)
	})
	return instance
}
