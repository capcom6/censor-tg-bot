package bot

import (
	"fmt"
	"os"

	"github.com/capcom6/censor-tg-bot/internal/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func newAPI(cfg config.Telegram) (*tgbotapi.BotAPI, error) {
	bot, err := tgbotapi.NewBotAPI(cfg.Token)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	bot.Debug = os.Getenv("DEBUG") != ""

	return bot, nil
}
