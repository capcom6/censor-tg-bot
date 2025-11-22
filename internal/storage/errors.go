package storage

import "errors"

var (
	ErrInitFailed = errors.New("failed to init storage")
	ErrInvalidTTL = errors.New("invalid ttl")
)
