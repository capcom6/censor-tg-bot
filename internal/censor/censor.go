package censor

import (
	"regexp"
	"strings"
)

var filter = regexp.MustCompile(`[^\p{Cyrillic}\p{Latin}][:graph:]`)

type Censor struct {
	blacklist []string
}

func New(config Config) *Censor {
	return &Censor{
		blacklist: config.Blacklist,
	}
}

func (c *Censor) IsAllow(text string) (bool, error) {
	if text == "" {
		return true, nil
	}

	text = filter.ReplaceAllString(text, "")
	text = strings.ToLower(text)
	for _, word := range c.blacklist {
		if strings.Contains(text, word) {
			return false, nil
		}
	}
	return true, nil
}
