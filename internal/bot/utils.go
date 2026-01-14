package bot

import (
	"html"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
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
