package censor

import "errors"

var (
	ErrAlreadyExists   = errors.New("plugin already exists")
	ErrInvalidConfig   = errors.New("invalid config")
	ErrInvalidStrategy = errors.New("invalid strategy")
	ErrPluginError     = errors.New("plugin error")
	ErrTimeout         = errors.New("timeout")
)
