package keyword

import (
	"context"
	"regexp"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/samber/lo"
)

type Plugin struct {
	blacklist []string
	filter    *regexp.Regexp
}

func New(config Config) plugin.Plugin {
	return &Plugin{
		blacklist: lo.Map(
			config.Blacklist,
			func(s string, _ int) string {
				return strings.ToLower(s)
			},
		),
		filter: regexp.MustCompile(`[^\p{Cyrillic}\p{Latin}][:graph:]`),
	}
}

func (p *Plugin) Name() string {
	return "keyword"
}

func (p *Plugin) Priority() int {
	const priority = 10
	return priority // Low priority = early execution
}

func (p *Plugin) Evaluate(_ context.Context, msg plugin.Message) (plugin.Result, error) {
	text := p.normalizeText(msg.Text)
	if text == "" {
		text = p.normalizeText(msg.Caption)
	}

	if text == "" {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "empty message",
			Metadata: nil,
			Plugin:   p.Name(),
		}, nil
	}

	for _, word := range p.blacklist {
		if strings.Contains(text, word) {
			return plugin.Result{
				Action: plugin.ActionBlock,
				Reason: "Message contains blacklisted keyword",
				Metadata: map[string]any{
					"keyword": word,
				},
				Plugin: p.Name(),
			}, nil
		}
	}

	return plugin.Result{
		Action:   plugin.ActionSkip,
		Reason:   "no blacklisted keywords found",
		Metadata: nil,
		Plugin:   p.Name(),
	}, nil
}

func (p *Plugin) normalizeText(text string) string {
	if text == "" {
		return ""
	}
	text = p.filter.ReplaceAllString(text, "")
	return strings.ToLower(text)
}

var _ plugin.Plugin = (*Plugin)(nil)
