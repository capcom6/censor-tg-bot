package bot

import (
	"context"
	"fmt"
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

// processMessage runs the delete -> notify -> count -> ban chain for a blocked
// message. evaluateMessage yields the decision together with the media note, and the
// note is handed to the renderer as data so it can label the photo honestly.
func (b *Bot) processMessage(ctx context.Context, bot *tgbotapifx.Bot, message *tgbotapi.Message) error {
	result, note := b.evaluateMessage(ctx, bot, message)
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

	// Enhanced admin notification with plugin details. The reason is model-authored
	// text describing attacker-supplied image content, which is the classic route for
	// image-borne prompt injection, so it is escaped like any other untrusted string.
	notification := fmt.Sprintf(
		"Removed message from %s\nPlugin: %s\nReason: %s\n<pre>%s</pre>",
		userToString(message.From),
		result.Plugin,
		escapeTelegramHTML(result.Reason),
		messageToString(message, note),
	)

	// A failed notification must never stop the chain. It used to return here, BEFORE
	// the violation count and the ban, so a single unescaped entity in a reason
	// deleted the message without ever counting it. notifyAdmins records and swallows
	// its own failures; the worst case left is a silent admin.
	b.notifyAdmins(bot, notification)

	// One message counts as ONE violation, however many times it is edited: Handler
	// re-evaluates edits and a cached Block on an edit would otherwise re-increment
	// the counter, making ban escalation reachable through edit spam. The msg: prefix
	// cannot collide with the bare decimal user ids stored below.
	if counted := b.countMessage(message); !counted {
		return nil
	}

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

	b.notifyAdmins(bot, "Banned "+userToString(message.From))

	return nil
}

// messageViolationKey namespaces the one-violation-per-message marker in the shared
// storage. The existing keys are bare decimal user ids, so a prefix can never collide
// with one of them.
func messageViolationKey(chatID int64, messageID int) string {
	return "msg:" + strconv.FormatInt(chatID, 10) + ":" + strconv.Itoa(messageID)
}

// countMessage reports whether this message is being counted for the first time. A
// storage failure is reported as counted: dropping the count would let a broken
// storage silently disable the ban, which is the more dangerous direction.
func (b *Bot) countMessage(message *tgbotapi.Message) bool {
	key := messageViolationKey(message.Chat.ID, message.MessageID)

	first, err := b.storage.GetOrSet(key)
	if err != nil {
		b.logger.Warn("error counting message", zap.String("key", key), zap.Error(err))

		return true
	}

	return first == 1
}

// pluginName is the plugin name reported for the decisions this module makes itself,
// as opposed to the ones a censor plugin makes.
const pluginName = "bot"

// evaluateMessage decides what happens to a message and returns the media note
// describing the fetch outcome, or "" when no photo was involved at all.
func (b *Bot) evaluateMessage(
	ctx context.Context,
	bot *tgbotapifx.Bot,
	message *tgbotapi.Message,
) (plugin.Result, string) {
	if message.From == nil {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "message from is nil",
			Metadata: nil,
			Plugin:   pluginName,
		}, ""
	}
	if message.From.ID == b.config.AdminID {
		return plugin.Result{
			Action:   plugin.ActionAllow,
			Reason:   "message from admin",
			Metadata: nil,
			Plugin:   pluginName,
		}, ""
	}

	chatID := int64(0)
	if message.Chat != nil {
		chatID = message.Chat.ID
	}

	media, mediaNote, mediaErr := b.fetchMedia(ctx, bot, message)

	// Failing to obtain the image is fail-closed: the block is returned from here so
	// that processMessage runs its existing delete -> notify -> count -> ban chain
	// unchanged, with no new execution path. censor.Evaluate is deliberately NOT
	// called on this path - the message is being deleted anyway, so there is nothing
	// left for the text plugins to decide, and running them would only cost time.
	if mediaErr != nil && !isMediaAbsence(mediaErr) {
		return plugin.Result{
			Action:   plugin.ActionBlock,
			Reason:   "telegram photo download failed: " + mediaErr.Error(),
			Metadata: map[string]any{"media": mediaNote, "error": mediaErr.Error()},
			Plugin:   pluginName,
		}, mediaNote
	}

	// An absent image is not a verdict. The message goes to the plugins exactly as
	// before, carrying a nil Media so only a really downloaded image reaches them.
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

	return result, mediaNote
}

// notifyAdmins sends one notification to the admin and records the outcome. It returns
// NOTHING, so a failed notification is structurally incapable of aborting the
// violation count or the ban that follow it.
//
// An HTML parse or entity failure is retried ONCE with ParseMode unset, so an
// unescaped entity degrades the notification to plain text instead of deleting the
// message without counting it. If the retry also fails the outcome is logged at Warn
// and counted as failed. This is the only defence that protects the ban chain against
// a future escaping bug, because it does not depend on the escaper being complete.
func (b *Bot) notifyAdmins(bot *tgbotapifx.Bot, message string) {
	htmlReq := tgbotapi.NewMessage(b.config.AdminID, message)
	htmlReq.ParseMode = parseModeHTML

	_, err := bot.Send(htmlReq)
	if err == nil {
		b.metrics.IncProcessedAction(MetricLabelActionAdminNotified, MetricLabelStatusSuccess)

		return
	}

	if !isParseError(err) {
		b.metrics.IncProcessedAction(MetricLabelActionAdminNotified, MetricLabelStatusFailed)
		// The send uses the same token-bearing transport as every other Telegram call,
		// so the error is stripped before it can reach the log.
		b.logger.Warn("error sending admin notification", zap.Error(tgbotapifx.RedactURLError(err)))

		return
	}

	b.logger.Warn("admin notification rejected as HTML, retrying as plain text",
		zap.Error(tgbotapifx.RedactURLError(err)),
	)

	plainReq := tgbotapi.NewMessage(b.config.AdminID, message)

	if _, retryErr := bot.Send(plainReq); retryErr != nil {
		b.metrics.IncProcessedAction(MetricLabelActionAdminNotified, MetricLabelStatusFailed)
		b.logger.Warn("error sending admin notification as plain text",
			zap.Error(tgbotapifx.RedactURLError(retryErr)),
		)

		return
	}

	b.metrics.IncProcessedAction(MetricLabelActionAdminNotified, MetricLabelStatusSuccess)
}

// parseModeHTML is the Telegram parse mode of the notification built here.
const parseModeHTML = "HTML"

// isParseError reports whether err is a markup rejection rather than a transport or
// authorisation failure. Only a markup rejection is worth retrying as plain text;
// retrying every failure would double the send volume during an outage.
//
// The markers are the substrings Telegram uses when it rejects a message because its
// markup could not be parsed, matched case-insensitively against the lowercased error
// text.
func isParseError(err error) bool {
	if err == nil {
		return false
	}

	markers := []string{
		"can't parse entities",
		"can't find end of the entity",
		"unsupported start tag",
		"entity parsing failed",
	}

	text := strings.ToLower(err.Error())
	for _, marker := range markers {
		if strings.Contains(text, marker) {
			return true
		}
	}

	return false
}
