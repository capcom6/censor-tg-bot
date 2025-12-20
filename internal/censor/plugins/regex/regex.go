package regex

import (
	"context"
	"regexp"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

type Plugin struct {
	patterns []*regexp.Regexp
}

func New(config Config) (plugin.Plugin, error) {
	return &Plugin{
		patterns: config.Patterns,
	}, nil
}

func (p *Plugin) Name() string {
	return "regex"
}

func (p *Plugin) Priority() int {
	const priority = 20
	return priority
}

func (p *Plugin) Evaluate(_ context.Context, msg plugin.Message) (plugin.Result, error) {
	text := msg.Text
	if text == "" {
		text = msg.Caption
	}

	if text == "" {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "empty message",
			Metadata: nil,
			Plugin:   p.Name(),
		}, nil
	}

	for _, pattern := range p.patterns {
		if pattern.MatchString(text) {
			return plugin.Result{
				Action: plugin.ActionBlock,
				Reason: "Message matches forbidden pattern",
				Metadata: map[string]any{
					"pattern": pattern.String(),
				},
				Plugin: p.Name(),
			}, nil
		}
	}

	return plugin.Result{
		Action:   plugin.ActionSkip,
		Reason:   "no forbidden patterns found",
		Metadata: nil,
		Plugin:   p.Name(),
	}, nil
}

// Cleanup implements plugin.Plugin.
func (p *Plugin) Cleanup(_ context.Context) {
	// no-op
}
