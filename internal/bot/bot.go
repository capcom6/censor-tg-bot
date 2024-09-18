package bot

import (
	"fmt"

	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/config"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type params struct {
	fx.In

	Config config.Telegram
	Api    *tgbotapi.BotAPI
	Censor *censor.Censor
	Logger *zap.Logger
}

type bot struct {
	cfg    config.Telegram
	api    *tgbotapi.BotAPI
	censor *censor.Censor
	logger *zap.Logger
}

func new(params params) *bot {
	return &bot{
		cfg:    params.Config,
		api:    params.Api,
		censor: params.Censor,
		logger: params.Logger,
	}
}

func (b *bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)
	go func() {
		for update := range updates {
			if update.Message == nil && update.EditedMessage == nil {
				continue
			}

			message := update.Message
			if message == nil {
				message = update.EditedMessage
			}

			if err := b.processMessage(*message); err != nil {
				b.logger.Error("error processing message", zap.Any("message", message), zap.Error(err))
			}
		}
	}()
}

func (b *bot) processMessage(message tgbotapi.Message) error {
	ok, err := b.censor.IsAllow(message.Text)
	if err != nil {
		return fmt.Errorf("censor error: %w", err)
	}
	if ok {
		return nil
	}

	b.logger.Info("message not allowed", zap.Any("message", message))

	deleteReq := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)

	if _, err := b.api.Request(deleteReq); err != nil {
		return fmt.Errorf("error deleting message: %w", err)
	}

	notifyReq := tgbotapi.NewMessage(b.cfg.AdminID, "Removed message from @"+message.From.UserName+"\n<pre>"+message.Text+"</pre>")
	notifyReq.ParseMode = "HTML"

	if _, err := b.api.Send(notifyReq); err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	return nil
}

func (b *bot) Stop() {
	b.api.StopReceivingUpdates()
}
