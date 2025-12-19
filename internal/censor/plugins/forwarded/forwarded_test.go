package forwarded_test

import (
	"context"
	"testing"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/forwarded"
	"github.com/stretchr/testify/require"
)

func TestPlugin_Name(t *testing.T) {
	config, _ := forwarded.NewConfig(map[string]any{})
	p := forwarded.New(config)
	require.Equal(t, "forwarded", p.Name())
}

func TestPlugin_Priority(t *testing.T) {
	config, _ := forwarded.NewConfig(map[string]any{})
	p := forwarded.New(config)
	require.Equal(t, 15, p.Priority())
}

func TestPlugin_Evaluate(t *testing.T) {
	allowedUserID := int64(12345)
	allowedChatID := int64(-1001234567890)

	tests := []struct {
		name           string
		config         forwarded.Config
		message        plugin.Message
		expected       plugin.Action
		expectedReason string
	}{
		{
			name:           "non-forwarded message skips",
			config:         forwarded.Config{},
			message:        plugin.Message{Text: "Hello world"},
			expected:       plugin.ActionSkip,
			expectedReason: "message is not forwarded",
		},
		{
			name:   "forwarded message without allowed sources blocks",
			config: forwarded.Config{},
			message: plugin.Message{
				Text:                "Forwarded message",
				ForwardedFromUserID: &[]int64{67890}[0],
			},
			expected:       plugin.ActionBlock,
			expectedReason: "message is forwarded from disallowed source",
		},
		{
			name: "forwarded from allowed user allows",
			config: forwarded.Config{
				AllowedUserIDs: []int64{allowedUserID},
			},
			message: plugin.Message{
				Text:                "Forwarded message from allowed user",
				ForwardedFromUserID: &allowedUserID,
			},
			expected:       plugin.ActionAllow,
			expectedReason: "forwarded from allowed user",
		},
		{
			name: "forwarded from allowed chat allows",
			config: forwarded.Config{
				AllowedChatIDs: []int64{allowedChatID},
			},
			message: plugin.Message{
				Text:                "Forwarded message from allowed chat",
				ForwardedFromChatID: &allowedChatID,
			},
			expected:       plugin.ActionAllow,
			expectedReason: "forwarded from allowed chat",
		},
		{
			name: "forwarded from allowed user and chat allows",
			config: forwarded.Config{
				AllowedUserIDs: []int64{allowedUserID},
				AllowedChatIDs: []int64{allowedChatID},
			},
			message: plugin.Message{
				Text:                "Forwarded message",
				ForwardedFromUserID: &allowedUserID,
				ForwardedFromChatID: &allowedChatID,
			},
			expected:       plugin.ActionAllow,
			expectedReason: "forwarded from allowed user",
			// Note: Plugin returns after first match, so only user ID is in metadata
		},
		{
			name: "forwarded from disallowed user blocks",
			config: forwarded.Config{
				AllowedUserIDs: []int64{allowedUserID},
			},
			message: plugin.Message{
				Text:                "Forwarded message from disallowed user",
				ForwardedFromUserID: &[]int64{99999}[0],
			},
			expected:       plugin.ActionBlock,
			expectedReason: "message is forwarded from disallowed source",
		},
		{
			name: "forwarded from disallowed chat blocks",
			config: forwarded.Config{
				AllowedChatIDs: []int64{allowedChatID},
			},
			message: plugin.Message{
				Text:                "Forwarded message from disallowed chat",
				ForwardedFromChatID: &[]int64{-1009876543210}[0],
			},
			expected:       plugin.ActionBlock,
			expectedReason: "message is forwarded from disallowed source",
		},
		{
			name: "empty allowed lists blocks",
			config: forwarded.Config{
				AllowedUserIDs: []int64{},
				AllowedChatIDs: []int64{},
			},
			message: plugin.Message{
				Text:                "Forwarded message",
				ForwardedFromUserID: &[]int64{67890}[0],
			},
			expected:       plugin.ActionBlock,
			expectedReason: "message is forwarded from disallowed source",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := forwarded.New(tt.config)
			result, err := p.Evaluate(context.Background(), tt.message)

			require.NoError(t, err)
			require.Equal(t, tt.expected, result.Action)
			require.Equal(t, tt.expectedReason, result.Reason)
			require.Equal(t, "forwarded", result.Plugin)

			// Verify metadata contains expected fields based on test case
			switch {
			case tt.name == "forwarded from allowed user and chat skips":
				// Plugin checks user first and returns early, so only user ID is in metadata
				require.Contains(t, result.Metadata, "forwarded_from_user_id")
				require.NotContains(t, result.Metadata, "forwarded_from_chat_id")
			case tt.expected == plugin.ActionBlock:
				// For block actions, ensure metadata contains forwarding info
				if tt.message.ForwardedFromUserID != nil {
					require.Contains(t, result.Metadata, "forwarded_from_user_id")
				}
				if tt.message.ForwardedFromChatID != nil {
					require.Contains(t, result.Metadata, "forwarded_from_chat_id")
				}
			case tt.expected == plugin.ActionSkip:
				if tt.message.ForwardedFromUserID != nil {
					require.Contains(t, result.Metadata, "forwarded_from_user_id")
				}
				if tt.message.ForwardedFromChatID != nil {
					require.Contains(t, result.Metadata, "forwarded_from_chat_id")
				}
			}
		})
	}
}

func TestConfig_NewConfig(t *testing.T) {
	tests := []struct {
		name    string
		input   map[string]any
		expect  func(*require.Assertions) forwarded.Config
		wantErr bool
	}{
		{
			name: "valid config with all fields",
			input: map[string]any{
				"allowed_user_ids": []any{int64(12345), int64(67890)},
				"allowed_chat_ids": []any{int64(-1001234567890)},
			},
			expect: func(_ *require.Assertions) forwarded.Config {
				return forwarded.Config{
					AllowedUserIDs: []int64{12345, 67890},
					AllowedChatIDs: []int64{-1001234567890},
				}
			},
			wantErr: false,
		},
		{
			name: "valid config with minimal fields",
			input: map[string]any{
				"allowed_user_ids": []any{int64(12345)},
			},
			expect: func(_ *require.Assertions) forwarded.Config {
				return forwarded.Config{
					AllowedUserIDs: []int64{12345},
					AllowedChatIDs: []int64{},
				}
			},
			wantErr: false,
		},
		{
			name:  "valid empty config",
			input: map[string]any{},
			expect: func(_ *require.Assertions) forwarded.Config {
				return forwarded.Config{
					AllowedUserIDs: []int64{},
					AllowedChatIDs: []int64{},
				}
			},
			wantErr: false,
		},
		{
			name: "invalid allowed_user_ids type",
			input: map[string]any{
				"allowed_user_ids": "not a slice",
			},
			wantErr: true,
		},
		{
			name: "invalid allowed_chat_ids type",
			input: map[string]any{
				"allowed_chat_ids": 123,
			},
			wantErr: true,
		},
		{
			name: "invalid user ID in allowed_user_ids",
			input: map[string]any{
				"allowed_user_ids": []any{"not a number"},
			},
			wantErr: true,
		},
		{
			name: "invalid chat ID in allowed_chat_ids",
			input: map[string]any{
				"allowed_chat_ids": []any{true},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := forwarded.NewConfig(tt.input)

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				expectedConfig := tt.expect(require.New(t))
				require.Equal(t, expectedConfig, result)
			}
		})
	}
}
