package bot

import (
	"context"
	"fmt"
	"strconv"

	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	"github.com/capcom6/censor-tg-bot/pkg/utils/slices"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

type Bot struct {
	config Config

	censor  *censor.Censor
	storage *storage.Storage
	metrics *Metrics

	logger *zap.Logger
}

func New(cfg Config, censor *censor.Censor, storage *storage.Storage, metrics *Metrics, logger *zap.Logger) *Bot {
	return &Bot{
		config:  cfg,
		censor:  censor,
		storage: storage,
		metrics: metrics,
		logger:  logger,
	}
}

func (b *Bot) Handler(_ context.Context, bot *tgbotapifx.Bot, update tgbotapi.Update) error {
	message := slices.FirstNotZero(
		update.Message,
		update.EditedMessage,
		update.ChannelPost,
		update.EditedChannelPost,
	)
	if message == nil {
		return nil
	}

	if err := b.processMessage(bot, message); err != nil {
		b.metrics.IncProcessedAction(MetricLabelActionMessageProcessed, MetricLabelStatusFailed)
		return err
	}

	b.metrics.IncProcessedAction(MetricLabelActionMessageProcessed, MetricLabelStatusSuccess)
	return nil
}

func (b *Bot) processMessage(bot *tgbotapifx.Bot, message *tgbotapi.Message) error {
	if message.From == nil {
		b.logger.Warn("message.From is nil, skipping processing")
		return nil
	}

	ok, err := b.isAllowedMessage(message)
	if err != nil {
		return fmt.Errorf("error checking message: %w", err)
	}
	if ok {
		return nil
	}

	b.logger.Info("message not allowed", zap.Any("message", message))

	deleteReq := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	if _, delErr := bot.Request(deleteReq); delErr != nil {
		b.metrics.IncProcessedAction(MetricLabelActionMessageDeleted, MetricLabelStatusFailed)
		return fmt.Errorf("error deleting message: %w", delErr)
	}
	b.metrics.IncProcessedAction(MetricLabelActionMessageDeleted, MetricLabelStatusSuccess)

	if ntfErr := b.notifyAdmins(bot, "Removed message from "+userToString(message.From)+"\n<pre>"+message.Text+"</pre>"); ntfErr != nil {
		b.metrics.IncProcessedAction(MetricLabelActionAdminNotified, MetricLabelStatusFailed)
		return fmt.Errorf("error notifying admins: %w", ntfErr)
	}
	b.metrics.IncProcessedAction(MetricLabelActionAdminNotified, MetricLabelStatusSuccess)

	cnt, err := b.storage.GetOrSet(strconv.FormatInt(message.From.ID, 10))
	if err != nil {
		b.logger.Warn("error getting violation count", zap.Any("message", message), zap.Error(err))
	}
	b.logger.Info("violation count", zap.Any("message", message), zap.Int("count", cnt))
	if cnt < int(b.config.BanThreshold) {
		return nil
	}

	b.logger.Info("ban user", zap.Any("message", message))

	banReq := tgbotapi.BanChatMemberConfig{
		ChatMemberConfig: tgbotapi.ChatMemberConfig{
			ChatID: message.Chat.ID,
			UserID: message.From.ID,
		},
	}
	if _, banErr := bot.Request(banReq); banErr != nil {
		b.metrics.IncProcessedAction(MetricLabelActionUserBanned, MetricLabelStatusFailed)
		return fmt.Errorf("error banning user: %w", banErr)
	}
	b.metrics.IncProcessedAction(MetricLabelActionUserBanned, MetricLabelStatusSuccess)

	if ntfErr := b.notifyAdmins(bot, "Banned "+userToString(message.From)); ntfErr != nil {
		return fmt.Errorf("error notifying admins: %w", ntfErr)
	}

	return nil
}

func (b *Bot) isAllowedMessage(message *tgbotapi.Message) (bool, error) {
	if message.From == nil {
		return false, nil
	}
	if message.From.ID == b.config.AdminID {
		return true, nil
	}

	if ok, err := b.censor.IsAllow(message.Text); err != nil {
		return false, fmt.Errorf("failed to check text: %w", err)
	} else if !ok {
		return false, nil
	}

	if ok, err := b.censor.IsAllow(message.Caption); err != nil {
		return false, fmt.Errorf("failed to check caption: %w", err)
	} else if !ok {
		return false, nil
	}

	return true, nil
}

func (b *Bot) notifyAdmins(bot *tgbotapifx.Bot, message string) error {
	notifyReq := tgbotapi.NewMessage(b.config.AdminID, message)
	notifyReq.ParseMode = "HTML"

	if _, err := bot.Send(notifyReq); err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	return nil
}
