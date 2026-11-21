package ratelimit

import (
	"context"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

func Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name: "ratelimit",
		Factory: func(params map[string]any, deps plugin.Deps) (plugin.Plugin, error) {
			config, err := NewConfig(params)
			if err != nil {
				return nil, err
			}

			return New(config, deps), nil
		},
	}
}

type Plugin struct {
	maxMessages int
	window      time.Duration
	storage     *Storage
}

func New(config Config, deps plugin.Deps) plugin.Plugin {
	return &Plugin{
		maxMessages: config.MaxMessages,
		window:      config.Window,
		storage:     NewStorage(deps.Cache),
	}
}

func (p *Plugin) Name() string {
	return "ratelimit"
}

func (p *Plugin) Capabilities() plugin.Capability {
	return plugin.CapabilityText
}

func (p *Plugin) Priority() int {
	const priority = 15
	return priority // Early execution; cheap in-memory counter
}

func (p *Plugin) Evaluate(ctx context.Context, msg plugin.Message) (plugin.Result, error) {
	count, err := p.storage.IncrementAndGet(ctx, msg.UserID, p.window)
	if err != nil {
		return plugin.Result{}, err
	}

	if count > int64(p.maxMessages) {
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

// Cleanup is a no-op: expired counters are recreated by the next message and
// reclaimed by the cachefx sweeper.
func (p *Plugin) Cleanup(context.Context) {
}
