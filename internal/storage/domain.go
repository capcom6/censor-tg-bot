package storage

import (
	"errors"
	"time"
)

type item struct {
	count int
	ttl   time.Duration
	since time.Time
}

func (i *item) inc() int {
	if time.Since(i.since) > i.ttl {
		i.since = time.Now()
		i.count = 0
	}

	i.count++

	return i.count
}

func newItem(ttl time.Duration) (*item, error) {
	if ttl == 0 {
		return nil, errors.New("ttl cannot be 0")
	}

	return &item{
		count: 1,
		ttl:   ttl,
		since: time.Now(),
	}, nil
}
