package storage

import "time"

// Config configures the violation counter window.
type Config struct {
	// TTL is the sliding idle-window of a violation counter: every
	// increment restarts it, so a counter resets only after TTL of inactivity.
	TTL time.Duration
}
