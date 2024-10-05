package bot

import (
	"fmt"
	"strconv"

	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/config"
	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/capcom6/censor-tg-bot/pkg/utils/slices"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type params struct {
	fx.In

	Config config.Telegram

	Api     *tgbotapi.BotAPI
	Censor  *censor.Censor
	Storage *storage.Storage

	Logger *zap.Logger
}

type bot struct {
	cfg config.Telegram

	api     *tgbotapi.BotAPI
	censor  *censor.Censor
	storage *storage.Storage

	logger *zap.Logger
}

func new(params params) *bot {
	return &bot{
		cfg:     params.Config,
		api:     params.Api,
		censor:  params.Censor,
		storage: params.Storage,
		logger:  params.Logger,
	}
}

func (b *bot) Start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)
	go func() {
		for update := range updates {
			message := slices.FirstNotZero(update.Message, update.EditedMessage, update.ChannelPost, update.EditedChannelPost)
			if message == nil {
				continue
			}

			if err := b.processMessage(*message); err != nil {
				b.logger.Error("error processing message", zap.Any("message", message), zap.Error(err))
			}
		}
	}()
}

func (b *bot) processMessage(message tgbotapi.Message) error {
	if message.From.ID == b.cfg.AdminID {
		return nil
	}

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

	if err := b.notifyAdmins("Removed message from " + userToString(message.From) + "\n<pre>" + message.Text + "</pre>"); err != nil {
		return fmt.Errorf("error notifying admins: %w", err)
	}

	cnt, err := b.storage.GetOrSet(strconv.FormatInt(message.From.ID, 10))
	if err != nil {
		b.logger.Warn("error getting violation count", zap.Any("message", message), zap.Error(err))
	}
	b.logger.Info("violation count", zap.Any("message", message), zap.Int("count", cnt))
	if cnt < b.cfg.BanThreshold {
		return nil
	}

	b.logger.Info("ban user", zap.Any("message", message))

	banReq := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: message.From.ID,
		},
	}
	if _, err := b.api.Request(banReq); err != nil {
		return fmt.Errorf("error banning user: %w", err)
	}

	if err := b.notifyAdmins("Banned " + userToString(message.From)); err != nil {
		return fmt.Errorf("error notifying admins: %w", err)
	}

	return nil
}

func (b *bot) notifyAdmins(message string) error {
	notifyReq := tgbotapi.NewMessage(b.cfg.AdminID, message)
	notifyReq.ParseMode = "HTML"

	if _, err := b.api.Send(notifyReq); err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	return nil
}

func (b *bot) Stop() {
	b.api.StopReceivingUpdates()
}
