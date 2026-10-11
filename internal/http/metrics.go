package http

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// commonMetrics holds the Prometheus metrics both scrapers export. The four
// metrics here are identical in type, labels and help text on both sides; only
// the name prefix differs, which newCommonMetrics takes as its namespace.
type commonMetrics struct {
	APIRequestsTotal    *prometheus.CounterVec
	APIRequestDuration  *prometheus.HistogramVec
	LastScrapeTimestamp *prometheus.GaugeVec
	DBOperationsTotal   *prometheus.CounterVec
}

// newCommonMetrics creates and registers the shared metrics. Prometheus joins
// namespace and name with an underscore, so a namespace of "heizsaison_oil"
// yields heizsaison_oil_api_requests_total and so on.
func newCommonMetrics(namespace string) commonMetrics {
	return commonMetrics{
		APIRequestsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "api_requests_total",
				Help:      "Total number of API requests by provider and status",
			},
			[]string{"provider", "status"},
		),
		APIRequestDuration: promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Namespace: namespace,
				Name:      "api_request_duration_seconds",
				Help:      "API request duration in seconds",
				Buckets:   prometheus.DefBuckets,
			},
			[]string{"provider"},
		),
		LastScrapeTimestamp: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: namespace,
				Name:      "last_scrape_timestamp",
				Help:      "Timestamp of the last successful scrape",
			},
			[]string{"provider"},
		),
		DBOperationsTotal: promauto.NewCounterVec(
			prometheus.CounterOpts{
				Namespace: namespace,
				Name:      "db_operations_total",
				Help:      "Total number of database operations by type and status",
			},
			[]string{"operation", "status"},
		),
	}
}

// RecordAPIRequest records an API request metric.
func (m *commonMetrics) RecordAPIRequest(provider, status string, duration float64) {
	m.APIRequestsTotal.WithLabelValues(provider, status).Inc()
	m.APIRequestDuration.WithLabelValues(provider).Observe(duration)
}

// RecordLastScrape records the last successful scrape timestamp.
func (m *commonMetrics) RecordLastScrape(provider string, timestamp float64) {
	m.LastScrapeTimestamp.WithLabelValues(provider).Set(timestamp)
}

// RecordDBOperation records a database operation metric.
func (m *commonMetrics) RecordDBOperation(operation, status string) {
	m.DBOperationsTotal.WithLabelValues(operation, status).Inc()
}
