package duplicate

import (
	"context"
	"errors"
	"fmt"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/capcom6/censor-tg-bot/internal/censor/plugins/duplicate/hashers"
	"github.com/samber/lo"
)

const (
	// textHashPrefix namespaces text hashes inside the duplicate storage.
	textHashPrefix = "text:"
	// mediaHashPrefix namespaces media hashes inside the duplicate storage.
	mediaHashPrefix = "media:"
)

// hasher computes a duplicate-detection hash for a message. Implementations
// return an error wrapping hashers.ErrSkipped when the message has no content
// for them to hash; such an error means "not applicable", not a failure.
type hasher interface {
	Hash(ctx context.Context, msg plugin.Message) (string, error)
}

func Metadata() plugin.Metadata {
	return plugin.Metadata{
		Name: "duplicate",
		Factory: func(params map[string]any, deps plugin.Deps) (plugin.Plugin, error) {
			config, err := NewConfig(params)
			if err != nil {
				return nil, err
			}

			return New(config, deps), nil
		},
	}
}

type Plugin struct {
	storage *Storage
	config  Config

	hashers map[string]hasher
}

func New(config Config, deps plugin.Deps) plugin.Plugin {
	return &Plugin{
		storage: NewStorage(deps.Cache, config.Window),
		config:  config,

		hashers: map[string]hasher{
			textHashPrefix:  hashers.NewText(),
			mediaHashPrefix: hashers.NewMedia(),
		},
	}
}

func (p *Plugin) Name() string {
	return "duplicate"
}

func (p *Plugin) Capabilities() plugin.Capability {
	return plugin.CapabilityText | plugin.CapabilityMedia
}

func (p *Plugin) Priority() int {
	const priority = 150
	return priority
}

func (p *Plugin) Evaluate(ctx context.Context, msg plugin.Message) (plugin.Result, error) {
	// Calculate max occurrences (original + allowed duplicates)
	maxOccurrences := p.config.MaxDuplicates + 1

	hashes := map[string]string{}
	errs := map[string]error{}
	var stats []int64
	for prefix, hasher := range p.hashers {
		hash, err := hasher.Hash(ctx, msg)
		if err != nil {
			errs[prefix] = err
			continue
		}

		if hash == "" {
			continue
		}

		hashes[prefix] = hash

		// Record the occurrence and check the limit below.
		count, err := p.storage.Record(ctx, msg.ChatID, prefix+hash)
		if err != nil {
			return plugin.Result{}, err
		}

		stats = append(stats, count)
	}

	skipErrs := lo.FilterValues(
		errs,
		func(_ string, err error) bool {
			return errors.Is(err, hashers.ErrSkipped)
		},
	)

	if len(skipErrs) == 0 && len(errs) == len(p.hashers) {
		return plugin.Result{}, errors.Join(lo.Values(errs)...)
	}

	metadata := map[string]any{}
	if textHash := hashes[textHashPrefix]; textHash != "" {
		metadata["message_hash"] = textHash
	}
	if mediaHash := hashes[mediaHashPrefix]; mediaHash != "" {
		metadata["media_hash"] = mediaHash
	}

	if len(skipErrs) == len(p.hashers) {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   errors.Join(lo.Values(errs)...).Error(),
			Metadata: metadata,
			Plugin:   p.Name(),
		}, nil
	}

	violations := lo.Filter(stats, func(count int64, _ int) bool {
		return count > int64(maxOccurrences)
	})

	if len(violations) == 0 {
		return plugin.Result{
			Action:   plugin.ActionSkip,
			Reason:   "duplicate limit not exceeded",
			Metadata: metadata,
			Plugin:   p.Name(),
		}, nil
	}

	metadata["count"] = violations[0]
	metadata["max_occurrences"] = maxOccurrences
	metadata["window"] = p.config.Window

	return plugin.Result{
		Action: plugin.ActionBlock,
		Reason: fmt.Sprintf(
			"duplicate limit exceeded (%d occurrences, max %d allowed in %s)",
			violations[0],
			maxOccurrences,
			p.config.Window,
		),
		Metadata: metadata,
		Plugin:   p.Name(),
	}, nil
}

// Cleanup is a no-op: expired counters are reclaimed by the cachefx sweeper.
func (p *Plugin) Cleanup(context.Context) {
}
