package hashers

import (
	"context"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"regexp"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

const minTextLength = 3 // Minimum text length for duplicate detection

var multiSpaceRegex = regexp.MustCompile(`\s+`)

type Text struct {
}

func NewText() *Text {
	return &Text{}
}

func (h *Text) Hash(_ context.Context, msg plugin.Message) (string, error) {
	// Get the message text to analyze
	text := h.getMessageText(msg)

	// Skip empty or very short messages
	if len(text) < minTextLength {
		return "", fmt.Errorf("%w: message too short for duplicate detection", ErrSkipped)
	}

	// Generate message hash for duplicate detection
	messageHash, err := h.generateTextHash(text)
	if err != nil {
		return "", err
	}

	return messageHash, nil
}

// getMessageText extracts the primary text content from a message.
// Prefers Text over Caption, falling back to Caption if Text is empty.
func (h *Text) getMessageText(msg plugin.Message) string {
	text := strings.TrimSpace(msg.Text)
	if text != "" {
		return h.normalizeText(text)
	}

	// Fallback to caption if text is empty
	caption := strings.TrimSpace(msg.Caption)
	if caption != "" {
		return h.normalizeText(caption)
	}
	return ""
}

func (h *Text) normalizeText(text string) string {
	// Convert to lowercase for case-insensitive matching
	text = strings.ToLower(text)
	// Collapse multiple whitespace characters to single space
	text = multiSpaceRegex.ReplaceAllString(text, " ")
	return strings.TrimSpace(text)
}

// generateTextHash creates a hash for duplicate detection.
func (h *Text) generateTextHash(text string) (string, error) {
	// Generate hash based on text content
	hasher := fnv.New32a()
	if _, err := hasher.Write([]byte(text)); err != nil {
		return "", fmt.Errorf("failed to generate hash: %w", err)
	}
	return hex.EncodeToString(hasher.Sum(nil)), nil
}
