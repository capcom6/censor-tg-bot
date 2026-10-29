package plugin

// Media carries the image content downloaded from a Telegram message.
//
// Message.Media is nil for every message that does not carry a photo, so a plugin
// must always nil-check the field before use. SizeBytes is the length of Bytes and
// SHA256 is the full lowercase hex digest of Bytes: plugins never re-hash content
// that already carries a digest.
type Media struct {
	Bytes       []byte // Content of the media
	ContentType string // MIME type detected from the bytes, e.g. "image/jpeg"
	SizeBytes   int    // Length of Bytes in bytes
	SHA256      string // Full lowercase hex SHA-256 digest of Bytes
}
