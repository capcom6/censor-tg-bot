package storage

import (
	"fmt"
	"time"
)

type item struct {
	count int
	ttl   time.Duration
	since time.Time
}

func (i *item) inc() int {
	if time.Since(i.since) > i.ttl {
		i.count = 0
	}

	i.since = time.Now()
	i.count++

	return i.count
}

func newItem(ttl time.Duration) (*item, error) {
	if ttl == 0 {
		return nil, fmt.Errorf("%w: ttl must be greater than 0", ErrInvalidTTL)
	}

	return &item{
		count: 1,
		ttl:   ttl,
		since: time.Now(),
	}, nil
}
