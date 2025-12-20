package censor

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/samber/lo"
	"go.uber.org/zap"
)

// ExecutionStrategy determines how plugins are executed.
type ExecutionStrategy string

const (
	StrategySequential ExecutionStrategy = "sequential" // execute plugins in priority order, stop on first Block
	StrategyParallel   ExecutionStrategy = "parallel"   // execute all plugins concurrently, aggregate results
)

func (s ExecutionStrategy) IsValid() bool {
	switch s {
	case StrategySequential, StrategyParallel:
		return true
	default:
		return false
	}
}

// Service orchestrates plugin execution.
type Service struct {
	config  Config
	plugins []plugin.Plugin

	metrics *Metrics
	logger  *zap.Logger

	mu sync.RWMutex
}

// New creates a new plugin manager.
func New(plugins []plugin.Plugin, config Config, metrics *Metrics, logger *zap.Logger) *Service {
	return &Service{
		config:  config,
		plugins: plugins,

		metrics: metrics,
		logger:  logger,

		mu: sync.RWMutex{},
	}
}

// Register adds a plugin to the manager.
func (s *Service) Register(p plugin.Plugin) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if plugin already exists
	for _, existing := range s.plugins {
		if existing.Name() == p.Name() {
			return fmt.Errorf("%w: %s", ErrAlreadyExists, p.Name())
		}
	}

	s.plugins = append(s.plugins, p)

	s.logger.Info("plugin registered",
		zap.String("plugin", p.Name()),
		zap.Int("priority", p.Priority()),
	)

	return nil
}

// GetPlugins returns a copy of the current plugins list (sorted by priority).
func (s *Service) GetPlugins() []plugin.Plugin {
	s.mu.RLock()
	defer s.mu.RUnlock()

	plugins := lo.Filter(
		s.plugins,
		func(p plugin.Plugin, _ int) bool {
			if !s.config.EnabledOnly {
				return true
			}

			if c, ok := s.config.Plugins[p.Name()]; ok {
				return c.Enabled
			}

			return false
		},
	)

	// Sort by priority (lower number = higher priority)
	sort.Slice(plugins, func(i, j int) bool {
		return s.getPluginPriority(plugins[i]) < s.getPluginPriority(plugins[j])
	})

	return plugins
}

// Evaluate runs plugins according to the configured strategy.
func (s *Service) Evaluate(ctx context.Context, msg plugin.Message) plugin.Result {
	plugins := s.GetPlugins()

	if len(plugins) == 0 {
		// No plugins registered, use configured skip action
		return plugin.Result{
			Action:   s.config.SkipAction,
			Reason:   "no plugins registered",
			Metadata: nil,
			Plugin:   "manager",
		}
	}

	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()

	var result plugin.Result
	var err error
	switch s.config.Strategy {
	case StrategySequential:
		result, err = s.evaluateSequential(ctx, msg, plugins)
	case StrategyParallel:
		result, err = s.evaluateParallel(ctx, msg, plugins)
	default:
		err = fmt.Errorf("%w: %s", ErrInvalidStrategy, s.config.Strategy)
	}

	if err != nil {
		result.Action = s.config.ErrorAction
		result.Reason = err.Error()
		result.Plugin = "manager"
	} else if result.Action == plugin.ActionSkip {
		result.Action = s.config.SkipAction
		result.Plugin = "manager"
	}

	s.metrics.RecordTotalEvaluation(result)

	return result
}

func (s *Service) Cleanup(ctx context.Context) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, p := range s.plugins {
		p.Cleanup(ctx)
	}
}

// getPluginPriority returns the priority of a plugin.
func (s *Service) getPluginPriority(p plugin.Plugin) int {
	if c, ok := s.config.Plugins[p.Name()]; ok {
		return c.Priority
	}
	return p.Priority()
}

// evaluateSequential executes plugins in priority order, stopping on first block.
func (s *Service) evaluateSequential(
	ctx context.Context,
	msg plugin.Message,
	plugins []plugin.Plugin,
) (plugin.Result, error) {
	hasAllow := false

	for _, p := range plugins {
		select {
		case <-ctx.Done():
			return plugin.Result{}, ErrTimeout
		default:
		}

		start := time.Now()
		result, err := p.Evaluate(ctx, msg)
		duration := time.Since(start)

		// Record metrics
		s.metrics.RecordEvaluation(p.Name(), result.Action, duration, err)

		if err != nil {
			s.logger.Error("plugin evaluation error",
				zap.String("plugin", p.Name()),
				zap.Error(err),
			)
			return plugin.Result{}, fmt.Errorf("%w: %s", ErrPluginError, p.Name())
		}

		switch result.Action {
		case plugin.ActionBlock:
			s.logger.Debug("plugin blocked message",
				zap.String("plugin", p.Name()),
				zap.String("reason", result.Reason),
			)
			return result, nil
		case plugin.ActionAllow:
			// In sequential mode, allow is not final - continue to next plugin
			s.logger.Debug("plugin allowed message", zap.String("plugin", p.Name()))
			hasAllow = true
		case plugin.ActionSkip:
			s.logger.Debug("plugin skipped message", zap.String("plugin", p.Name()))
		}
	}

	if hasAllow {
		return plugin.Result{
			Action:   plugin.ActionAllow,
			Reason:   "all plugins passed",
			Metadata: nil,
			Plugin:   "manager",
		}, nil
	}

	return plugin.Result{
		Action:   plugin.ActionSkip,
		Reason:   "all plugins skipped",
		Metadata: nil,
		Plugin:   "manager",
	}, nil
}

// evaluateParallel executes all plugins concurrently.
func (s *Service) evaluateParallel(
	ctx context.Context,
	msg plugin.Message,
	plugins []plugin.Plugin,
) (plugin.Result, error) {
	type resultWithPlugin struct {
		result plugin.Result
		err    error
		plugin plugin.Plugin
	}

	results := make(chan resultWithPlugin, len(plugins))

	// Start all plugin evaluations concurrently
	for _, p := range plugins {
		go func(p plugin.Plugin) {
			start := time.Now()
			result, err := p.Evaluate(ctx, msg)
			duration := time.Since(start)

			// Record metrics
			s.metrics.RecordEvaluation(p.Name(), result.Action, duration, err)

			results <- resultWithPlugin{result, err, p}
		}(p)
	}

	// Wait for all plugins to complete or context to be done
	allResults := []resultWithPlugin{}
	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			select {
			case r := <-results:
				allResults = append(allResults, r)
				if len(allResults) == len(plugins) {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	<-done

	if len(allResults) != len(plugins) {
		return plugin.Result{}, ErrTimeout
	}

	// Aggregate results
	hasAllow := false
	for _, r := range allResults {
		if r.err != nil {
			s.logger.Error("plugin evaluation error",
				zap.String("plugin", r.plugin.Name()),
				zap.Error(r.err),
			)
			return plugin.Result{}, fmt.Errorf("%w: %s", ErrPluginError, r.plugin.Name())
		}

		if r.result.Action == plugin.ActionBlock {
			s.logger.Debug("plugin blocked message",
				zap.String("plugin", r.plugin.Name()),
				zap.String("reason", r.result.Reason),
			)
			return r.result, nil
		}

		if r.result.Action == plugin.ActionAllow {
			s.logger.Debug("plugin allowed message", zap.String("plugin", r.plugin.Name()))
			hasAllow = true
		}
	}

	if hasAllow {
		return plugin.Result{
			Action:   plugin.ActionAllow,
			Reason:   "at least one plugin allowed, none blocked",
			Metadata: nil,
			Plugin:   "manager",
		}, nil
	}

	return plugin.Result{
		Action:   plugin.ActionSkip,
		Reason:   "all plugins skipped",
		Metadata: nil,
		Plugin:   "manager",
	}, nil
}
