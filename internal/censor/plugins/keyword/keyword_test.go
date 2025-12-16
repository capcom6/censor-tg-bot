package keyword_test

import (
	"context"
	"testing"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/keyword"
	"github.com/stretchr/testify/require"
)

func TestPlugin_Evaluate(t *testing.T) {
	p := keyword.New(keyword.Config{
		Blacklist: []string{"spam", "scam"},
	})

	tests := []struct {
		name     string
		message  plugin.Message
		expected plugin.Action
	}{
		{"clean text skips", plugin.Message{Text: "Hello world"}, plugin.ActionSkip},
		{"blacklisted word in text blocks", plugin.Message{Text: "Buy now! SPAM!"}, plugin.ActionBlock},
		{"blacklisted word in caption blocks", plugin.Message{Caption: "scam alert"}, plugin.ActionBlock},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Evaluate(context.Background(), tt.message)
			require.NoError(t, err)
			require.Equal(t, tt.expected, result.Action)
		})
	}
}
