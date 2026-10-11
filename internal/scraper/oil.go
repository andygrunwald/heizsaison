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

// PrometheusMetrics defines the interface for recording Prometheus metrics.
type PrometheusMetrics interface {
	commonPrometheusMetrics

	RecordCurrentPrice(provider, scope, productType string, price float64)
	RecordPricesStored(provider string, count float64)
}

// Metrics holds scraping metrics for a provider.
type Metrics struct {
	commonMetrics

	LastPrice *float64
}

// MetricsSnapshot is a thread-safe copy of Metrics data.
type MetricsSnapshot struct {
	commonSnapshot

	LastPrice *float64
}

// GetSnapshot returns a thread-safe snapshot of the metrics.
func (m *Metrics) GetSnapshot() MetricsSnapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return MetricsSnapshot{
		commonSnapshot: m.snapshot(),
		LastPrice:      m.LastPrice,
	}
}

// Scraper orchestrates scraping from multiple providers.
type Scraper struct {
	db               *database.DB
	providers        map[string]api.Provider
	providerMetrics  map[string]*Metrics
	promMetrics      PrometheusMetrics
	storeRawResponse bool
	logger           zerolog.Logger
	mu               sync.RWMutex
}

// New creates a new Scraper.
func New(db *database.DB, storeRawResponse bool, logger zerolog.Logger) *Scraper {
	return &Scraper{
		db:               db,
		providers:        make(map[string]api.Provider),
		providerMetrics:  make(map[string]*Metrics),
		storeRawResponse: storeRawResponse,
		logger:           logger.With().Str("component", "oil").Logger(),
	}
}

// RegisterProvider registers a provider with the scraper.
func (s *Scraper) RegisterProvider(provider api.Provider) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.providers[provider.Name()] = provider
	s.providerMetrics[provider.Name()] = &Metrics{}
}

// GetProviders returns all registered providers.
func (s *Scraper) GetProviders() []api.Provider {
	s.mu.RLock()
	defer s.mu.RUnlock()
	providers := make([]api.Provider, 0, len(s.providers))
	for _, p := range s.providers {
		providers = append(providers, p)
	}
	return providers
}

// GetProviderNames returns the names of all registered providers.
func (s *Scraper) GetProviderNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	names := make([]string, 0, len(s.providers))
	for name := range s.providers {
		names = append(names, name)
	}
	return names
}

// GetMetrics returns the metrics for a provider.
func (s *Scraper) GetMetrics(providerName string) *Metrics {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.providerMetrics[providerName]
}

// SetPrometheusMetrics sets the Prometheus metrics recorder.
func (s *Scraper) SetPrometheusMetrics(m PrometheusMetrics) {
	s.promMetrics = m
}

// ScrapeAll scrapes current prices from all registered providers.
func (s *Scraper) ScrapeAll(ctx context.Context) error {
	s.mu.RLock()
	providers := make([]api.Provider, 0, len(s.providers))
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

// ScrapeProvider scrapes current prices from a specific provider.
func (s *Scraper) ScrapeProvider(ctx context.Context, providerName string) error {
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

	prices, err := provider.FetchCurrentPrices(ctx)
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
		if len(prices) > 0 {
			metrics.LastPrice = &prices[0].PricePer100L
			if len(prices[0].RawResponse) > 0 {
				// Store a truncated version for status endpoint
				rawResp := string(prices[0].RawResponse)
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
			Msg("failed to fetch prices")
		return err
	}

	// Record successful scrape timestamp
	if s.promMetrics != nil {
		s.promMetrics.RecordLastScrape(providerName, float64(time.Now().Unix()))
	}

	s.logger.Info().
		Str("provider", providerName).
		Int("count", len(prices)).
		Dur("duration", duration).
		Msg("fetched prices")

	// Store prices in database
	var storedCount float64
	for _, price := range prices {
		// Check if already exists
		exists, err := s.db.ExistsForDate(ctx, price.Provider, price.ProductType, price.Date, price.ZipCode)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("provider", price.Provider).
				Str("product_type", price.ProductType).
				Str("date", price.Date.Format("2006-01-02")).
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
				Str("provider", price.Provider).
				Str("product_type", price.ProductType).
				Str("date", price.Date.Format("2006-01-02")).
				Msg("price already exists, skipping")
			continue
		}

		if err := s.db.InsertPrice(ctx, price, s.storeRawResponse); err != nil {
			s.logger.Error().
				Err(err).
				Str("provider", price.Provider).
				Str("product_type", price.ProductType).
				Str("date", price.Date.Format("2006-01-02")).
				Msg("failed to insert price")
			if s.promMetrics != nil {
				s.promMetrics.RecordDBOperation("insert", "error")
			}
		} else {
			storedCount++
			if s.promMetrics != nil {
				s.promMetrics.RecordDBOperation("insert", "success")
				s.promMetrics.RecordCurrentPrice(price.Provider, string(price.Scope), price.ProductType, price.PricePer100L)
			}
		}
	}

	// Record total prices stored for this provider
	if s.promMetrics != nil && storedCount > 0 {
		s.promMetrics.RecordPricesStored(providerName, storedCount)
	}

	return nil
}

// Backfill backfills historical data from a provider.
func (s *Scraper) Backfill(ctx context.Context, providerName string, from, to time.Time, minDelay, maxDelay int) error {
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

	s.logger.Info().
		Str("provider", providerName).
		Str("from", from.Format("2006-01-02")).
		Str("to", to.Format("2006-01-02")).
		Msg("starting backfill")

	// Fetch all historical prices at once (HeizOel24 supports date range queries)
	prices, err := provider.FetchHistoricalPrices(ctx, from, to)
	if err != nil {
		return err
	}

	s.logger.Info().
		Str("provider", providerName).
		Int("count", len(prices)).
		Msg("fetched historical prices")

	// Store prices in database
	inserted := 0
	skipped := 0
	for _, price := range prices {
		// Check if already exists
		exists, err := s.db.ExistsForDate(ctx, price.Provider, price.ProductType, price.Date, price.ZipCode)
		if err != nil {
			s.logger.Error().
				Err(err).
				Str("provider", price.Provider).
				Str("date", price.Date.Format("2006-01-02")).
				Msg("failed to check existence")
			continue
		}

		if exists {
			skipped++
			continue
		}

		if err := s.db.InsertPrice(ctx, price, s.storeRawResponse); err != nil {
			s.logger.Error().
				Err(err).
				Str("provider", price.Provider).
				Str("date", price.Date.Format("2006-01-02")).
				Msg("failed to insert price")
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
func (s *Scraper) HasScrapedToday(ctx context.Context, providerName string) (bool, error) {
	s.mu.RLock()
	provider, ok := s.providers[providerName]
	s.mu.RUnlock()

	if !ok {
		return false, nil
	}

	// Get today's date
	today := time.Now().Truncate(24 * time.Hour)

	// Check for each possible product type
	// For simplicity, we'll just check if any record exists for today
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Use the provider's standard product type or check the database
	zipCode := ""
	if provider.PriceScope() == models.PriceScopeLocal {
		// For local providers, we'd need to know the zip code
		// This is a simplification - in practice you'd want to pass this
		return false, nil
	}

	// Check if a record exists for today
	exists, err := s.db.ExistsForDate(ctx, providerName, "standard", today, zipCode)
	if err != nil {
		return false, err
	}

	return exists, nil
}
