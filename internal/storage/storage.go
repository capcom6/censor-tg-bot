package storage

import (
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/config"
)

type Storage struct {
	ttl time.Duration

	items map[string]*item
	mux   *sync.Mutex
}

func (s *Storage) GetOrSet(key string) (int, error) {
	s.mux.Lock()
	defer s.mux.Unlock()

	i, ok := s.items[key]
	if ok {
		return i.inc(), nil
	}

	i, err := newItem(s.ttl)
	if err != nil {
		return 0, err
	}

	s.items[key] = i

	return 1, nil
}

func New(config config.Storage) (*Storage, error) {
	u, err := url.Parse(config.URL)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse url: %w", ErrInitFailed, err)
	}

	if u.Scheme != "memory" {
		return nil, fmt.Errorf("%w: unsupported scheme: %s", ErrInitFailed, u.Scheme)
	}

	ttl, err := time.ParseDuration(u.Query().Get("ttl"))
	if err != nil {
		return nil, fmt.Errorf("%w: error parsing ttl: %w", ErrInitFailed, err)
	}

	return &Storage{
		ttl:   ttl,
		items: make(map[string]*item),
		mux:   &sync.Mutex{},
	}, nil
}
