package bot

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/censor"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/storage"
	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

type Bot struct {
	config Config

	censor  *censor.Service
	storage *storage.Storage
	metrics *Metrics

	logger *zap.Logger
}

func New(
	cfg Config,
	censor *censor.Service,
	storage *storage.Storage,
	metrics *Metrics,
	logger *zap.Logger,
) (*Bot, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return &Bot{
		config:  cfg,
		censor:  censor,
		storage: storage,
		metrics: metrics,
		logger:  logger,
	}, nil
}

func (b *Bot) Handler(ctx context.Context, bot *tgbotapifx.Bot, update tgbotapi.Update) error {
	message, ok := lo.Find([]*tgbotapi.Message{
		update.Message,
		update.EditedMessage,
		update.ChannelPost,
		update.EditedChannelPost,
	}, lo.IsNotEmpty)
	if !ok {
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
	result := b.evaluateMessage(ctx, bot, message)
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

func (b *Bot) evaluateMessage(ctx context.Context, bot *tgbotapifx.Bot, message *tgbotapi.Message) plugin.Result {
	const pluginName = "bot"

	if message.From == nil {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "message from is nil",
			Metadata: nil,
			Plugin:   pluginName,
		}
	}
	if message.From.ID == b.config.AdminID {
		return plugin.Result{
			Action:   plugin.ActionAllow,
			Reason:   "message from admin",
			Metadata: nil,
			Plugin:   pluginName,
		}
	}

	chatID := int64(0)
	if message.Chat != nil {
		chatID = message.Chat.ID
	}

	var media *plugin.Media
	if b.censor.NeedMedia() {
		var err error
		media, err = b.downloadMedia(ctx, bot, message)
		if err != nil {
			// A failed download is a transport problem, not a moderation
			// decision: the message is neither deleted nor counted as a
			// violation, so a sender is never punished for it.
			b.logger.Error("error downloading media", zap.Error(err))

			return plugin.Result{
				Action:   plugin.ActionSkip,
				Reason:   "error downloading media",
				Metadata: nil,
				Plugin:   pluginName,
			}
		}
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

			Media: media,
		},
	)

	return result
}

func (b *Bot) downloadMedia(
	ctx context.Context,
	bot *tgbotapifx.Bot,
	message *tgbotapi.Message,
) (*plugin.Media, error) {
	if len(message.Photo) == 0 {
		return nil, nil //nolint:nilnil // empty result case
	}

	photo := selectPhotoSize(message.Photo, b.config.Media.MaxBytes)
	if photo == nil {
		b.logger.Warn("no photo variant fits the media limit",
			zap.Int("variants", len(message.Photo)),
			zap.Int64("limit_bytes", b.config.Media.MaxBytes),
		)

		return nil, fmt.Errorf(
			"%w: %d variants, limit %d bytes",
			ErrMediaDownloadFailed,
			len(message.Photo),
			b.config.Media.MaxBytes,
		)
	}

	// The download gets its own budget: this deadline is one term of the per-message
	// worst case media.timeout + censor.Timeout.
	downloadCtx, cancel := context.WithTimeout(ctx, b.config.Media.Timeout)
	defer cancel()

	buffer := bytes.NewBuffer(nil)
	if err := bot.DownloadFile(downloadCtx, photo.FileID, buffer, b.config.Media.MaxBytes); err != nil {
		return nil, fmt.Errorf("error downloading file: %w", err)
	}

	data := buffer.Bytes()

	contentType := http.DetectContentType(data)
	if !strings.HasPrefix(contentType, imageContentTypePrefix) {
		b.logger.Warn("downloaded media is not an image",
			zap.String("content_type", contentType),
			zap.Int("size_bytes", buffer.Len()),
		)

		return nil, fmt.Errorf("%w: detected %s", ErrMediaDownloadFailed, contentType)
	}

	sum := sha256.Sum256(data)

	return &plugin.Media{
		Bytes:       data,
		ContentType: contentType,
		SizeBytes:   len(data),
		SHA256:      hex.EncodeToString(sum[:]),
	}, nil
}

func (b *Bot) notifyAdmins(bot *tgbotapifx.Bot, message string) error {
	notifyReq := tgbotapi.NewMessage(b.config.AdminID, message)
	notifyReq.ParseMode = "HTML"

	if _, err := bot.Send(notifyReq); err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	return nil
}
