package bot

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type (
	MetricLabelAction string
	MetricLabelStatus string

	// MetricLabelMediaResult is the outcome of one Telegram photo download. Its four
	// values are the whole label set: an ABSENCE of an image is not a FAILURE to
	// obtain one, and the two must stay greppable apart.
	MetricLabelMediaResult string
)

const (
	metricsNamespace = "censor"
	metricsSubsystem = "bot"

	MetricLabelActionMessageProcessed MetricLabelAction = "message_processed"
	MetricLabelActionMessageDeleted   MetricLabelAction = "message_deleted"
	MetricLabelActionUserBanned       MetricLabelAction = "user_banned"
	MetricLabelActionAdminNotified    MetricLabelAction = "admin_notified"

	MetricLabelStatusSuccess MetricLabelStatus = "success"
	MetricLabelStatusFailed  MetricLabelStatus = "failed"

	MetricLabelMediaResultOK          MetricLabelMediaResult = "ok"
	MetricLabelMediaResultTooLarge    MetricLabelMediaResult = "too_large"
	MetricLabelMediaResultNotImage    MetricLabelMediaResult = "not_image"
	MetricLabelMediaResultFetchFailed MetricLabelMediaResult = "fetch_failed"
)

type Metrics struct {
	processedActionsTotal *prometheus.CounterVec

	mediaDownloadsTotal     *prometheus.CounterVec
	mediaDownloadBytesTotal prometheus.Counter
}

func NewMetrics() *Metrics {
	return &Metrics{
		processedActionsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "processed_actions_total",
			Help:      "Total number of bot actions performed",
		}, []string{"action", "status"}),

		mediaDownloadsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "media_downloads_total",
			Help:      "Total number of Telegram photo download attempts by outcome",
		}, []string{"result"}),

		mediaDownloadBytesTotal: promauto.NewCounter(prometheus.CounterOpts{
			Namespace: metricsNamespace,
			Subsystem: metricsSubsystem,
			Name:      "media_download_bytes_total",
			Help:      "Total number of image bytes downloaded from Telegram",
		}),
	}
}

func (m *Metrics) IncProcessedAction(action MetricLabelAction, status MetricLabelStatus) {
	m.processedActionsTotal.WithLabelValues(string(action), string(status)).Inc()
}

// IncMediaDownload records one photo download outcome.
func (m *Metrics) IncMediaDownload(result MetricLabelMediaResult) {
	m.mediaDownloadsTotal.WithLabelValues(string(result)).Inc()
}

// AddMediaDownloadBytes records the size of one successfully downloaded photo.
func (m *Metrics) AddMediaDownloadBytes(size int) {
	m.mediaDownloadBytesTotal.Add(float64(size))
}
