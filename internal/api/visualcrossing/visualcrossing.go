// Package visualcrossing provides a weather provider using the Visual Crossing Timeline API.
package visualcrossing

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/andygrunwald/heizsaison/internal/models"
	"github.com/andygrunwald/heizsaison/internal/useragent"
)

const (
	providerName = "visualcrossing"
	baseURL      = "https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline"
)

// apiResponse represents the Visual Crossing Timeline API response.
type apiResponse struct {
	QueryCost       int       `json:"queryCost"`
	Latitude        float64   `json:"latitude"`
	Longitude       float64   `json:"longitude"`
	ResolvedAddress string    `json:"resolvedAddress"`
	Timezone        string    `json:"timezone"`
	Days            []dayData `json:"days"`
}

type dayData struct {
	DateTime    string   `json:"datetime"`
	TempMax     *float64 `json:"tempmax"`
	TempMin     *float64 `json:"tempmin"`
	Temp        *float64 `json:"temp"`
	FeelsLike   *float64 `json:"feelslike"`
	Humidity    *float64 `json:"humidity"`
	Precip      *float64 `json:"precip"`
	WindSpeed   *float64 `json:"windspeed"`
	WindGust    *float64 `json:"windgust"`
	WindDir     *float64 `json:"winddir"`
	CloudCover  *float64 `json:"cloudcover"`
	Pressure    *float64 `json:"pressure"`
	UVIndex     *float64 `json:"uvindex"`
	Sunrise     string   `json:"sunrise"`
	Sunset      string   `json:"sunset"`
	Conditions  string   `json:"conditions"`
	Description string   `json:"description"`
}

// Provider implements the weatherapi.Provider interface for Visual Crossing.
type Provider struct {
	logger zerolog.Logger
	client *http.Client
	apiKey string
}

// New creates a new Visual Crossing provider.
func New(logger zerolog.Logger, apiKey string) *Provider {
	return &Provider{
		logger: logger.With().Str("provider", providerName).Logger(),
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		apiKey: apiKey,
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return providerName
}

// SupportsBackfill returns true.
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
	return p.fetchTimeline(ctx, lat, lon, today, today)
}

// FetchHistoricalWeather fetches weather for a date range.
func (p *Provider) FetchHistoricalWeather(ctx context.Context, lat, lon float64, from, to time.Time) ([]models.WeatherResult, error) {
	return p.fetchTimeline(ctx, lat, lon, from.Format("2006-01-02"), to.Format("2006-01-02"))
}

func (p *Provider) fetchTimeline(ctx context.Context, lat, lon float64, startDate, endDate string) ([]models.WeatherResult, error) {
	requestURL := fmt.Sprintf("%s/%.4f,%.4f/%s/%s?key=%s&include=days&unitGroup=metric&contentType=json",
		baseURL, lat, lon, startDate, endDate, p.apiKey)

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

	if apiResp.QueryCost > 0 {
		p.logger.Debug().Int("queryCost", apiResp.QueryCost).Msg("query cost")
	}

	return p.parseResponse(apiResp, lat, lon, body)
}

func (p *Provider) parseResponse(resp apiResponse, lat, lon float64, rawBody []byte) ([]models.WeatherResult, error) {
	results := make([]models.WeatherResult, 0, len(resp.Days))
	now := time.Now()

	for _, day := range resp.Days {
		date, err := time.Parse("2006-01-02", day.DateTime)
		if err != nil {
			p.logger.Warn().Str("date", day.DateTime).Msg("skipping unparseable date")
			continue
		}

		result := models.WeatherResult{
			Date:               date,
			Provider:           providerName,
			Latitude:           models.RoundCoord(lat),
			Longitude:          models.RoundCoord(lon),
			TemperatureMinC:    day.TempMin,
			TemperatureMaxC:    day.TempMax,
			TemperatureMeanC:   day.Temp,
			PrecipitationMmSum: day.Precip,
			WindSpeedMaxKmh:    day.WindSpeed,
			WindGustMaxKmh:     day.WindGust,
			CloudCoverPercent:  day.CloudCover,
			HumidityPercent:    day.Humidity,
			PressureHpa:        day.Pressure,
			RawResponse:        rawBody,
			FetchedAt:          now,
		}

		// Visual Crossing does not provide sunshine duration directly

		results = append(results, result)
	}

	return results, nil
}
