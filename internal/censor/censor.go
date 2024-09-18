package censor

import (
	"regexp"
	"strings"

	"github.com/capcom6/censor-tg-bot/internal/config"
)

var filter = regexp.MustCompile(`[^\p{Cyrillic}\p{Latin}][:graph:]`)

type Censor struct {
	blacklist []string
}

func New(config config.Censor) *Censor {
	return &Censor{
		blacklist: config.Blacklist,
	}
}

func (c *Censor) IsAllow(text string) (bool, error) {
	text = filter.ReplaceAllString(text, "")
	text = strings.ToLower(text)
	for _, word := range c.blacklist {
		if strings.Contains(text, word) {
			return false, nil
		}
	}
	return true, nil
}
