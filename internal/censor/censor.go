package censor

import (
	"regexp"
	"strings"
)

var filter = regexp.MustCompile(`[^\p{Cyrillic}\p{Latin}][:graph:]`)

type censor struct {
	blacklist []string
}

func New(config Config) *censor {
	return &censor{
		blacklist: config.Blacklist,
	}
}

func (c *censor) IsAllow(text string) (bool, error) {
	text = filter.ReplaceAllString(text, "")
	text = strings.ToLower(text)
	for _, word := range c.blacklist {
		if strings.Contains(text, word) {
			return false, nil
		}
	}
	return true, nil
}
