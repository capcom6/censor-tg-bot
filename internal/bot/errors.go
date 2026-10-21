package bot

import "errors"

var (
	ErrInvalidConfig = errors.New("invalid config")

	ErrMediaDownloadFailed = errors.New("failed to download media")
)
