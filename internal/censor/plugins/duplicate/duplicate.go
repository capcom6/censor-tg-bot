package duplicate

import (
	"context"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

const (
	minTextLength = 3 // Minimum text length for duplicate detection
)

var multiSpaceRegex = regexp.MustCompile(`\s+`)

func Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name: "duplicate",
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
	messageHash, err := p.generateMessageHash(text)
	if err != nil {
		return plugin.Result{}, err
	}

	// Calculate max occurrences (original + allowed duplicates)
	maxOccurrences := p.config.MaxDuplicates + 1

	// Record duplicate and check if limit exceeded
	stat := p.storage.Record(
		msg.ChatID,
		messageHash,
	)

	if stat.Count > maxOccurrences {
		return plugin.Result{
			Action: plugin.ActionBlock,
			Reason: fmt.Sprintf(
				"duplicate limit exceeded (%d occurrences, max %d allowed in %s)",
				stat.Count,
				maxOccurrences,
				p.config.Window,
			),
			Metadata: map[string]any{
				"count":           stat.Count,
				"max_occurrences": maxOccurrences,
				"window":          p.config.Window.String(),
				"message_hash":    messageHash,
			},
			Plugin: p.Name(),
		}, nil
	}

	// Within limits, skip to let other plugins decide
	return plugin.Result{
		Action: plugin.ActionSkip,
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
		return p.normalizeText(text)
	}

	// Fallback to caption if text is empty
	caption := strings.TrimSpace(msg.Caption)
	if caption != "" {
		return p.normalizeText(caption)
	}
	return ""
}

func (p *Plugin) normalizeText(text string) string {
	// Convert to lowercase for case-insensitive matching
	text = strings.ToLower(text)
	// Collapse multiple whitespace characters to single space
	text = multiSpaceRegex.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// generateMessageHash creates a hash for duplicate detection.
func (p *Plugin) generateMessageHash(text string) (string, error) {
	// Generate hash based on text content
	hasher := fnv.New32a()
	if _, err := hasher.Write([]byte(text)); err != nil {
		return "", fmt.Errorf("failed to generate hash: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// Cleanup performs maintenance tasks for the plugin.
// Should be called periodically to clean up expired entries.
func (p *Plugin) Cleanup(_ context.Context) {
	p.storage.Cleanup()
}
