package bot_test

import (
	"testing"

	"github.com/capcom6/censor-tg-bot/internal/bot"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/stretchr/testify/require"
)

func TestSelectPhotoSize(t *testing.T) {
	t.Parallel()

	const maxBytes = 1024 * 1024

	tests := []struct {
		name     string
		photos   []tgbotapi.PhotoSize
		maxBytes int64
		wantFile string
		wantNil  bool
	}{
		{
			name: "prefers the smallest variant clearing the dimension preference",
			photos: []tgbotapi.PhotoSize{
				{FileID: "large", Width: 1280, Height: 1280, FileSize: 500},
				{FileID: "medium", Width: 800, Height: 800, FileSize: 300},
				{FileID: "small", Width: 600, Height: 600, FileSize: 100},
			},
			maxBytes: maxBytes,
			wantFile: "small",
		},
		{
			name: "falls back to the smallest variant clearing the dimension floor",
			photos: []tgbotapi.PhotoSize{
				{FileID: "floor", Width: 400, Height: 400, FileSize: 300},
				{FileID: "smaller", Width: 300, Height: 300, FileSize: 100},
			},
			maxBytes: maxBytes,
			wantFile: "smaller",
		},
		{
			name: "falls back to the smallest eligible variant below the dimension floor",
			photos: []tgbotapi.PhotoSize{
				{FileID: "panorama", Width: 4000, Height: 200, FileSize: 500},
				{FileID: "narrow", Width: 800, Height: 100, FileSize: 100},
			},
			maxBytes: maxBytes,
			wantFile: "narrow",
		},
		{
			name: "skips variants larger than the limit",
			photos: []tgbotapi.PhotoSize{
				{FileID: "huge", Width: 1280, Height: 1280, FileSize: 4 * maxBytes},
				{FileID: "fits", Width: 600, Height: 600, FileSize: 100},
			},
			maxBytes: maxBytes,
			wantFile: "fits",
		},
		{
			name: "treats an unknown size as eligible",
			photos: []tgbotapi.PhotoSize{
				{FileID: "unknown", Width: 1280, Height: 1280, FileSize: 0},
				{FileID: "huge", Width: 1280, Height: 1280, FileSize: 4 * maxBytes},
			},
			maxBytes: maxBytes,
			wantFile: "unknown",
		},
		{
			name: "returns nil when no variant fits the limit",
			photos: []tgbotapi.PhotoSize{
				{FileID: "huge", Width: 1280, Height: 1280, FileSize: 4 * maxBytes},
			},
			maxBytes: maxBytes,
			wantNil:  true,
		},
		{
			name:     "returns nil without variants",
			photos:   nil,
			maxBytes: maxBytes,
			wantNil:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := bot.SelectPhotoSizeForTest(tt.photos, tt.maxBytes)

			if tt.wantNil {
				require.Nil(t, got)

				return
			}

			require.NotNil(t, got)
			require.Equal(t, tt.wantFile, got.FileID)
		})
	}
}
