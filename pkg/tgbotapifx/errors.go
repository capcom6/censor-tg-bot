package tgbotapifx

import "errors"

var (
	ErrInvalidConfig = errors.New("invalid config")

	// ErrDownloadFailed reports that Telegram did not deliver the file: a transport
	// failure, a non-200 response or an unreadable body. The wrapped cause has its
	// request URL removed, so the error is safe to log.
	ErrDownloadFailed = errors.New("telegram file download failed")
)
