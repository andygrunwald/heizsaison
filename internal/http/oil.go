package http

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/rs/zerolog"

	"github.com/andygrunwald/heizsaison/internal/database"
	"github.com/andygrunwald/heizsaison/internal/models"
	"github.com/andygrunwald/heizsaison/internal/scheduler"
	"github.com/andygrunwald/heizsaison/internal/scraper"
)

// oilNamespace prefixes every metric the oil scraper exports.
const oilNamespace = "heizsaison_oil"

// Metrics holds all Prometheus metrics for the oil scraper.
type Metrics struct {
	commonMetrics

	// Scrape metrics
	CurrentPriceEUR *prometheus.GaugeVec

	// Database metrics
	PricesStoredTotal *prometheus.GaugeVec
}

// NewMetrics creates and registers Prometheus metrics.
func NewMetrics() *Metrics {
	return &Metrics{
		commonMetrics: newCommonMetrics(oilNamespace),
		CurrentPriceEUR: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: oilNamespace,
				Name:      "current_price_eur",
				Help:      "Current oil price in EUR per 100L",
			},
			[]string{"provider", "scope", "product_type"},
		),
		PricesStoredTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: oilNamespace,
				Name:      "prices_stored_total",
				Help:      "Total number of prices stored in database by provider",
			},
			[]string{"provider"},
		),
	}
}

// RecordCurrentPrice records the current oil price.
func (m *Metrics) RecordCurrentPrice(provider, scope, productType string, price float64) {
	m.CurrentPriceEUR.WithLabelValues(provider, scope, productType).Set(price)
}

// RecordPricesStored records the total number of prices stored for a provider.
func (m *Metrics) RecordPricesStored(provider string, count float64) {
	m.PricesStoredTotal.WithLabelValues(provider).Set(count)
}

// Server represents the HTTP server for metrics and status endpoints.
type Server struct {
	baseServer
	metrics *Metrics
}

// NewServer creates a new HTTP server.
func NewServer(addr string, s *scraper.Scraper, sched *scheduler.Scheduler, db *database.DB, logger zerolog.Logger) *Server {
	return &Server{
		baseServer: newBaseServer(addr, NewStatusHandler(s, sched, db), logger),
		metrics:    NewMetrics(),
	}
}

// Metrics returns the Prometheus metrics.
func (s *Server) Metrics() *Metrics {
	return s.metrics
}

// StatusHandler handles the /status endpoint.
type StatusHandler struct {
	scraper   *scraper.Scraper
	scheduler *scheduler.Scheduler
	db        *database.DB
	startTime time.Time
}

// NewStatusHandler creates a new StatusHandler.
func NewStatusHandler(s *scraper.Scraper, sched *scheduler.Scheduler, db *database.DB) *StatusHandler {
	return &StatusHandler{
		scraper:   s,
		scheduler: sched,
		db:        db,
		startTime: time.Now(),
	}
}

// ServeHTTP implements the http.Handler interface.
func (h *StatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	response := models.StatusResponse{
		Status:        "healthy",
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
		Providers:     make(map[string]models.ProviderStatus),
	}

	// Get scheduler status
	response.SchedulerRunning, response.LastScheduledScrapeAt, response.NextScrapeAt = schedulerStatus(h.scheduler)

	// Get provider statuses
	for _, provider := range h.scraper.GetProviders() {
		metrics := h.scraper.GetMetrics(provider.Name())
		if metrics == nil {
			continue
		}

		snapshot := metrics.GetSnapshot()
		providerStatus := models.ProviderStatus{
			Enabled:            true,
			LastScrapeAt:       snapshot.LastScrapeAt,
			LastScrapeSuccess:  snapshot.LastScrapeSuccess,
			LastResponseTimeMs: snapshot.LastResponseTime.Milliseconds(),
			LastPrice:          snapshot.LastPrice,
			LastError:          snapshot.LastError,
			TotalRequests:      snapshot.TotalRequests,
			TotalErrors:        snapshot.TotalErrors,
			LastRawResponse:    snapshot.LastRawResponse,
		}

		response.Providers[provider.Name()] = providerStatus
	}

	// Get database status
	response.Database = h.getDatabaseStatus(ctx)

	writeJSON(w, response)
}

func (h *StatusHandler) getDatabaseStatus(ctx context.Context) models.DatabaseStatus {
	status := models.DatabaseStatus{
		Connected: databaseConnected(h.db),
	}

	if !status.Connected {
		return status
	}

	// Get total prices count
	count, err := h.db.GetTotalPricesCount(ctx)
	if err == nil {
		status.TotalPricesStored = count
	}

	return status
}
