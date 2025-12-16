package censor

import (
	"time"

	"github.com/capcom6/censor-tg-bot/internal/censor/plugin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	pluginEvaluations *prometheus.CounterVec   // Labels: plugin, action (allow|block|skip)
	pluginDuration    *prometheus.HistogramVec // Labels: plugin
	pluginErrors      *prometheus.CounterVec   // Labels: plugin
	totalEvaluations  *prometheus.CounterVec   // Labels: result (allowed|blocked)
}

func NewMetrics() *Metrics {
	return &Metrics{
		pluginEvaluations: promauto.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "censor",
			Name:      "plugin_evaluations_total",
			Help:      "Total number of plugin evaluations, labeled by plugin name and action",
		}, []string{"plugin", "action"}),

		pluginDuration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Subsystem: "censor",
			Name:      "plugin_duration_seconds",
			Help:      "Histogram of plugin evaluation durations",
			Buckets:   []float64{1e-6, 1e-5, 1e-4, 0.001, 0.01, 0.1, 1, 10},
		}, []string{"plugin"}),

		pluginErrors: promauto.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "censor",
			Name:      "plugin_errors_total",
			Help:      "Total number of plugin evaluation errors",
		}, []string{"plugin"}),

		totalEvaluations: promauto.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "censor",
			Name:      "evaluations_total",
			Help:      "Total number of message evaluations, labeled by result",
		}, []string{"result"}),
	}
}

// RecordEvaluation records metrics for a plugin evaluation.
func (m *Metrics) RecordEvaluation(pluginName string, action plugin.Action, duration time.Duration, err error) {
	// Record plugin evaluation count
	m.pluginEvaluations.WithLabelValues(pluginName, string(action)).Inc()

	// Record plugin duration
	m.pluginDuration.WithLabelValues(pluginName).Observe(duration.Seconds())

	// Record errors if any
	if err != nil {
		m.pluginErrors.WithLabelValues(pluginName).Inc()
	}
}

// RecordTotalEvaluation records the final result of message evaluation.
func (m *Metrics) RecordTotalEvaluation(result plugin.Result) {
	m.totalEvaluations.WithLabelValues(string(result.Action)).Inc()
}
