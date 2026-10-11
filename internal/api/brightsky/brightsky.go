// Package brightsky provides a weather provider using the Bright Sky API (DWD data).
package brightsky

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
	"github.com/andygrunwald/heizsaison/internal/numeric"
	"github.com/andygrunwald/heizsaison/internal/useragent"
)

const (
	providerName = "brightsky"
	baseURL      = "https://api.brightsky.dev/weather"
)

// apiResponse represents the Bright Sky API response.
type apiResponse struct {
	Weather []weatherRecord `json:"weather"`
	Sources []source        `json:"sources"`
}

type weatherRecord struct {
	Timestamp        string   `json:"timestamp"`
	Temperature      *float64 `json:"temperature"`
	Precipitation    *float64 `json:"precipitation"`
	WindSpeed        *float64 `json:"wind_speed"`
	WindGustSpeed    *float64 `json:"wind_gust_speed"`
	WindDirection    *float64 `json:"wind_direction"`
	Sunshine         *float64 `json:"sunshine"`
	CloudCover       *float64 `json:"cloud_cover"`
	PressureMSL      *float64 `json:"pressure_msl"`
	RelativeHumidity *float64 `json:"relative_humidity"`
	DewPoint         *float64 `json:"dew_point"`
	Visibility       *float64 `json:"visibility"`
	Condition        *string  `json:"condition"`
	SourceID         int      `json:"source_id"`
}

type source struct {
	ID           int     `json:"id"`
	DWDStationID string  `json:"dwd_station_id"`
	StationName  string  `json:"station_name"`
	Latitude     float64 `json:"lat"`
	Longitude    float64 `json:"lon"`
	Height       float64 `json:"height"`
}

// Provider implements the weatherapi.Provider interface for Bright Sky.
type Provider struct {
	logger zerolog.Logger
	client *http.Client
}

// New creates a new Bright Sky provider.
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

// SupportsBackfill returns true — Bright Sky has data from 2010+.
func (p *Provider) SupportsBackfill() bool {
	return true
}

// RequiresAPIKey returns false — Bright Sky is free and requires no API key.
func (p *Provider) RequiresAPIKey() bool {
	return false
}

// FetchCurrentWeather fetches today's weather for the given location.
func (p *Provider) FetchCurrentWeather(ctx context.Context, lat, lon float64) ([]models.WeatherResult, error) {
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	return p.fetchAndAggregate(ctx, lat, lon, today, tomorrow)
}

// FetchHistoricalWeather fetches weather for a date range and aggregates hourly to daily.
func (p *Provider) FetchHistoricalWeather(ctx context.Context, lat, lon float64, from, to time.Time) ([]models.WeatherResult, error) {
	// Bright Sky can handle multi-day ranges but for large ranges we chunk to avoid timeouts
	var allResults []models.WeatherResult

	chunkSize := 30 // days per request
	current := from
	for current.Before(to) || current.Equal(to) {
		end := current.AddDate(0, 0, chunkSize)
		if end.After(to) {
			end = to.AddDate(0, 0, 1) // last_date is exclusive
		} else {
			end = end.AddDate(0, 0, 1) // last_date is exclusive
		}

		results, err := p.fetchAndAggregate(ctx, lat, lon, current.Format("2006-01-02"), end.Format("2006-01-02"))
		if err != nil {
			return nil, fmt.Errorf("fetching chunk %s to %s: %w", current.Format("2006-01-02"), end.Format("2006-01-02"), err)
		}

		allResults = append(allResults, results...)
		current = current.AddDate(0, 0, chunkSize)
	}

	return allResults, nil
}

func (p *Provider) fetchAndAggregate(ctx context.Context, lat, lon float64, date, lastDate string) ([]models.WeatherResult, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing base URL: %w", err)
	}

	q := u.Query()
	q.Set("lat", fmt.Sprintf("%.4f", lat))
	q.Set("lon", fmt.Sprintf("%.4f", lon))
	q.Set("date", date)
	q.Set("last_date", lastDate)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
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

	return p.aggregateHourlyToDaily(apiResp.Weather, lat, lon, body)
}

// aggregateHourlyToDaily groups hourly records by date and computes daily aggregates.
func (p *Provider) aggregateHourlyToDaily(records []weatherRecord, lat, lon float64, rawBody []byte) ([]models.WeatherResult, error) {
	// Group records by date
	type dayBucket struct {
		temps    []float64
		precips  []float64
		winds    []float64
		gusts    []float64
		sunshine []float64
		clouds   []float64
		humidity []float64
		pressure []float64
	}

	buckets := make(map[string]*dayBucket)
	var dateOrder []string

	for _, rec := range records {
		t, err := time.Parse(time.RFC3339, rec.Timestamp)
		if err != nil {
			p.logger.Warn().Str("timestamp", rec.Timestamp).Msg("skipping unparseable timestamp")
			continue
		}
		dateKey := t.Format("2006-01-02")

		bucket, exists := buckets[dateKey]
		if !exists {
			bucket = &dayBucket{}
			buckets[dateKey] = bucket
			dateOrder = append(dateOrder, dateKey)
		}

		if rec.Temperature != nil {
			bucket.temps = append(bucket.temps, *rec.Temperature)
		}
		if rec.Precipitation != nil {
			bucket.precips = append(bucket.precips, *rec.Precipitation)
		}
		if rec.WindSpeed != nil {
			bucket.winds = append(bucket.winds, *rec.WindSpeed)
		}
		if rec.WindGustSpeed != nil {
			bucket.gusts = append(bucket.gusts, *rec.WindGustSpeed)
		}
		if rec.Sunshine != nil {
			bucket.sunshine = append(bucket.sunshine, *rec.Sunshine)
		}
		if rec.CloudCover != nil {
			bucket.clouds = append(bucket.clouds, *rec.CloudCover)
		}
		if rec.RelativeHumidity != nil {
			bucket.humidity = append(bucket.humidity, *rec.RelativeHumidity)
		}
		if rec.PressureMSL != nil {
			bucket.pressure = append(bucket.pressure, *rec.PressureMSL)
		}
	}

	now := time.Now()
	results := make([]models.WeatherResult, 0, len(dateOrder))

	for _, dateKey := range dateOrder {
		bucket := buckets[dateKey]
		date, _ := time.Parse("2006-01-02", dateKey)

		result := models.WeatherResult{
			Date:        date,
			Provider:    providerName,
			Latitude:    models.RoundCoord(lat),
			Longitude:   models.RoundCoord(lon),
			RawResponse: rawBody,
			FetchedAt:   now,
		}

		if len(bucket.temps) > 0 {
			minT := numeric.SliceMin(bucket.temps)
			maxT := numeric.SliceMax(bucket.temps)
			meanT := numeric.SliceMean(bucket.temps)
			result.TemperatureMinC = &minT
			result.TemperatureMaxC = &maxT
			result.TemperatureMeanC = &meanT
		}
		if len(bucket.precips) > 0 {
			sum := numeric.SliceSum(bucket.precips)
			result.PrecipitationMmSum = &sum
		}
		if len(bucket.winds) > 0 {
			maxW := numeric.SliceMax(bucket.winds)
			result.WindSpeedMaxKmh = &maxW
		}
		if len(bucket.gusts) > 0 {
			maxG := numeric.SliceMax(bucket.gusts)
			result.WindGustMaxKmh = &maxG
		}
		if len(bucket.sunshine) > 0 {
			// Bright Sky returns sunshine in minutes per hour; sum and convert to seconds
			sumMin := numeric.SliceSum(bucket.sunshine)
			sumSec := sumMin * 60
			result.SunshineDurationS = &sumSec
		}
		if len(bucket.clouds) > 0 {
			mean := numeric.SliceMean(bucket.clouds)
			result.CloudCoverPercent = &mean
		}
		if len(bucket.humidity) > 0 {
			mean := numeric.SliceMean(bucket.humidity)
			result.HumidityPercent = &mean
		}
		if len(bucket.pressure) > 0 {
			mean := numeric.SliceMean(bucket.pressure)
			result.PressureHpa = &mean
		}

		results = append(results, result)
	}

	return results, nil
}
