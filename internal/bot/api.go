package bot

import (
	"os"

	"github.com/capcom6/censor-tg-bot/internal/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func newApi(cfg config.Telegram) (*tgbotapi.BotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, err
	}

	bot.Debug = os.Getenv("DEBUG") != ""

	return bot, nil
}
