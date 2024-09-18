package main

import (
	"log"

	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func main() {
	cfg := config.Get()

	censor := censor.New(censor.Config{
		Blacklist: cfg.Censor.Blacklist,
	})
	bot, err := tgbotapi.NewBotAPI(cfg.Telegram.Token)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = false

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil && update.EditedMessage == nil {
			continue
		}

		message := update.Message
		if message == nil {
			message = update.EditedMessage
		}

		log.Printf("[%s] %s", message.From.UserName, message.Text)

		ok, err := censor.IsAllow(message.Text)
		if err != nil {
			log.Printf("Error checking message: %s", err)
			continue
		}
		if ok {
			continue
		}

		deleteReq := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)

		if _, err := bot.Request(deleteReq); err != nil {
			log.Printf("Error deleting message: %s", err)
		}

		notifyReq := tgbotapi.NewMessage(cfg.Telegram.AdminID, "Removed message from @"+message.From.UserName+"\n<pre>"+message.Text+"</pre>")
		notifyReq.ParseMode = "HTML"

		if _, err := bot.Send(notifyReq); err != nil {
			log.Printf("Error sending message: %s", err)
		}

		// msg := tgbotapi.NewMessage(message.Chat.ID, message.Text)
		// msg.ReplyToMessageID = message.MessageID

		// if _, err := bot.Send(msg); err != nil {
		// 	log.Printf("Error sending message: %s", err)
		// }
	}
}
