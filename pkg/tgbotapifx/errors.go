package tgbotapifx

import "errors"

var (
	ErrInvalidConfig = errors.New("invalid config")

	// ErrDownloadFailed reports that Telegram did not deliver the file: a transport
	// failure, a non-200 response or an unreadable body. The wrapped cause is already
	// redacted, so the error is safe to log.
	ErrDownloadFailed = errors.New("telegram file download failed")

	// ErrFileTooLarge reports that the downloaded file exceeds the caller's limit.
	// It is an absence rather than a failure: the message itself was not offending.
	ErrFileTooLarge = errors.New("telegram file exceeds the size limit")
)
