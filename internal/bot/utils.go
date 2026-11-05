package bot

import (
	"fmt"
	"html"
	"strconv"
	"strings"
	"unicode/utf8"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const (
	// maxNotificationLen is Telegram's sendMessage text limit; a longer
	// notification is rejected by the API.
	maxNotificationLen = 4096
	// ellipsis marks content dropped by truncate.
	ellipsis = "…"
	// preWrapLen is the length of the "\n<pre></pre>" added around the
	// fallback content.
	preWrapLen = len("\n<pre></pre>")
	// fallbackShareDivisor splits the space left after the header between the
	// reason and the fallback content of a failed copy, so each gets half.
	fallbackShareDivisor = 2
)

func userToString(user *tgbotapi.User) string {
	if user.UserName != "" {
		return "@" + user.UserName
	}

	name := strings.TrimSpace(html.EscapeString(user.FirstName) + " " + html.EscapeString(user.LastName))
	if name == "" {
		name = strconv.FormatInt(user.ID, 10)
	}

	return "<a href=\"tg://user?id=" + strconv.FormatInt(user.ID, 10) + "\">" + name + "</a>"
}

func messageToString(message *tgbotapi.Message) string {
	if message.Text != "" {
		return message.Text
	}

	if message.Caption != "" {
		return message.Caption
	}

	return ""
}

// removedNotification builds the admin notice for a removed message.
//
// The notice is sent with parse_mode=HTML, so plugin, reason and the fallback
// content are escaped while userToString markup and the <pre> tags are kept as
// markup. When the message was copied to the admin the content is omitted; a
// failed copy falls back to the escaped text or caption inside a <pre> block.
// The fallback is budgeted before the reason so a long reason cannot leave an
// empty <pre> block, and the result never exceeds maxNotificationLen bytes.
func removedNotification(plugin, reason string, message *tgbotapi.Message, copied bool) string {
	fixed := fmt.Sprintf(
		"Removed message from %s\nPlugin: %s\nReason: ",
		userToString(message.From),
		html.EscapeString(plugin),
	)

	available := maxNotificationLen - len(fixed)
	reasonBudget := available

	var body string
	if !copied {
		available -= preWrapLen
		body = html.EscapeString(messageToString(message))
		// Reserve space for the fallback content first: a long reason must not
		// leave the admin with an empty <pre> block when the copy failed.
		bodyBudget := min(len(body), available/fallbackShareDivisor)
		body = truncate(body, bodyBudget)
		reasonBudget = available - bodyBudget
	}

	notification := fixed + truncate(html.EscapeString(reason), reasonBudget)

	if !copied {
		notification += "\n<pre>" + body + "</pre>"
	}

	return notification
}

// truncate limits s to maxBytes bytes, appending an ellipsis when content was
// dropped. The cut lands on a rune boundary and never inside a HTML entity,
// so the result stays safe to insert into HTML markup. A non-positive budget
// yields an empty string.
func truncate(s string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(s) <= maxBytes {
		return s
	}

	suffix := ellipsis
	limit := maxBytes - len(suffix)
	if limit <= 0 {
		suffix = ""
		limit = maxBytes
	}

	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	// An '&' left by the cut without its closing ';' starts a partial entity;
	// drop it so the markup cannot break.
	if i := strings.LastIndex(s[:cut], "&"); i >= 0 && !strings.Contains(s[i:cut], ";") {
		cut = i
	}

	return s[:cut] + suffix
}
