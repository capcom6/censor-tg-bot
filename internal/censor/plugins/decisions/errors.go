package decisions

import "errors"

var (
	// ErrRequestFailed is returned when the decisions call does not succeed.
	ErrRequestFailed = errors.New("decisions request failed")
	// ErrInvalidAnswer is returned when an expected answer is missing or has an unexpected type.
	ErrInvalidAnswer = errors.New("invalid answer")
	// ErrInvalidProbability is returned when a probability is outside the 0.0 - 1.0 range.
	ErrInvalidProbability = errors.New("invalid probability value")
)
