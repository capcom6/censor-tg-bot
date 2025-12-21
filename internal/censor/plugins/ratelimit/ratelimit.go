package ratelimit

import (
	"context"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

func Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name: "ratelimit",
		Factory: func(params map[string]any) (plugin.Plugin, error) {
			config, err := NewConfig(params)
			if err != nil {
				return nil, err
			}

			return New(config), nil
		},
	}
}

type Plugin struct {
	maxMessages int
	window      time.Duration
	storage     *Storage
}

func New(config Config) plugin.Plugin {
	return &Plugin{
		maxMessages: config.MaxMessages,
		window:      config.Window,
		storage:     NewStorage(),
	}
}

func (p *Plugin) Name() string {
	return "ratelimit"
}

func (p *Plugin) Priority() int {
	const priority = 5
	return priority // Very high priority (early execution)
}

func (p *Plugin) Evaluate(_ context.Context, msg plugin.Message) (plugin.Result, error) {
	count, err := p.storage.IncrementAndGet(msg.UserID, p.window)
	if err != nil {
		return plugin.Result{}, err
	}

	if count > p.maxMessages {
		return plugin.Result{
			Action: plugin.ActionBlock,
			Reason: "Rate limit exceeded",
			Metadata: map[string]any{
				"count": count,
				"limit": p.maxMessages,
			},
			Plugin: p.Name(),
		}, nil
	}

	return plugin.Result{
		Action:   plugin.ActionSkip,
		Reason:   "rate limit not exceeded",
		Metadata: nil,
		Plugin:   p.Name(),
	}, nil
}

// Cleanup implements plugin.Plugin.
func (p *Plugin) Cleanup(_ context.Context) {
	p.storage.Cleanup()
}
