// Package openweather provides a weather provider using the OpenWeather One Call API 3.0.
package openweather

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog"

	"github.com/andygrunwald/heizsaison/internal/models"
	"github.com/andygrunwald/heizsaison/internal/useragent"
)

const (
	providerName  = "openweather"
	daySummaryURL = "https://api.openweathermap.org/data/3.0/onecall/day_summary"
)

// daySummaryResponse represents the OpenWeather day_summary API response.
type daySummaryResponse struct {
	Lat           float64       `json:"lat"`
	Lon           float64       `json:"lon"`
	TZ            string        `json:"tz"`
	Date          string        `json:"date"`
	Units         string        `json:"units"`
	CloudCover    cloudCover    `json:"cloud_cover"`
	Humidity      humidity      `json:"humidity"`
	Precipitation precipitation `json:"precipitation"`
	Temperature   temperature   `json:"temperature"`
	Pressure      pressure      `json:"pressure"`
	Wind          wind          `json:"wind"`
}

type cloudCover struct {
	Afternoon *float64 `json:"afternoon"`
}

type humidity struct {
	Afternoon *float64 `json:"afternoon"`
}

type precipitation struct {
	Total *float64 `json:"total"`
}

type temperature struct {
	Min       *float64 `json:"min"`
	Max       *float64 `json:"max"`
	Morning   *float64 `json:"morning"`
	Afternoon *float64 `json:"afternoon"`
	Evening   *float64 `json:"evening"`
	Night     *float64 `json:"night"`
}

type pressure struct {
	Afternoon *float64 `json:"afternoon"`
}

type wind struct {
	Max windMax `json:"max"`
}

type windMax struct {
	Speed     *float64 `json:"speed"`
	Direction *float64 `json:"direction"`
}

// Provider implements the weatherapi.Provider interface for OpenWeather.
type Provider struct {
	logger   zerolog.Logger
	client   *http.Client
	apiKey   string
	minDelay int
	maxDelay int
}

// New creates a new OpenWeather provider.
// minDelay and maxDelay are in seconds, used for rate limiting during backfill.
func New(logger zerolog.Logger, apiKey string, minDelay, maxDelay int) *Provider {
	return &Provider{
		logger: logger.With().Str("provider", providerName).Logger(),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey:   apiKey,
		minDelay: minDelay,
		maxDelay: maxDelay,
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return providerName
}

// SupportsBackfill returns true — OpenWeather has data from 1979+.
func (p *Provider) SupportsBackfill() bool {
	return true
}

// RequiresAPIKey returns true.
func (p *Provider) RequiresAPIKey() bool {
	return true
}

// FetchCurrentWeather fetches today's weather for the given location.
func (p *Provider) FetchCurrentWeather(ctx context.Context, lat, lon float64) ([]models.WeatherResult, error) {
	today := time.Now().Format("2006-01-02")
	result, rawBody, err := p.fetchSingleDay(ctx, lat, lon, today)
	if err != nil {
		return nil, err
	}
	result.RawResponse = rawBody
	return []models.WeatherResult{result}, nil
}

// FetchHistoricalWeather fetches weather for a date range, one day at a time.
func (p *Provider) FetchHistoricalWeather(ctx context.Context, lat, lon float64, from, to time.Time) ([]models.WeatherResult, error) {
	var results []models.WeatherResult

	for d := from; !d.After(to); d = d.AddDate(0, 0, 1) {
		select {
		case <-ctx.Done():
			return results, ctx.Err()
		default:
		}

		dateStr := d.Format("2006-01-02")
		result, rawBody, err := p.fetchSingleDay(ctx, lat, lon, dateStr)
		if err != nil {
			p.logger.Error().Err(err).Str("date", dateStr).Msg("failed to fetch day, skipping")
			continue
		}
		result.RawResponse = rawBody
		results = append(results, result)

		// Rate limiting: sleep between requests (except after the last one)
		if !d.Equal(to) && p.minDelay > 0 {
			delay := p.minDelay
			if p.maxDelay > p.minDelay {
				delay = p.minDelay + rand.Intn(p.maxDelay-p.minDelay+1)
			}
			time.Sleep(time.Duration(delay) * time.Second)
		}
	}

	return results, nil
}

func (p *Provider) fetchSingleDay(ctx context.Context, lat, lon float64, date string) (models.WeatherResult, []byte, error) {
	u, err := url.Parse(daySummaryURL)
	if err != nil {
		return models.WeatherResult{}, nil, fmt.Errorf("parsing URL: %w", err)
	}

	q := u.Query()
	q.Set("lat", fmt.Sprintf("%.4f", lat))
	q.Set("lon", fmt.Sprintf("%.4f", lon))
	q.Set("date", date)
	q.Set("appid", p.apiKey)
	q.Set("units", "metric")
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return models.WeatherResult{}, nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", useragent.Random())
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return models.WeatherResult{}, nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return models.WeatherResult{}, nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return models.WeatherResult{}, nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp daySummaryResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return models.WeatherResult{}, nil, fmt.Errorf("parsing response JSON: %w", err)
	}

	result := p.mapResponse(apiResp, lat, lon)
	return result, body, nil
}

func (p *Provider) mapResponse(resp daySummaryResponse, lat, lon float64) models.WeatherResult {
	parsedDate, _ := time.Parse("2006-01-02", resp.Date)

	result := models.WeatherResult{
		Date:      parsedDate,
		Provider:  providerName,
		Latitude:  models.RoundCoord(lat),
		Longitude: models.RoundCoord(lon),
		FetchedAt: time.Now(),
	}

	result.TemperatureMinC = resp.Temperature.Min
	result.TemperatureMaxC = resp.Temperature.Max

	// Calculate mean temperature from available data points
	if resp.Temperature.Min != nil && resp.Temperature.Max != nil {
		mean := (*resp.Temperature.Min + *resp.Temperature.Max) / 2
		result.TemperatureMeanC = &mean
	}

	result.PrecipitationMmSum = resp.Precipitation.Total

	// Convert wind speed from m/s to km/h
	if resp.Wind.Max.Speed != nil {
		kmh := *resp.Wind.Max.Speed * 3.6
		result.WindSpeedMaxKmh = &kmh
	}

	result.CloudCoverPercent = resp.CloudCover.Afternoon
	result.HumidityPercent = resp.Humidity.Afternoon
	result.PressureHpa = resp.Pressure.Afternoon

	return result
}
