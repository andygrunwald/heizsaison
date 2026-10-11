package scraper

import (
	"context"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/andygrunwald/heizsaison/internal/api"
	"github.com/andygrunwald/heizsaison/internal/database"
	"github.com/andygrunwald/heizsaison/internal/models"
)

// WeatherPrometheusMetrics defines the interface for recording Prometheus metrics.
type WeatherPrometheusMetrics interface {
	commonPrometheusMetrics

	RecordCurrentTemperature(provider string, temp float64)
	RecordObservationsStored(provider string, count float64)
}

// WeatherMetrics holds scraping metrics for a provider.
type WeatherMetrics struct {
	commonMetrics

	LastTemperature *float64
}

// WeatherMetricsSnapshot is a thread-safe copy of WeatherMetrics data.
type WeatherMetricsSnapshot struct {
	commonSnapshot

	LastTemperature *float64
}

// GetSnapshot returns a thread-safe snapshot of the metrics.
func (m *WeatherMetrics) GetSnapshot() WeatherMetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return WeatherMetricsSnapshot{
		commonSnapshot:  m.snapshot(),
		LastTemperature: m.LastTemperature,
	}
}

// WeatherScraper orchestrates scraping from multiple weather providers.
type WeatherScraper struct {
	db               *database.DB
	providers        map[string]api.WeatherProvider
	providerMetrics  map[string]*WeatherMetrics
	promMetrics      WeatherPrometheusMetrics
	storeRawResponse bool
	latitude         float64
	longitude        float64
	logger           zerolog.Logger
	mu               sync.RWMutex
}

// NewWeather creates a new WeatherScraper.
func NewWeather(db *database.DB, storeRawResponse bool, latitude, longitude float64, logger zerolog.Logger) *WeatherScraper {
	return &WeatherScraper{
		db:               db,
		providers:        make(map[string]api.WeatherProvider),
		providerMetrics:  make(map[string]*WeatherMetrics),
		storeRawResponse: storeRawResponse,
		latitude:         models.RoundCoord(latitude),
		longitude:        models.RoundCoord(longitude),
		logger:           logger.With().Str("component", "weather").Logger(),
	}
}

// RegisterProvider registers a provider with the scraper.
func (s *WeatherScraper) RegisterProvider(provider api.WeatherProvider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[provider.Name()] = provider
	s.providerMetrics[provider.Name()] = &WeatherMetrics{}
}

// GetProviders returns all registered providers.
func (s *WeatherScraper) GetProviders() []api.WeatherProvider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	providers := make([]api.WeatherProvider, 0, len(s.providers))
	for _, p := range s.providers {
		providers = append(providers, p)
	}
	return providers
}

// GetProviderNames returns the names of all registered providers.
func (s *WeatherScraper) GetProviderNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.providers))
	for name := range s.providers {
		names = append(names, name)
	}
	return names
}

// GetMetrics returns the metrics for a provider.
func (s *WeatherScraper) GetMetrics(providerName string) *WeatherMetrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.providerMetrics[providerName]
}

// SetPrometheusMetrics sets the Prometheus metrics recorder.
func (s *WeatherScraper) SetPrometheusMetrics(m WeatherPrometheusMetrics) {
	s.promMetrics = m
}

// ScrapeAll scrapes current weather from all registered providers.
func (s *WeatherScraper) ScrapeAll(ctx context.Context) error {
	s.mu.RLock()
	providers := make([]api.WeatherProvider, 0, len(s.providers))
	for _, p := range s.providers {
		providers = append(providers, p)
	}
	s.mu.RUnlock()

	for _, provider := range providers {
		if err := s.ScrapeProvider(ctx, provider.Name()); err != nil {
			s.logger.Error().
				Err(err).
				Str("provider", provider.Name()).
				Msg("failed to scrape provider")
		}
	}

	return nil
}

// ScrapeProvider scrapes current weather from a specific provider.
func (s *WeatherScraper) ScrapeProvider(ctx context.Context, providerName string) error {
	s.mu.RLock()
	provider, ok := s.providers[providerName]
	metrics := s.providerMetrics[providerName]
	s.mu.RUnlock()

	if !ok {
		s.logger.Warn().Str("provider", providerName).Msg("provider not found")
		return nil
	}

	s.logger.Info().Str("provider", providerName).Msg("scraping provider")

	start := time.Now()
	metrics.mu.Lock()
	metrics.TotalRequests++
	metrics.mu.Unlock()

	observations, err := provider.FetchCurrentWeather(ctx, s.latitude, s.longitude)
	duration := time.Since(start)

	now := time.Now()
	metrics.mu.Lock()
	metrics.LastScrapeAt = &now
	metrics.LastResponseTime = duration
	if err != nil {
		metrics.TotalErrors++
		metrics.LastScrapeSuccess = false
		errStr := err.Error()
		metrics.LastError = &errStr
	} else {
		metrics.LastScrapeSuccess = true
		metrics.LastError = nil
		if len(observations) > 0 && observations[0].TemperatureMeanC != nil {
			metrics.LastTemperature = observations[0].TemperatureMeanC
			if len(observations[0].RawResponse) > 0 {
				rawResp := string(observations[0].RawResponse)
				if len(rawResp) > 10000 {
					rawResp = rawResp[:10000] + "..."
				}
				metrics.LastRawResponse = rawResp
			}
		}
	}
	metrics.mu.Unlock()

	// Record Prometheus metrics for API request
	if s.promMetrics != nil {
		status := "success"
		if err != nil {
			status = "error"
		}
		s.promMetrics.RecordAPIRequest(providerName, status, duration.Seconds())
	}

	if err != nil {
		s.logger.Error().
			Err(err).
			Str("provider", providerName).
			Dur("duration", duration).
			Msg("failed to fetch weather")
		return err
	}

	// Record successful scrape timestamp
	if s.promMetrics != nil {
		s.promMetrics.RecordLastScrape(providerName, float64(time.Now().Unix()))
	}

	s.logger.Info().
		Str("provider", providerName).
		Int("count", len(observations)).
		Dur("duration", duration).
		Msg("fetched weather observations")

	// Store observations in database
	var storedCount float64
	for _, obs := range observations {
		// Check if already exists
		exists, err := s.db.WeatherExistsForDate(ctx, obs.Provider, obs.Date, s.latitude, s.longitude)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("provider", obs.Provider).
				Str("date", obs.Date.Format("2006-01-02")).
				Msg("failed to check existence")
			if s.promMetrics != nil {
				s.promMetrics.RecordDBOperation("select", "error")
			}
			continue
		}
		if s.promMetrics != nil {
			s.promMetrics.RecordDBOperation("select", "success")
		}

		if exists {
			s.logger.Debug().
				Str("provider", obs.Provider).
				Str("date", obs.Date.Format("2006-01-02")).
				Msg("observation already exists, skipping")
			continue
		}

		if err := s.db.InsertWeatherObservation(ctx, obs, s.storeRawResponse); err != nil {
			s.logger.Error().
				Err(err).
				Str("provider", obs.Provider).
				Str("date", obs.Date.Format("2006-01-02")).
				Msg("failed to insert observation")
			if s.promMetrics != nil {
				s.promMetrics.RecordDBOperation("insert", "error")
			}
		} else {
			storedCount++
			if s.promMetrics != nil {
				s.promMetrics.RecordDBOperation("insert", "success")
				if obs.TemperatureMeanC != nil {
					s.promMetrics.RecordCurrentTemperature(obs.Provider, *obs.TemperatureMeanC)
				}
			}
		}
	}

	// Record total observations stored for this provider
	if s.promMetrics != nil && storedCount > 0 {
		s.promMetrics.RecordObservationsStored(providerName, storedCount)
	}

	return nil
}

// Backfill backfills historical weather data from a provider.
func (s *WeatherScraper) Backfill(ctx context.Context, providerName string, from, to time.Time, minDelay, maxDelay int) error {
	s.mu.RLock()
	provider, ok := s.providers[providerName]
	s.mu.RUnlock()

	if !ok {
		s.logger.Warn().Str("provider", providerName).Msg("provider not found")
		return nil
	}

	if !provider.SupportsBackfill() {
		s.logger.Warn().
			Str("provider", providerName).
			Msg("provider does not support backfill")
		return nil
	}

	// Warn if backfill range is large for providers with rate limits
	days := int(to.Sub(from).Hours() / 24)
	if days > 900 && provider.RequiresAPIKey() {
		s.logger.Warn().
			Str("provider", providerName).
			Int("days", days).
			Msg("large backfill range for rate-limited provider, may exceed daily API quota")
	}

	s.logger.Info().
		Str("provider", providerName).
		Str("from", from.Format("2006-01-02")).
		Str("to", to.Format("2006-01-02")).
		Int("days", days).
		Msg("starting backfill")

	observations, err := provider.FetchHistoricalWeather(ctx, s.latitude, s.longitude, from, to)
	if err != nil {
		return err
	}

	s.logger.Info().
		Str("provider", providerName).
		Int("count", len(observations)).
		Msg("fetched historical weather observations")

	// Store observations in database
	inserted := 0
	skipped := 0
	for _, obs := range observations {
		exists, err := s.db.WeatherExistsForDate(ctx, obs.Provider, obs.Date, s.latitude, s.longitude)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("provider", obs.Provider).
				Str("date", obs.Date.Format("2006-01-02")).
				Msg("failed to check existence")
			continue
		}

		if exists {
			skipped++
			continue
		}

		if err := s.db.InsertWeatherObservation(ctx, obs, s.storeRawResponse); err != nil {
			s.logger.Error().
				Err(err).
				Str("provider", obs.Provider).
				Str("date", obs.Date.Format("2006-01-02")).
				Msg("failed to insert observation")
		} else {
			inserted++
		}
	}

	s.logger.Info().
		Str("provider", providerName).
		Int("inserted", inserted).
		Int("skipped", skipped).
		Msg("backfill completed")

	return nil
}

// HasScrapedToday checks if the provider has been scraped today.
func (s *WeatherScraper) HasScrapedToday(ctx context.Context, providerName string) (bool, error) {
	s.mu.RLock()
	_, ok := s.providers[providerName]
	s.mu.RUnlock()

	if !ok {
		return false, nil
	}

	today := time.Now().Truncate(24 * time.Hour)

	exists, err := s.db.WeatherExistsForDate(ctx, providerName, today, s.latitude, s.longitude)
	if err != nil {
		return false, err
	}

	return exists, nil
}
