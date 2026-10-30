package bot_test

import (
	"testing"

	"github.com/capcom6/censor-tg-bot/internal/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"
)

func TestRemovedNotification(t *testing.T) {
	t.Parallel()

	const wantBase = "Removed message from @alice\nPlugin: duplicate\nReason: photo reposted"

	alice := &tgbotapi.User{ID: 42, UserName: "alice"}

	tests := []struct {
		name    string
		message *tgbotapi.Message
		copied  bool
		want    string
	}{
		{
			name:    "omits the text when the message was copied",
			message: &tgbotapi.Message{From: alice, Text: "blocked text"},
			copied:  true,
			want:    wantBase,
		},
		{
			name:    "keeps the text as a fallback when the copy failed",
			message: &tgbotapi.Message{From: alice, Text: "blocked text"},
			copied:  false,
			want:    wantBase + "\n<pre>blocked text</pre>",
		},
		{
			name:    "omits the caption when the message was copied",
			message: &tgbotapi.Message{From: alice, Caption: "blocked caption"},
			copied:  true,
			want:    wantBase,
		},
		{
			name:    "keeps the caption as a fallback when the copy failed",
			message: &tgbotapi.Message{From: alice, Caption: "blocked caption"},
			copied:  false,
			want:    wantBase + "\n<pre>blocked caption</pre>",
		},
		{
			name:    "keeps the empty content block when the copy failed",
			message: &tgbotapi.Message{From: alice},
			copied:  false,
			want:    wantBase + "\n<pre></pre>",
		},
		{
			name:    "omits the empty content block when the message was copied",
			message: &tgbotapi.Message{From: alice},
			copied:  true,
			want:    wantBase,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := bot.RemovedNotificationForTest("duplicate", "photo reposted", tt.message, tt.copied)

			require.Equal(t, tt.want, got)
		})
	}
}
