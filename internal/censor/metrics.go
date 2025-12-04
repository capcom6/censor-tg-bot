package censor

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type (
	MetricLabelResult string
)

const (
	MetricLabelResultFiltered MetricLabelResult = "filtered"
	MetricLabelResultAllowed  MetricLabelResult = "allowed"
)

type Metrics struct {
	processedTotal *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		processedTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "censor",
			Name:      "processed_total",
			Help:      "Total number of messages processed by censor, labeled by result",
		}, []string{"result"}),
	}
}

func (m *Metrics) IncProcessedTotal(result MetricLabelResult) {
	m.processedTotal.WithLabelValues(string(result)).Inc()
}
