package bot

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type (
	MetricLabelAction string
	MetricLabelStatus string
)

const (
	MetricLabelActionMessageProcessed MetricLabelAction = "message_processed"
	MetricLabelActionMessageDeleted   MetricLabelAction = "message_deleted"
	MetricLabelActionUserBanned       MetricLabelAction = "user_banned"
	MetricLabelActionAdminNotified    MetricLabelAction = "admin_notified"

	MetricLabelStatusSuccess MetricLabelStatus = "success"
	MetricLabelStatusFailed  MetricLabelStatus = "failed"
)

type Metrics struct {
	processedActionsTotal *prometheus.CounterVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		processedActionsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Subsystem: "bot",
			Name:      "processed_actions_total",
			Help:      "Total number of bot actions performed",
		}, []string{"action", "status"}),
	}
}

func (m *Metrics) IncProcessedAction(action MetricLabelAction, status MetricLabelStatus) {
	m.processedActionsTotal.WithLabelValues(string(action), string(status)).Inc()
}
