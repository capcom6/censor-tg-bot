package plugins

import (
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/duplicate"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/forwarded"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/keyword"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/ratelimit"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/regex"
	"github.com/go-core-fx/logger"
	"go.uber.org/fx"
)

func Module() fx.Option {
	return fx.Module(
		"plugins",
		logger.WithNamedLogger("plugins"),
		fx.Provide(
			fx.Annotate(keyword.New, fx.ResultTags(`group:"plugins"`)),
			fx.Annotate(ratelimit.New, fx.ResultTags(`group:"plugins"`)),
			fx.Annotate(regex.New, fx.ResultTags(`group:"plugins"`)),
			fx.Annotate(forwarded.New, fx.ResultTags(`group:"plugins"`)),
			fx.Annotate(duplicate.New, fx.ResultTags(`group:"plugins"`)),
		),
	)
}
