package hashers_test

import (
	"context"
	"testing"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/duplicate/hashers"
	"github.com/stretchr/testify/require"
)

func TestText_Hash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		message  plugin.Message
		wantHash string
		wantErr  bool
	}{
		{
			name:     "hashes the text",
			message:  plugin.Message{Text: "Hello world"},
			wantHash: "d58b3fa7",
		},
		{
			name:     "ignores the case",
			message:  plugin.Message{Text: "HELLO WORLD"},
			wantHash: "d58b3fa7",
		},
		{
			name:     "collapses the whitespace",
			message:  plugin.Message{Text: "Hello   world"},
			wantHash: "d58b3fa7",
		},
		{
			name:     "falls back to the caption",
			message:  plugin.Message{Caption: "Hello world"},
			wantHash: "d58b3fa7",
		},
		{
			name:    "skips a short message",
			message: plugin.Message{Text: "Hi"},
			wantErr: true,
		},
		{
			name:    "skips an empty message",
			message: plugin.Message{},
			wantErr: true,
		},
	}

	hasher := hashers.NewText()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := hasher.Hash(context.Background(), tt.message)

			if tt.wantErr {
				require.ErrorIs(t, err, hashers.ErrSkipped)
				require.Empty(t, got)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.wantHash, got)
		})
	}
}

func TestText_HashIsDeterministic(t *testing.T) {
	t.Parallel()

	hasher := hashers.NewText()
	message := plugin.Message{Text: "Hello world"}

	first, err := hasher.Hash(context.Background(), message)
	require.NoError(t, err)

	second, err := hasher.Hash(context.Background(), message)
	require.NoError(t, err)

	require.Equal(t, first, second)
}
