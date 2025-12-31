package llm

import "errors"

var (
	ErrUnexpectedResponseCount = errors.New("unexpected response count")
	ErrInvalidConfidence       = errors.New("invalid confidence value")
)
