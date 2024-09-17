package main

import (
	"log"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	cfg := config.Get()

	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.Token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		log.Printf("[%s] %s", update.Message.From.UserName, update.Message.Text)

		if !strings.Contains(update.Message.Text, "$") {
			continue
		}

		deleteReq := tgbotapi.NewDeleteMessage(update.Message.Chat.ID, update.Message.MessageID)

		if _, err := bot.Send(deleteReq); err != nil {
			log.Printf("Error deleting message: %s", err)
		}

		notifyReq := tgbotapi.NewMessage(cfg.Telegram.AdminID, "Removed message from @"+update.Message.From.UserName+"\n<pre>"+update.Message.Text+"</pre>")
		notifyReq.ParseMode = "HTML"

		if _, err := bot.Send(notifyReq); err != nil {
			log.Printf("Error sending message: %s", err)
		}

		// msg := tgbotapi.NewMessage(update.Message.Chat.ID, update.Message.Text)
		// msg.ReplyToMessageID = update.Message.MessageID

		// if _, err := bot.Send(msg); err != nil {
		// 	log.Printf("Error sending message: %s", err)
		// }
	}
}
