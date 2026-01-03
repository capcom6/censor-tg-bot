package plugins

import (
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/duplicate"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/forwarded"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/keyword"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/llm"
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
			fx.Annotate(keyword.Metadata, fx.ResultTags(`group:"metadata"`)),
			fx.Annotate(ratelimit.Metadata, fx.ResultTags(`group:"metadata"`)),
			fx.Annotate(regex.Metadata, fx.ResultTags(`group:"metadata"`)),
			fx.Annotate(forwarded.Metadata, fx.ResultTags(`group:"metadata"`)),
			fx.Annotate(duplicate.Metadata, fx.ResultTags(`group:"metadata"`)),
			fx.Annotate(llm.Metadata, fx.ResultTags(`group:"metadata"`)),
		),
	)
}
