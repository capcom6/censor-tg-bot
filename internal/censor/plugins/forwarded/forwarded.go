package forwarded

import (
	"context"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/samber/lo"
)

type Plugin struct {
	config Config
}

func New(config Config) plugin.Plugin {
	return &Plugin{
		config: config,
	}
}

func (p *Plugin) Name() string {
	return "forwarded"
}

func (p *Plugin) Priority() int {
	const priority = 15
	return priority
}

func (p *Plugin) Evaluate(_ context.Context, msg plugin.Message) (plugin.Result, error) {
	// Check if the message is forwarded
	if msg.ForwardedFromUserID == nil && msg.ForwardedFromChatID == nil {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "message is not forwarded",
			Metadata: nil,
			Plugin:   p.Name(),
		}, nil
	}

	// Check if forwarded from an allowed user
	if msg.ForwardedFromUserID != nil {
		if lo.Contains(p.config.AllowedUserIDs, *msg.ForwardedFromUserID) {
			return plugin.Result{
				Action: plugin.ActionAllow,
				Reason: "forwarded from allowed user",
				Metadata: map[string]any{
					"forwarded_from_user_id": *msg.ForwardedFromUserID,
				},
				Plugin: p.Name(),
			}, nil
		}
	}

	// Check if forwarded from an allowed chat
	if msg.ForwardedFromChatID != nil {
		if lo.Contains(p.config.AllowedChatIDs, *msg.ForwardedFromChatID) {
			return plugin.Result{
				Action: plugin.ActionAllow,
				Reason: "forwarded from allowed chat",
				Metadata: map[string]any{
					"forwarded_from_chat_id": *msg.ForwardedFromChatID,
				},
				Plugin: p.Name(),
			}, nil
		}
	}

	// Block the forwarded message if not from an allowed source
	return plugin.Result{
		Action: plugin.ActionBlock,
		Reason: "message is forwarded from disallowed source",
		Metadata: map[string]any{
			"forwarded_from_user_id": msg.ForwardedFromUserID,
			"forwarded_from_chat_id": msg.ForwardedFromChatID,
		},
		Plugin: p.Name(),
	}, nil
}

var _ plugin.Plugin = (*Plugin)(nil)
