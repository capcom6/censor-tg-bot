package hashers

import "errors"

// ErrSkipped signals that the hasher has no content to hash for this message.
// It means "not applicable" and must not be treated as a failure.
var ErrSkipped = errors.New("skipped")
