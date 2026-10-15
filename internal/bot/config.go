package bot

import (
	"fmt"
	"time"
)

type Config struct {
	AdminID      int64
	BanThreshold uint8
	Media        MediaConfig
}

// MediaConfig bounds the download of one Telegram photo.
//
// The update pump is a single serial loop (see pkg/tgbotapifx.Run), so Timeout is
// added to censor.Timeout to obtain the per-message worst case: 5s + 30s = 35s by
// default.
type MediaConfig struct {
	MaxBytes int64         // Maximum accepted size of the downloaded file
	Timeout  time.Duration // Deadline for one download; part of the per-message latency budget
}

// Validate rejects media settings that would fail closed forever: a zero timeout
// expires every download before it starts and a non-positive limit rejects every
// variant, so every photo would be deleted and its sender counted toward a ban.
// Rejecting at construction fails the app at startup instead.
func (c Config) Validate() error {
	if c.Media.MaxBytes <= 0 || c.Media.Timeout <= 0 {
		return fmt.Errorf(
			"%w: max_bytes and timeout must be positive (max_bytes=%d, timeout=%s)",
			ErrInvalidMediaConfig,
			c.Media.MaxBytes,
			c.Media.Timeout,
		)
	}

	return nil
}
