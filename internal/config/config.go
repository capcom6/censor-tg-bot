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

type Config struct {
	Telegram Telegram
}

var instance = Config{}
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
