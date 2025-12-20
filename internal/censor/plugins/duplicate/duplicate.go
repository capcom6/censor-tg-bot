package duplicate

import (
	"context"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

const (
	minTextLength = 3 // Minimum text length for duplicate detection
)

type Plugin struct {
	storage *Storage
	config  Config
}

func New(config Config) plugin.Plugin {
	return &Plugin{
		storage: NewStorage(config.Window),
		config:  config,
	}
}

func (p *Plugin) Name() string {
	return "duplicate"
}

func (p *Plugin) Priority() int {
	const priority = 150
	return priority
}

func (p *Plugin) Evaluate(_ context.Context, msg plugin.Message) (plugin.Result, error) {
	// Get the message text to analyze
	text := p.getMessageText(msg)

	// Skip empty or very short messages
	if len(text) < minTextLength {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "message too short for duplicate detection",
			Metadata: map[string]any{"text_length": len(text)},
			Plugin:   p.Name(),
		}, nil
	}

	// Generate message hash for duplicate detection
	messageHash := p.generateMessageHash(text)

	// Record duplicate and check if limit exceeded
	stat := p.storage.Record(
		msg.ChatID,
		messageHash,
	)

	if stat.Count > p.config.MaxDuplicates {
		return plugin.Result{
			Action: plugin.ActionBlock,
			Reason: fmt.Sprintf(
				"duplicate messages exceeded limit (%d in %s)",
				p.config.MaxDuplicates,
				p.config.Window,
			),
			Metadata: map[string]any{
				"count":          stat.Count,
				"max_duplicates": p.config.MaxDuplicates,
				"window":         p.config.Window.String(),
				"message_hash":   messageHash,
			},
			Plugin: p.Name(),
		}, nil
	}

	// Within limits, allow the message
	return plugin.Result{
		Action: plugin.ActionAllow,
		Reason: "duplicate limit not exceeded",
		Metadata: map[string]any{
			"message_hash": messageHash,
		},
		Plugin: p.Name(),
	}, nil
}

// getMessageText extracts the primary text content from a message.
// Prefers Text over Caption, falling back to Caption if Text is empty.
func (p *Plugin) getMessageText(msg plugin.Message) string {
	text := strings.TrimSpace(msg.Text)
	if text != "" {
		return text
	}

	// Fallback to caption if text is empty
	return strings.TrimSpace(msg.Caption)
}

// generateMessageHash creates a hash for duplicate detection.
func (p *Plugin) generateMessageHash(text string) string {
	// Generate hash based on text content
	hasher := fnv.New32a()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

// Cleanup performs maintenance tasks for the plugin.
// Should be called periodically to clean up expired entries.
func (p *Plugin) Cleanup(_ context.Context) {
	p.storage.Cleanup()
}
