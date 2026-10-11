package dwdcdc

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/andygrunwald/heizsaison/internal/models"
)

const (
	providerName  = "dwdcdc"
	recentBaseURL = "https://opendata.dwd.de/climate_environment/CDC/observations_germany/climate/daily/kl/recent"
	histBaseURL   = "https://opendata.dwd.de/climate_environment/CDC/observations_germany/climate/daily/kl/historical"
)

// Provider implements the weatherapi.Provider interface for DWD CDC-OpenData.
type Provider struct {
	logger  zerolog.Logger
	client  *http.Client
	mu      sync.Mutex
	station *Station // cached after first lookup
}

// New creates a new DWD CDC-OpenData provider.
func New(logger zerolog.Logger) *Provider {
	return &Provider{
		logger: logger.With().Str("provider", providerName).Logger(),
		client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return providerName
}

// SupportsBackfill returns true — DWD has 50+ years of historical data.
func (p *Provider) SupportsBackfill() bool {
	return true
}

// RequiresAPIKey returns false — DWD CDC-OpenData is freely accessible.
func (p *Provider) RequiresAPIKey() bool {
	return false
}

// FetchCurrentWeather fetches the most recent available weather observation.
// DWD data has a ~1 day lag, so this typically returns yesterday's data.
func (p *Provider) FetchCurrentWeather(ctx context.Context, lat, lon float64) ([]models.WeatherResult, error) {
	station, err := p.ensureStation(ctx, lat, lon)
	if err != nil {
		return nil, err
	}

	// Download recent data ZIP
	zipURL := fmt.Sprintf("%s/tageswerte_KL_%s_akt.zip", recentBaseURL, station.ID)
	data, err := downloadAndExtractZIP(ctx, p.client, zipURL)
	if err != nil {
		return nil, fmt.Errorf("downloading recent data: %w", err)
	}

	records, err := parseDataFile(data)
	if err != nil {
		return nil, fmt.Errorf("parsing data file: %w", err)
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("no records found in recent data")
	}

	// Sort by date descending, return the most recent record
	sort.Slice(records, func(i, j int) bool {
		return records[i].Date.After(records[j].Date)
	})

	latest := records[0]
	p.logger.Debug().
		Str("date", latest.Date.Format("2006-01-02")).
		Str("station", station.Name).
		Msg("returning most recent available observation")

	result := recordToWeatherResult(latest, lat, lon, data)
	return []models.WeatherResult{result}, nil
}

// FetchHistoricalWeather fetches weather observations for a date range.
func (p *Provider) FetchHistoricalWeather(ctx context.Context, lat, lon float64, from, to time.Time) ([]models.WeatherResult, error) {
	station, err := p.ensureStation(ctx, lat, lon)
	if err != nil {
		return nil, err
	}

	var allRecords []dailyRecord

	// Try historical data first (quality-controlled)
	histURL := fmt.Sprintf("%s/tageswerte_KL_%s_%s_%s_hist.zip",
		histBaseURL, station.ID,
		station.DateFrom.Format("20060102"),
		station.DateTo.Format("20060102"))

	histData, err := downloadAndExtractZIP(ctx, p.client, histURL)
	if err != nil {
		p.logger.Debug().Err(err).Msg("historical data not available, trying recent only")
	} else {
		histRecords, err := parseDataFile(histData)
		if err != nil {
			p.logger.Warn().Err(err).Msg("failed to parse historical data")
		} else {
			allRecords = append(allRecords, histRecords...)
		}
	}

	// Also fetch recent data (which may overlap or extend beyond historical)
	recentURL := fmt.Sprintf("%s/tageswerte_KL_%s_akt.zip", recentBaseURL, station.ID)
	recentData, err := downloadAndExtractZIP(ctx, p.client, recentURL)
	if err != nil {
		p.logger.Debug().Err(err).Msg("recent data not available")
	} else {
		recentRecords, err := parseDataFile(recentData)
		if err != nil {
			p.logger.Warn().Err(err).Msg("failed to parse recent data")
		} else {
			allRecords = append(allRecords, recentRecords...)
		}
	}

	if len(allRecords) == 0 {
		return nil, fmt.Errorf("no data available for station %s", station.ID)
	}

	// Deduplicate by date (prefer later entries which may be more recent/corrected)
	seen := make(map[string]int) // date → index in deduped
	var deduped []dailyRecord
	for _, rec := range allRecords {
		key := rec.Date.Format("2006-01-02")
		if idx, exists := seen[key]; exists {
			deduped[idx] = rec // overwrite with later entry
		} else {
			seen[key] = len(deduped)
			deduped = append(deduped, rec)
		}
	}

	// Filter to requested date range
	filtered := filterByDateRange(deduped, from, to)

	// Sort by date
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Date.Before(filtered[j].Date)
	})

	// Use the combined raw data for RawResponse
	var rawBody []byte
	if histData != nil {
		rawBody = histData
	} else if recentData != nil {
		rawBody = recentData
	}

	results := make([]models.WeatherResult, 0, len(filtered))
	for _, rec := range filtered {
		results = append(results, recordToWeatherResult(rec, lat, lon, rawBody))
	}

	p.logger.Info().
		Str("station", station.Name).
		Int("count", len(results)).
		Msg("fetched historical weather observations")

	return results, nil
}

// ensureStation lazily discovers and caches the nearest DWD station.
func (p *Provider) ensureStation(ctx context.Context, lat, lon float64) (*Station, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.station != nil {
		return p.station, nil
	}

	stations, err := fetchStationList(ctx, p.client)
	if err != nil {
		return nil, fmt.Errorf("fetching station list: %w", err)
	}

	nearest, err := findNearestStation(stations, lat, lon)
	if err != nil {
		return nil, err
	}

	dist := haversineDistance(lat, lon, nearest.Latitude, nearest.Longitude)
	p.logger.Info().
		Str("station_id", nearest.ID).
		Str("station_name", nearest.Name).
		Float64("distance_km", dist).
		Float64("station_lat", nearest.Latitude).
		Float64("station_lon", nearest.Longitude).
		Msg("selected nearest DWD station")

	p.station = &nearest
	return p.station, nil
}
