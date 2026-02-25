package users

import (
	"context"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

const (
	DefaultPriority = 5
)

// Plugin implements the user-based blacklist/whitelist plugin.
type Plugin struct {
	config    Config
	blacklist map[int64]struct{}
	whitelist map[int64]struct{}
}

// New creates a new users plugin instance.
func New(config Config) plugin.Plugin {
	toSet := func(ids []int) map[int64]struct{} {
		out := make(map[int64]struct{}, len(ids))
		for _, id := range ids {
			out[int64(id)] = struct{}{}
		}
		return out
	}
	blacklist := toSet(config.Blacklist)
	whitelist := toSet(config.Whitelist)

	return &Plugin{
		config:    config,
		blacklist: blacklist,
		whitelist: whitelist,
	}
}

// Metadata returns the plugin metadata for registration.
func Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name: "users",
		Factory: func(params map[string]any) (plugin.Plugin, error) {
			config, err := NewConfig(params)
			if err != nil {
				return nil, err
			}
			return New(config), nil
		},
	}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "users"
}

// Priority returns the plugin execution priority.
func (p *Plugin) Priority() int {
	// Default priority is 5, which runs before content-based checks
	return DefaultPriority
}

// Evaluate checks if the message sender is in the blacklist or whitelist.
func (p *Plugin) Evaluate(_ context.Context, msg plugin.Message) (plugin.Result, error) {
	userID := msg.UserID

	// Check whitelist first
	if _, ok := p.whitelist[userID]; ok {
		return plugin.Result{
			Action: plugin.ActionAllow,
			Reason: "User is whitelisted",
			Metadata: map[string]any{
				"list": "whitelist",
			},
			Plugin: p.Name(),
		}, nil
	}

	// Check blacklist
	if _, ok := p.blacklist[userID]; ok {
		return plugin.Result{
			Action: plugin.ActionBlock,
			Reason: "User is blacklisted",
			Metadata: map[string]any{
				"list": "blacklist",
			},
			Plugin: p.Name(),
		}, nil
	}

	// User not in any list - skip
	return plugin.Result{
		Action:   plugin.ActionSkip,
		Reason:   "User not in blacklist or whitelist",
		Metadata: nil,
		Plugin:   p.Name(),
	}, nil
}

// Cleanup performs cleanup tasks for the plugin.
func (p *Plugin) Cleanup(_ context.Context) {
	// No cleanup needed for this plugin
}
