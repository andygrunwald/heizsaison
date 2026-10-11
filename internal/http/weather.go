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

// weatherNamespace prefixes every metric the weather scraper exports.
const weatherNamespace = "heizsaison_weather"

// WeatherMetrics holds all Prometheus metrics for the weather scraper.
type WeatherMetrics struct {
	commonMetrics

	// Scrape metrics
	CurrentTemperature *prometheus.GaugeVec

	// Database metrics
	ObservationsStoredTotal *prometheus.GaugeVec
}

// NewWeatherMetrics creates and registers Prometheus metrics.
func NewWeatherMetrics() *WeatherMetrics {
	return &WeatherMetrics{
		commonMetrics: newCommonMetrics(weatherNamespace),
		CurrentTemperature: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: weatherNamespace,
				Name:      "current_temperature_celsius",
				Help:      "Current temperature in Celsius",
			},
			[]string{"provider"},
		),
		ObservationsStoredTotal: promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Namespace: weatherNamespace,
				Name:      "observations_stored_total",
				Help:      "Total number of observations stored in database by provider",
			},
			[]string{"provider"},
		),
	}
}

// RecordCurrentTemperature records the current temperature.
func (m *WeatherMetrics) RecordCurrentTemperature(provider string, temp float64) {
	m.CurrentTemperature.WithLabelValues(provider).Set(temp)
}

// RecordObservationsStored records the total number of observations stored for a provider.
func (m *WeatherMetrics) RecordObservationsStored(provider string, count float64) {
	m.ObservationsStoredTotal.WithLabelValues(provider).Set(count)
}

// WeatherServer represents the HTTP server for metrics and status endpoints.
type WeatherServer struct {
	baseServer
	metrics *WeatherMetrics
}

// NewWeatherServer creates a new HTTP server.
func NewWeatherServer(addr string, s *scraper.WeatherScraper, sched *scheduler.Scheduler, db *database.DB, logger zerolog.Logger) *WeatherServer {
	return &WeatherServer{
		baseServer: newBaseServer(addr, NewWeatherStatusHandler(s, sched, db), logger),
		metrics:    NewWeatherMetrics(),
	}
}

// Metrics returns the Prometheus metrics.
func (s *WeatherServer) Metrics() *WeatherMetrics {
	return s.metrics
}

// WeatherStatusHandler handles the /status endpoint.
type WeatherStatusHandler struct {
	scraper   *scraper.WeatherScraper
	scheduler *scheduler.Scheduler
	db        *database.DB
	startTime time.Time
}

// NewWeatherStatusHandler creates a new WeatherStatusHandler.
func NewWeatherStatusHandler(s *scraper.WeatherScraper, sched *scheduler.Scheduler, db *database.DB) *WeatherStatusHandler {
	return &WeatherStatusHandler{
		scraper:   s,
		scheduler: sched,
		db:        db,
		startTime: time.Now(),
	}
}

// ServeHTTP implements the http.Handler interface.
func (h *WeatherStatusHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	response := models.WeatherStatusResponse{
		Status:        "healthy",
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
		Providers:     make(map[string]models.WeatherProviderStatus),
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
		providerStatus := models.WeatherProviderStatus{
			Enabled:            true,
			LastScrapeAt:       snapshot.LastScrapeAt,
			LastScrapeSuccess:  snapshot.LastScrapeSuccess,
			LastResponseTimeMs: snapshot.LastResponseTime.Milliseconds(),
			LastTemperature:    snapshot.LastTemperature,
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

func (h *WeatherStatusHandler) getDatabaseStatus(ctx context.Context) models.WeatherDatabaseStatus {
	status := models.WeatherDatabaseStatus{
		Connected: databaseConnected(h.db),
	}

	if !status.Connected {
		return status
	}

	// Get total observations count
	count, err := h.db.GetTotalWeatherCount(ctx)
	if err == nil {
		status.TotalObservationsStored = count
	}

	return status
}
