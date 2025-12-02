package bot

import (
	"github.com/capcom6/censor-tg-bot/pkg/tgbotapifx"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"bot",
		logger.WithNamedLogger("bot"),
		fx.Provide(New),
		fx.Invoke(func(bot *Bot, api *tgbotapifx.Bot) {
			api.SetDefaultHandler(bot.Handler)
		}),
	)
}
