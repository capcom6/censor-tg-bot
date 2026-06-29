package tgbotapifx

import "time"

type Config struct {
	Token           string
	LongPollTimeout time.Duration
	Debug           bool
	ProxyURL        string
	Timeout         time.Duration
}
