package hashers_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/duplicate/hashers"
	"github.com/stretchr/testify/require"
)

func TestMedia_Hash(t *testing.T) {
	t.Parallel()

	sum := sha256.Sum256([]byte("image content"))
	digest := hex.EncodeToString(sum[:])

	tests := []struct {
		name     string
		message  plugin.Message
		wantHash string
		wantErr  bool
	}{
		{
			name: "uses the digest of the media",
			message: plugin.Message{
				Media: &plugin.Media{
					Bytes:       []byte("image content"),
					ContentType: "image/jpeg",
					SizeBytes:   len("image content"),
					SHA256:      digest,
				},
			},
			wantHash: digest,
		},
		{
			name: "computes the digest when the media carries none",
			message: plugin.Message{
				Media: &plugin.Media{
					Bytes:       []byte("image content"),
					ContentType: "image/jpeg",
					SizeBytes:   len("image content"),
				},
			},
			wantHash: digest,
		},
		{
			name:    "skips a message without media",
			message: plugin.Message{Text: "Hello world"},
			wantErr: true,
		},
		{
			name: "skips empty media",
			message: plugin.Message{
				Media: &plugin.Media{ContentType: "image/jpeg"},
			},
			wantErr: true,
		},
	}

	hasher := hashers.NewMedia()

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
