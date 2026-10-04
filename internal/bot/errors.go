package bot

import "errors"

// The fetchMedia sentinels describe the ABSENCE of an image: the message was not
// offending and must not be deleted. Only an error that is none of them is treated
// as a FAILURE to obtain the image, which blocks the message instead.

// ErrNoMedia reports that there is nothing to download: the message carries no
// photo, or no enabled plugin asked for media in the first place.
var ErrNoMedia = errors.New("message carries no media")

// ErrMediaTooLarge reports that every photo variant exceeds bot.media.max_bytes, or
// that the downloaded file did.
var ErrMediaTooLarge = errors.New("photo exceeds bot.media.max_bytes")

// ErrUnsupportedMediaType reports that the downloaded bytes are not an image.
var ErrUnsupportedMediaType = errors.New("downloaded media is not an image")

// ErrInvalidMediaConfig reports media settings that cannot bound a download.
var ErrInvalidMediaConfig = errors.New("invalid bot.media config")
