package storage

import "errors"

var (
	// ErrInvalidTTL is returned when the configured window is not positive.
	ErrInvalidTTL = errors.New("invalid ttl")
)
