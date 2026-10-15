package bot

import (
	"context"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// SelectPhotoSizeForTest exposes selectPhotoSize to the external test package.
func SelectPhotoSizeForTest(photos []tgbotapi.PhotoSize, maxBytes int64) *tgbotapi.PhotoSize {
	return selectPhotoSize(photos, maxBytes)
}

// EscapeTelegramHTMLForTest exposes escapeTelegramHTML to the external test package.
func EscapeTelegramHTMLForTest(s string) string {
	return escapeTelegramHTML(s)
}

// MessageToStringForTest exposes messageToString to the external test package.
func MessageToStringForTest(message *tgbotapi.Message, mediaNote string) string {
	return messageToString(message, mediaNote)
}

// IsParseErrorForTest exposes isParseError to the external test package.
func IsParseErrorForTest(err error) bool {
	return isParseError(err)
}

// EvaluateMessageForTest exposes evaluateMessage to the external test package.
func EvaluateMessageForTest(
	ctx context.Context,
	b *Bot,
	api *tgbotapifx.Bot,
	message *tgbotapi.Message,
) (plugin.Result, string) {
	return b.evaluateMessage(ctx, api, message)
}
