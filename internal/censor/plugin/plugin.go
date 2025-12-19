package plugin

import "context"

// Result represents the decision made by a plugin.
type Result struct {
	Action   Action         // Allow, Block, or Skip
	Reason   string         // Human-readable reason for the decision
	Metadata map[string]any // Additional context (e.g., matched keyword, confidence score)
	Plugin   string         // Plugin name that made the decision
}

// Action represents the plugin's decision.
type Action string

const (
	ActionSkip  Action = "skip"  // Plugin doesn't have an opinion (continue to next plugin)
	ActionAllow Action = "allow" // Allow the message
	ActionBlock Action = "block" // Block the message
)

func (a Action) IsValid() bool {
	return a == ActionSkip || a == ActionAllow || a == ActionBlock
}

// Message contains all inspectable content from a Telegram message.
type Message struct {
	Text                string // Message text
	Caption             string // Message caption (for media)
	UserID              int64  // User ID who sent the message
	ChatID              int64  // Chat ID where message was sent
	MessageID           int    // Message ID
	IsEdit              bool   // Whether this is an edited message
	ForwardedFromUserID *int64 // User ID of original message author (if forwarded)
	ForwardedFromChatID *int64 // Chat ID where original message was sent (if forwarded)
}

// Plugin is the interface that all censor plugins must implement.
type Plugin interface {
	// Name returns the unique identifier for this plugin
	Name() string

	// Evaluate inspects a message and returns a decision
	Evaluate(ctx context.Context, msg Message) (Result, error)

	// Priority returns the execution priority (lower = earlier execution)
	// Useful for short-circuiting expensive operations
	Priority() int
}
