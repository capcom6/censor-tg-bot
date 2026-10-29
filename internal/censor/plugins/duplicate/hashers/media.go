package hashers

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
)

type Media struct {
}

func NewMedia() *Media {
	return &Media{}
}

func (m *Media) Hash(_ context.Context, msg plugin.Message) (string, error) {
	media := msg.Media
	if media == nil {
		return "", fmt.Errorf("%w: media not present", ErrSkipped)
	}

	if media.SHA256 != "" {
		return media.SHA256, nil
	}

	if len(media.Bytes) == 0 {
		return "", fmt.Errorf("%w: media content empty", ErrSkipped)
	}

	sum := sha256.Sum256(media.Bytes)

	return hex.EncodeToString(sum[:]), nil
}
