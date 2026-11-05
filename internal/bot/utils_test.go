package bot_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/capcom6/censor-tg-bot/internal/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"
)

// escapedEntity matches every entity [html.EscapeString] can produce.
var escapedEntity = regexp.MustCompile(`&(?:lt|gt|amp|#39|#34);`)

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

func TestRemovedNotificationEscapesHTML(t *testing.T) {
	t.Parallel()

	alice := &tgbotapi.User{ID: 42, UserName: "alice"}

	got := bot.RemovedNotificationForTest(
		"<dup>",
		"<b>ban</b>",
		&tgbotapi.Message{From: alice, Text: "a <b>bold</b> & \"quoted\""},
		false,
	)

	require.Equal(t,
		"Removed message from @alice\nPlugin: &lt;dup&gt;\nReason: &lt;b&gt;ban&lt;/b&gt;"+
			"\n<pre>a &lt;b&gt;bold&lt;/b&gt; &amp; &#34;quoted&#34;</pre>",
		got,
	)
}

func TestRemovedNotificationStaysWithinTelegramLimit(t *testing.T) {
	t.Parallel()

	alice := &tgbotapi.User{ID: 42, UserName: "alice"}

	tests := []struct {
		name   string
		reason string
		text   string
		copied bool
	}{
		{
			name:   "long text when the copy failed",
			reason: "photo reposted",
			text:   strings.Repeat("<tag ", 1500),
			copied: false,
		},
		{
			name:   "long reason when the copy failed",
			reason: strings.Repeat("bad <content> ", 500),
			text:   strings.Repeat("x", 3000),
			copied: false,
		},
		{
			name:   "long reason when the message was copied",
			reason: strings.Repeat("bad <content> ", 500),
			text:   strings.Repeat("x", 3000),
			copied: true,
		},
		{
			name:   "only markup characters in the text",
			reason: "photo reposted",
			text:   strings.Repeat("<", 3000),
			copied: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := bot.RemovedNotificationForTest(
				"duplicate",
				tt.reason,
				&tgbotapi.Message{From: alice, Text: tt.text},
				tt.copied,
			)

			require.LessOrEqual(t, len(got), 4096)
			require.Contains(t, got, "…")
			requireEntitiesWellFormed(t, got)
		})
	}
}

// TestRemovedNotificationKeepsFallbackWhenReasonIsLong covers CodeRabbit PR
// #33 finding utils.go:67 — a reason that fills its budget must not leave the
// fallback content of a failed copy in an empty <pre> block.
func TestRemovedNotificationKeepsFallbackWhenReasonIsLong(t *testing.T) {
	t.Parallel()

	alice := &tgbotapi.User{ID: 42, UserName: "alice"}

	got := bot.RemovedNotificationForTest(
		"duplicate",
		strings.Repeat("bad <content> ", 500),
		&tgbotapi.Message{From: alice, Text: "blocked text"},
		false,
	)

	require.Contains(t, got, "\n<pre>blocked text</pre>")
	require.LessOrEqual(t, len(got), 4096)
	requireEntitiesWellFormed(t, got)
}

// requireEntitiesWellFormed fails when the notification contains a '&' that
// does not start one of the entities [html.EscapeString] produces, i.e. when
// a truncation cut an entity in half.
func requireEntitiesWellFormed(t *testing.T, s string) {
	t.Helper()

	require.NotContains(t, escapedEntity.ReplaceAllString(s, ""), "&")
}
