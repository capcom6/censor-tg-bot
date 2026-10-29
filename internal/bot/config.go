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

// Validate checks the configuration values the bot cannot run without.
//
// The media limits are validated because a non-positive limit makes every photo
// undownloadable: a zero size limit rejects every photo variant that reports
// its size, and a zero timeout expires the download context immediately.
func (c Config) Validate() error {
	if c.Media.MaxBytes <= 0 {
		return fmt.Errorf("%w: media.max_bytes must be positive, got: %d", ErrInvalidConfig, c.Media.MaxBytes)
	}

	if c.Media.Timeout <= 0 {
		return fmt.Errorf("%w: media.timeout must be positive, got: %s", ErrInvalidConfig, c.Media.Timeout)
	}

	return nil
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
