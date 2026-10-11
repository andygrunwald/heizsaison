// Package openmeteo provides a weather provider using the Open-Meteo API.
package openmeteo

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rs/zerolog"

	"github.com/andygrunwald/heizsaison/internal/models"
	"github.com/andygrunwald/heizsaison/internal/useragent"
)

const (
	providerName    = "openmeteo"
	forecastBaseURL = "https://api.open-meteo.com/v1/forecast"
	archiveBaseURL  = "https://archive-api.open-meteo.com/v1/archive"
	timezone        = "Europe/Berlin"
	dailyParams     = "temperature_2m_max,temperature_2m_min,temperature_2m_mean,precipitation_sum,wind_speed_10m_max,wind_gusts_10m_max,sunshine_duration,cloud_cover_mean,relative_humidity_2m_mean,surface_pressure_mean"
)

// apiResponse represents the Open-Meteo API response.
type apiResponse struct {
	Latitude   float64    `json:"latitude"`
	Longitude  float64    `json:"longitude"`
	Timezone   string     `json:"timezone"`
	Daily      dailyData  `json:"daily"`
	DailyUnits dailyUnits `json:"daily_units"`
}

type dailyData struct {
	Time                 []string   `json:"time"`
	TemperatureMax       []*float64 `json:"temperature_2m_max"`
	TemperatureMin       []*float64 `json:"temperature_2m_min"`
	TemperatureMean      []*float64 `json:"temperature_2m_mean"`
	PrecipitationSum     []*float64 `json:"precipitation_sum"`
	WindSpeedMax         []*float64 `json:"wind_speed_10m_max"`
	WindGustMax          []*float64 `json:"wind_gusts_10m_max"`
	SunshineDuration     []*float64 `json:"sunshine_duration"`
	CloudCoverMean       []*float64 `json:"cloud_cover_mean"`
	RelativeHumidityMean []*float64 `json:"relative_humidity_2m_mean"`
	SurfacePressureMean  []*float64 `json:"surface_pressure_mean"`
}

type dailyUnits struct {
	Time                 string `json:"time"`
	TemperatureMax       string `json:"temperature_2m_max"`
	TemperatureMin       string `json:"temperature_2m_min"`
	TemperatureMean      string `json:"temperature_2m_mean"`
	PrecipitationSum     string `json:"precipitation_sum"`
	WindSpeedMax         string `json:"wind_speed_10m_max"`
	WindGustMax          string `json:"wind_gusts_10m_max"`
	SunshineDuration     string `json:"sunshine_duration"`
	CloudCoverMean       string `json:"cloud_cover_mean"`
	RelativeHumidityMean string `json:"relative_humidity_2m_mean"`
	SurfacePressureMean  string `json:"surface_pressure_mean"`
}

// Provider implements the weatherapi.Provider interface for Open-Meteo.
type Provider struct {
	logger zerolog.Logger
	client *http.Client
}

// New creates a new Open-Meteo provider.
func New(logger zerolog.Logger) *Provider {
	return &Provider{
		logger: logger.With().Str("provider", providerName).Logger(),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return providerName
}

// SupportsBackfill returns true — Open-Meteo supports historical data back to 1940.
func (p *Provider) SupportsBackfill() bool {
	return true
}

// RequiresAPIKey returns false — Open-Meteo free tier requires no API key.
func (p *Provider) RequiresAPIKey() bool {
	return false
}

// FetchCurrentWeather fetches today's weather for the given location.
func (p *Provider) FetchCurrentWeather(ctx context.Context, lat, lon float64) ([]models.WeatherResult, error) {
	u, err := url.Parse(forecastBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing forecast URL: %w", err)
	}

	q := u.Query()
	q.Set("latitude", fmt.Sprintf("%.4f", lat))
	q.Set("longitude", fmt.Sprintf("%.4f", lon))
	q.Set("daily", dailyParams)
	q.Set("timezone", timezone)
	q.Set("forecast_days", "1")
	q.Set("past_days", "1")
	u.RawQuery = q.Encode()

	return p.fetch(ctx, u.String(), lat, lon)
}

// FetchHistoricalWeather fetches weather for a date range.
func (p *Provider) FetchHistoricalWeather(ctx context.Context, lat, lon float64, from, to time.Time) ([]models.WeatherResult, error) {
	u, err := url.Parse(archiveBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing archive URL: %w", err)
	}

	q := u.Query()
	q.Set("latitude", fmt.Sprintf("%.4f", lat))
	q.Set("longitude", fmt.Sprintf("%.4f", lon))
	q.Set("daily", dailyParams)
	q.Set("timezone", timezone)
	q.Set("start_date", from.Format("2006-01-02"))
	q.Set("end_date", to.Format("2006-01-02"))
	u.RawQuery = q.Encode()

	return p.fetch(ctx, u.String(), lat, lon)
}

func (p *Provider) fetch(ctx context.Context, requestURL string, lat, lon float64) ([]models.WeatherResult, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", useragent.Random())
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response JSON: %w", err)
	}

	return p.parseResponse(apiResp, lat, lon, body)
}

func (p *Provider) parseResponse(resp apiResponse, lat, lon float64, rawBody []byte) ([]models.WeatherResult, error) {
	daily := resp.Daily
	results := make([]models.WeatherResult, 0, len(daily.Time))
	now := time.Now()

	for i, dateStr := range daily.Time {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			p.logger.Warn().Str("date", dateStr).Msg("skipping unparseable date")
			continue
		}

		result := models.WeatherResult{
			Date:        date,
			Provider:    providerName,
			Latitude:    models.RoundCoord(lat),
			Longitude:   models.RoundCoord(lon),
			RawResponse: rawBody,
			FetchedAt:   now,
		}

		if i < len(daily.TemperatureMin) {
			result.TemperatureMinC = daily.TemperatureMin[i]
		}
		if i < len(daily.TemperatureMax) {
			result.TemperatureMaxC = daily.TemperatureMax[i]
		}
		if i < len(daily.TemperatureMean) {
			result.TemperatureMeanC = daily.TemperatureMean[i]
		}
		if i < len(daily.PrecipitationSum) {
			result.PrecipitationMmSum = daily.PrecipitationSum[i]
		}
		if i < len(daily.WindSpeedMax) {
			result.WindSpeedMaxKmh = daily.WindSpeedMax[i]
		}
		if i < len(daily.WindGustMax) {
			result.WindGustMaxKmh = daily.WindGustMax[i]
		}
		if i < len(daily.SunshineDuration) {
			result.SunshineDurationS = daily.SunshineDuration[i]
		}
		if i < len(daily.CloudCoverMean) {
			result.CloudCoverPercent = daily.CloudCoverMean[i]
		}
		if i < len(daily.RelativeHumidityMean) {
			result.HumidityPercent = daily.RelativeHumidityMean[i]
		}
		if i < len(daily.SurfacePressureMean) {
			result.PressureHpa = daily.SurfacePressureMean[i]
		}

		results = append(results, result)
	}

	return results, nil
}
