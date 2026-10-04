package plugin

// Media carries the image content downloaded from a Telegram message.
//
// Message.Media is nil for every message that does not carry a photo, so a plugin
// must always nil-check the field before use. SizeBytes is the length of Bytes and
// SHA256 is the full lowercase hex digest of Bytes: plugins never re-hash content
// that already carries a digest.
type Media struct {
	Bytes       []byte // Raw image bytes exactly as downloaded
	ContentType string // MIME type detected from the bytes, e.g. "image/jpeg"
	SizeBytes   int    // Length of Bytes in bytes
	SHA256      string // Full lowercase hex SHA-256 digest of Bytes
}

// MediaConsumer is an optional capability interface implemented by plugins that
// need the media content of a message.
//
// The service queries it through Service.RequiresMedia before any media is
// downloaded, so NeedsMedia is called with an empty Message and an implementation
// must decide from its own configuration only.
type MediaConsumer interface {
	// NeedsMedia reports whether the plugin needs the media of the given message.
	NeedsMedia(msg Message) bool
}
