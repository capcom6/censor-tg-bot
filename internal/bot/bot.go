package bot

import (
	"context"
	"fmt"
	"strconv"

	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	"github.com/capcom6/censor-tg-bot/pkg/utils/slices"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"go.uber.org/zap"
)

type Bot struct {
	config Config

	censor  *censor.Service
	storage *storage.Storage
	metrics *Metrics

	logger *zap.Logger
}

func New(cfg Config, censor *censor.Service, storage *storage.Storage, metrics *Metrics, logger *zap.Logger) *Bot {
	return &Bot{
		config:  cfg,
		censor:  censor,
		storage: storage,
		metrics: metrics,
		logger:  logger,
	}
}

func (b *Bot) Handler(ctx context.Context, bot *tgbotapifx.Bot, update tgbotapi.Update) error {
	message := slices.FirstNotZero(
		update.Message,
		update.EditedMessage,
		update.ChannelPost,
		update.EditedChannelPost,
	)
	if message == nil {
		return nil
	}

	if err := b.processMessage(ctx, bot, message); err != nil {
		b.metrics.IncProcessedAction(MetricLabelActionMessageProcessed, MetricLabelStatusFailed)
		return err
	}

	b.metrics.IncProcessedAction(MetricLabelActionMessageProcessed, MetricLabelStatusSuccess)
	return nil
}

func (b *Bot) processMessage(ctx context.Context, bot *tgbotapifx.Bot, message *tgbotapi.Message) error {
	result := b.evaluateMessage(ctx, message)
	if result.Action != plugin.ActionBlock {
		return nil
	}

	b.logger.Info("message blocked",
		zap.String("plugin", result.Plugin),
		zap.String("reason", result.Reason),
		zap.Any("metadata", result.Metadata),
		zap.Any("message", message),
	)

	deleteReq := tgbotapi.NewDeleteMessage(message.Chat.ID, message.MessageID)
	if _, delErr := bot.Request(deleteReq); delErr != nil {
		b.metrics.IncProcessedAction(MetricLabelActionMessageDeleted, MetricLabelStatusFailed)
		return fmt.Errorf("error deleting message: %w", delErr)
	}
	b.metrics.IncProcessedAction(MetricLabelActionMessageDeleted, MetricLabelStatusSuccess)

	// Enhanced admin notification with plugin details
	notification := fmt.Sprintf(
		"Removed message from %s\nPlugin: %s\nReason: %s\n<pre>%s</pre>",
		userToString(message.From),
		result.Plugin,
		result.Reason,
		messageToString(message),
	)
	if ntfErr := b.notifyAdmins(bot, notification); ntfErr != nil {
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

func (b *Bot) evaluateMessage(ctx context.Context, message *tgbotapi.Message) plugin.Result {
	if message.From == nil {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "message from is nil",
			Metadata: nil,
			Plugin:   "bot",
		}
	}
	if message.From.ID == b.config.AdminID {
		return plugin.Result{
			Action:   plugin.ActionAllow,
			Reason:   "message from admin",
			Metadata: nil,
			Plugin:   "bot",
		}
	}

	chatID := int64(0)
	if message.Chat != nil {
		chatID = message.Chat.ID
	}

	result := b.censor.Evaluate(
		ctx,
		plugin.Message{
			Text:      message.Text,
			Caption:   message.Caption,
			UserID:    message.From.ID,
			ChatID:    chatID,
			MessageID: message.MessageID,
			IsEdit:    message.EditDate != 0,
			ForwardedFromUserID: func() *int64 {
				if message.ForwardFrom != nil {
					return &message.ForwardFrom.ID
				}
				return nil
			}(),
			ForwardedFromChatID: func() *int64 {
				if message.ForwardFromChat != nil {
					return &message.ForwardFromChat.ID
				}
				return nil
			}(),
		},
	)

	return result
}

func (b *Bot) notifyAdmins(bot *tgbotapifx.Bot, message string) error {
	notifyReq := tgbotapi.NewMessage(b.config.AdminID, message)
	notifyReq.ParseMode = "HTML"

	if _, err := bot.Send(notifyReq); err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	return nil
}
