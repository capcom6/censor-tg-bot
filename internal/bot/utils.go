package bot

import (
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// photoLabel marks a photo that really was downloaded and hashed.
	photoLabel = "[photo]"

	// The reasons a photo is present on a message but was never examined. The
	// notification says so explicitly rather than implying a review happened.
	mediaReasonTooLarge    = "too large"
	mediaReasonNotImage    = "unrecognized format"
	mediaReasonFetchFailed = "download failed"
)

// escapeTelegramHTML escapes the characters Telegram's HTML parse mode treats as
// markup. It is the single escape sink for every untrusted string in the admin
// notification.
//
// The supported named entity set is exactly &amp;, &lt;, &gt; and &quot;, verified
// against the upstream Bot API documentation, section "Formatting options / HTML
// style": "The API currently supports only the following named HTML entities: &lt;,
// &gt;, &amp; and &quot;." (https://core.telegram.org/bots/api#html-style).
//
// [html.EscapeString] is deliberately NOT used. It emits &#39; and &#34;, which are
// numeric character references rather than named entities. The same upstream page does
// say "All numerical HTML entities are supported", so those would very likely be
// accepted, but they sit outside the documented named subset and this is not a place
// to find out during an incident. The apostrophe is left LITERAL because no entity is
// needed for a quote inside a text node, which is also the most common character in a
// model's reason.
//
// [strings.Replacer] scans left to right once and never rescans its own output, so an
// escaped ampersand cannot be escaped again.
func escapeTelegramHTML(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
	).Replace(s)
}

func userToString(user *tgbotapi.User) string {
	if user.UserName != "" {
		return "@" + user.UserName
	}

	name := strings.TrimSpace(escapeTelegramHTML(user.FirstName) + " " + escapeTelegramHTML(user.LastName))
	if name == "" {
		name = strconv.FormatInt(user.ID, 10)
	}

	return "<a href=\"tg://user?id=" + strconv.FormatInt(user.ID, 10) + "\">" + name + "</a>"
}

// messageToString renders the message body of the admin notification.
//
// mediaNote is the fetch outcome reported by fetchMedia, and the rendering never
// claims a review that did not happen: "[photo]" appears only when the bytes really
// were downloaded and hashed, and every other outcome says the photo was not examined
// and why. An empty note means no photo was involved at all, which keeps the
// text-only rendering unchanged apart from escaping.
func messageToString(message *tgbotapi.Message, mediaNote string) string {
	switch mediaNote {
	case mediaNoteFetched:
		return photoLabelWithCaption(message)
	case mediaNoteTooLarge:
		return photoNotExamined(mediaReasonTooLarge)
	case mediaNoteNotImage:
		return photoNotExamined(mediaReasonNotImage)
	case mediaNoteFetchFailed:
		return photoNotExamined(mediaReasonFetchFailed)
	}

	if message.Text != "" {
		return escapeTelegramHTML(message.Text)
	}

	if message.Caption != "" {
		return escapeTelegramHTML(message.Caption)
	}

	return ""
}

// photoLabelWithCaption labels a fetched photo, keeping the caption when there is one.
func photoLabelWithCaption(message *tgbotapi.Message) string {
	if message.Caption == "" {
		return photoLabel
	}

	return photoLabel + " " + escapeTelegramHTML(message.Caption)
}

// photoNotExamined renders the label for a photo that is on the message but was not
// sent to any model, so an admin is never told a photo was reviewed when it was not.
func photoNotExamined(reason string) string {
	return "[photo not examined: " + reason + "]"
}
