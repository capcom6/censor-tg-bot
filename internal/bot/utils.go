package bot

import (
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func userToString(user *tgbotapi.User) string {
	if user.UserName != "" {
		return "@" + user.UserName
	}
	return "<pre>" + strconv.FormatInt(user.ID, 10) + "</pre>"
}
