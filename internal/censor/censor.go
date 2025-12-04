package censor

import (
	"regexp"
	"strings"
)

var filter = regexp.MustCompile(`[^\p{Cyrillic}\p{Latin}][:graph:]`)

type Censor struct {
	blacklist []string

	metrics *Metrics
}

func New(config Config, metrics *Metrics) *Censor {
	return &Censor{
		blacklist: config.Blacklist,

		metrics: metrics,
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
			c.metrics.IncProcessedTotal(MetricLabelResultFiltered)
			return false, nil
		}
	}

	c.metrics.IncProcessedTotal(MetricLabelResultAllowed)
	return true, nil
}
