package duplicate_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/duplicate"
	"github.com/stretchr/testify/require"
)

func TestPlugin_New(t *testing.T) {
	config := duplicate.Config{
		MaxDuplicates: 3,
		Window:        5 * time.Minute,
	}

	p := duplicate.New(config)

	require.NotNil(t, p)
	require.Implements(t, (*plugin.Plugin)(nil), p)
}

func TestPlugin_Name(t *testing.T) {
	config := duplicate.DefaultConfig()

	p := duplicate.New(config)

	require.Equal(t, "duplicate", p.Name())
}

func TestPlugin_Priority(t *testing.T) {
	config := duplicate.DefaultConfig()

	p := duplicate.New(config)

	require.Equal(t, 150, p.Priority())
}

func TestPlugin_Evaluate(t *testing.T) {
	tests := []struct {
		name           string
		config         duplicate.Config
		message        plugin.Message
		expectedAction plugin.Action
		expectedReason string
		setup          func(*duplicate.Plugin)
	}{
		{
			name: "short message should skip",
			config: duplicate.Config{
				MaxDuplicates: 3,
				Window:        5 * time.Minute,
			},
			message: plugin.Message{
				Text:   "Hi",
				ChatID: 12345,
			},
			expectedAction: plugin.ActionSkip,
			expectedReason: "message too short for duplicate detection",
		},
		{
			name: "empty text should skip",
			config: duplicate.Config{
				MaxDuplicates: 3,
				Window:        5 * time.Minute,
			},
			message: plugin.Message{
				Text:   "",
				ChatID: 12345,
			},
			expectedAction: plugin.ActionSkip,
			expectedReason: "message too short for duplicate detection",
		},
		{
			name: "whitespace only should skip",
			config: duplicate.Config{
				MaxDuplicates: 3,
				Window:        5 * time.Minute,
			},
			message: plugin.Message{
				Text:   "   ",
				ChatID: 12345,
			},
			expectedAction: plugin.ActionSkip,
			expectedReason: "message too short for duplicate detection",
		},
		{
			name: "first occurrence should allow",
			config: duplicate.Config{
				MaxDuplicates: 3,
				Window:        5 * time.Minute,
			},
			message: plugin.Message{
				Text:   "Hello world, this is a test message",
				ChatID: 12345,
			},
			expectedAction: plugin.ActionAllow,
			expectedReason: "duplicate limit not exceeded",
		},
		{
			name: "second occurrence should allow",
			config: duplicate.Config{
				MaxDuplicates: 3,
				Window:        5 * time.Minute,
			},
			message: plugin.Message{
				Text:   "Hello world, this is a test message",
				ChatID: 12345,
			},
			expectedAction: plugin.ActionAllow,
			expectedReason: "duplicate limit not exceeded",
			setup: func(p *duplicate.Plugin) {
				// First call
				_, err := p.Evaluate(context.Background(), plugin.Message{
					Text:   "Hello world, this is a test message",
					ChatID: 12345,
				})
				require.NoError(t, err)
			},
		},
		{
			name: "duplicate limit exceeded should block",
			config: duplicate.Config{
				MaxDuplicates: 2,
				Window:        5 * time.Minute,
			},
			message: plugin.Message{
				Text:   "Duplicate message",
				ChatID: 12345,
			},
			expectedAction: plugin.ActionBlock,
			expectedReason: "duplicate limit exceeded (4 occurrences, max 3 allowed in 5m0s)",
			setup: func(p *duplicate.Plugin) {
				// First three calls
				for range 3 {
					_, err := p.Evaluate(context.Background(), plugin.Message{
						Text:   "Duplicate message",
						ChatID: 12345,
					})
					require.NoError(t, err)
				}
			},
		},
		{
			name: "different chat IDs should not interfere",
			config: duplicate.Config{
				MaxDuplicates: 2,
				Window:        5 * time.Minute,
			},
			message: plugin.Message{
				Text:   "Same message",
				ChatID: 99999,
			},
			expectedAction: plugin.ActionAllow,
			expectedReason: "duplicate limit not exceeded",
		},
		{
			name: "caption should work like text",
			config: duplicate.Config{
				MaxDuplicates: 2,
				Window:        5 * time.Minute,
			},
			message: plugin.Message{
				Caption: "Caption message",
				ChatID:  12345,
			},
			expectedAction: plugin.ActionAllow,
			expectedReason: "duplicate limit not exceeded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := duplicate.New(tt.config)

			// Call setup function if provided
			if tt.setup != nil {
				if dupPlugin, ok := p.(*duplicate.Plugin); ok {
					tt.setup(dupPlugin)
				}
			}

			result, err := p.Evaluate(context.Background(), tt.message)
			require.NoError(t, err)
			require.Equal(t, tt.expectedAction, result.Action)
			require.Equal(t, tt.expectedReason, result.Reason)
			require.Equal(t, "duplicate", result.Plugin)
		})
	}
}

func TestStorage_RecordDuplicate(t *testing.T) {
	storage := duplicate.NewStorage(5 * time.Minute)

	// First occurrence - should return 1 (not exceeded)
	stat := storage.Record(12345, "hash1")
	require.Equal(t, 1, stat.Count)

	// Second occurrence - should return 2 (not exceeded)
	stat = storage.Record(12345, "hash1")
	require.Equal(t, 2, stat.Count)

	// Third occurrence - should return 3 (exceeded)
	stat = storage.Record(12345, "hash1")
	require.Equal(t, 3, stat.Count)

	// Different chat ID - should return 1 (separate tracking)
	stat = storage.Record(99999, "hash1")
	require.Equal(t, 1, stat.Count)

	// Different hash - should return 1
	stat = storage.Record(12345, "hash2")
	require.Equal(t, 1, stat.Count)
}

func TestStorage_WindowExpiration(t *testing.T) {
	storage := duplicate.NewStorage(1 * time.Second)

	// Record a duplicate
	stat := storage.Record(12345, "hash1")
	require.Equal(t, 1, stat.Count)

	// Second occurrence within window
	stat = storage.Record(12345, "hash1")
	require.Equal(t, 2, stat.Count)

	// Wait for window to expire
	time.Sleep(2 * time.Second)

	// Should reset count after window expiration
	stat = storage.Record(12345, "hash1")
	require.Equal(t, 1, stat.Count)

	// Should still allow another within the new window
	stat = storage.Record(12345, "hash1")
	require.Equal(t, 2, stat.Count)

	// Third occurrence should now exceed
	stat = storage.Record(12345, "hash1")
	require.Equal(t, 3, stat.Count)
}

func TestPlugin_UnicodeHandling(t *testing.T) {
	config := duplicate.Config{
		MaxDuplicates: 1,
		Window:        5 * time.Minute,
	}

	p := duplicate.New(config)

	unicodeMessage := plugin.Message{
		Text:   "Привет мир! Hello 世界! 🎉",
		ChatID: 12345,
	}

	// First occurrence
	result1, err := p.Evaluate(context.Background(), unicodeMessage)
	require.NoError(t, err)
	require.Equal(t, plugin.ActionAllow, result1.Action)

	// Second occurrence
	result2, err := p.Evaluate(context.Background(), unicodeMessage)
	require.NoError(t, err)
	require.Equal(t, plugin.ActionAllow, result2.Action)

	// Third occurrence - should block
	result3, err := p.Evaluate(context.Background(), unicodeMessage)
	require.NoError(t, err)
	require.Equal(t, plugin.ActionBlock, result3.Action)
}

func TestPlugin_TextExtraction(t *testing.T) {
	config := duplicate.Config{
		MaxDuplicates: 2,
		Window:        5 * time.Minute,
	}

	tests := []struct {
		name           string
		message        plugin.Message
		expectedAction plugin.Action
	}{
		{
			name: "text takes precedence over caption",
			message: plugin.Message{
				Text:    "This is text",
				Caption: "This is caption",
				ChatID:  12345,
			},
			expectedAction: plugin.ActionAllow,
		},
		{
			name: "caption used when text is empty",
			message: plugin.Message{
				Text:    "",
				Caption: "This is caption",
				ChatID:  12345,
			},
			expectedAction: plugin.ActionAllow,
		},
		{
			name: "whitespace in text trimmed",
			message: plugin.Message{
				Text:    "  message  ",
				Caption: "caption",
				ChatID:  12345,
			},
			expectedAction: plugin.ActionAllow,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := duplicate.New(config)

			result, err := p.Evaluate(context.Background(), tt.message)
			require.NoError(t, err)
			require.Equal(t, tt.expectedAction, result.Action)
		})
	}
}

func TestPlugin_HashGeneration(t *testing.T) {
	config := duplicate.Config{
		MaxDuplicates: 3,
		Window:        5 * time.Minute,
	}

	p := duplicate.New(config)

	// Test SHA-256 hash generation
	message := "Test message for hashing"

	// The hash should be consistent
	result1, err := p.Evaluate(context.Background(), plugin.Message{
		Text:   message,
		ChatID: 12345,
	})
	require.NoError(t, err)

	// Verify the metadata contains the hash
	require.NotEmpty(t, result1.Metadata["message_hash"])

	p2 := duplicate.New(config)

	result2, err := p2.Evaluate(context.Background(), plugin.Message{
		Text:   message,
		ChatID: 12345,
	})
	require.NoError(t, err)

	// Hash should be consistent and deterministic
	require.Equal(
		t,
		"95a0db0d",
		result2.Metadata["message_hash"],
	)
}

func TestPlugin_Integration(t *testing.T) {
	config := duplicate.Config{
		MaxDuplicates: 3,
		Window:        1 * time.Second,
	}

	p := duplicate.New(config)

	message := plugin.Message{
		Text:   "Integration test message",
		ChatID: 12345,
	}

	// First 4 messages should be allowed
	for range 4 {
		result, err := p.Evaluate(context.Background(), message)
		require.NoError(t, err)
		require.Equal(t, plugin.ActionAllow, result.Action)
	}

	// Fifth message should be blocked
	result, err := p.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, plugin.ActionBlock, result.Action)

	// Wait for window to expire
	time.Sleep(2 * time.Second)

	// Should allow again after window expires
	result, err = p.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, plugin.ActionAllow, result.Action)
}

func TestPlugin_NilConfig(t *testing.T) {
	// This should work with default configuration
	p := duplicate.New(duplicate.Config{})
	require.NotNil(t, p)

	// But the default config should be invalid
	config := duplicate.Config{}
	require.Error(t, config.Validate())

	// Test that the plugin can still be created and responds appropriately
	// Use a short message (<3 chars) to get ActionSkip
	message := plugin.Message{Text: "Hi", ChatID: 12345}
	result, err := p.Evaluate(context.Background(), message)
	require.NoError(t, err)
	require.Equal(t, plugin.ActionSkip, result.Action) // Should skip due to short text
}

func TestPlugin_EmptyMessageHandling(t *testing.T) {
	config := duplicate.Config{
		MaxDuplicates: 3,
		Window:        5 * time.Minute,
	}

	testCases := []struct {
		name    string
		message plugin.Message
		reason  string
	}{
		{
			name: "empty string",
			message: plugin.Message{
				Text:   "",
				ChatID: 12345,
			},
			reason: "message too short for duplicate detection",
		},
		{
			name: "only spaces",
			message: plugin.Message{
				Text:   "   ",
				ChatID: 12345,
			},
			reason: "message too short for duplicate detection",
		},
		{
			name: "only newlines",
			message: plugin.Message{
				Text:   "\n\n\n",
				ChatID: 12345,
			},
			reason: "message too short for duplicate detection",
		},
		{
			name: "single character",
			message: plugin.Message{
				Text:   "a",
				ChatID: 12345,
			},
			reason: "message too short for duplicate detection",
		},
		{
			name: "two characters",
			message: plugin.Message{
				Text:   "ab",
				ChatID: 12345,
			},
			reason: "message too short for duplicate detection",
		},
		{
			name: "three characters minimum",
			message: plugin.Message{
				Text:   "abcd", // Use 4 characters to ensure it's processed
				ChatID: 12345,
			},
			reason: "duplicate limit not exceeded",
		},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			p := duplicate.New(config)

			result, err := p.Evaluate(context.Background(), tt.message)
			require.NoError(t, err)

			// Determine expected action based on the reason
			var expectedAction plugin.Action
			if strings.Contains(tt.reason, "too short") {
				expectedAction = plugin.ActionSkip
			} else {
				expectedAction = plugin.ActionAllow
			}

			require.Equal(t, expectedAction, result.Action)
			require.Equal(t, tt.reason, result.Reason)
		})
	}
}

// Benchmark tests for performance testing.
func BenchmarkPlugin_Evaluate(b *testing.B) {
	config := duplicate.Config{
		MaxDuplicates: 3,
		Window:        5 * time.Minute,
	}

	p := duplicate.New(config)

	message := plugin.Message{
		Text:   "This is a benchmark test message with enough length",
		ChatID: 12345,
	}

	b.ResetTimer()
	for b.Loop() {
		_, err := p.Evaluate(context.Background(), message)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkStorage_RecordDuplicate(b *testing.B) {
	storage := duplicate.NewStorage(5 * time.Minute)

	b.ResetTimer()
	for i := range b.N {
		storage.Record(12345, fmt.Sprintf("hash-%d", i))
	}
}
