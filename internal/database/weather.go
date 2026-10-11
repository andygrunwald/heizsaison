package database

import (
	"context"
	"fmt"
	"time"

	"github.com/andygrunwald/heizsaison/internal/models"
)

// InsertWeatherObservation inserts a new weather observation record into the database.
func (d *DB) InsertWeatherObservation(ctx context.Context, obs models.WeatherResult, storeRawResponse bool) error {
	query := `
		INSERT INTO weather_observations (
			provider, observation_date, latitude, longitude,
			temperature_min_c, temperature_max_c, temperature_mean_c,
			precipitation_mm_sum, wind_speed_max_kmh, wind_gust_max_kmh,
			sunshine_duration_s, cloud_cover_percent, humidity_percent, pressure_hpa,
			raw_response, fetched_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		ON CONFLICT (provider, observation_date, latitude, longitude)
		DO UPDATE SET
			temperature_min_c = EXCLUDED.temperature_min_c,
			temperature_max_c = EXCLUDED.temperature_max_c,
			temperature_mean_c = EXCLUDED.temperature_mean_c,
			precipitation_mm_sum = EXCLUDED.precipitation_mm_sum,
			wind_speed_max_kmh = EXCLUDED.wind_speed_max_kmh,
			wind_gust_max_kmh = EXCLUDED.wind_gust_max_kmh,
			sunshine_duration_s = EXCLUDED.sunshine_duration_s,
			cloud_cover_percent = EXCLUDED.cloud_cover_percent,
			humidity_percent = EXCLUDED.humidity_percent,
			pressure_hpa = EXCLUDED.pressure_hpa,
			raw_response = EXCLUDED.raw_response,
			fetched_at = EXCLUDED.fetched_at
	`

	var rawResponse []byte
	if storeRawResponse {
		rawResponse = obs.RawResponse
	}

	lat := models.RoundCoord(obs.Latitude)
	lon := models.RoundCoord(obs.Longitude)

	_, err := d.db.ExecContext(ctx, query,
		obs.Provider,
		obs.Date.Format("2006-01-02"),
		lat,
		lon,
		obs.TemperatureMinC,
		obs.TemperatureMaxC,
		obs.TemperatureMeanC,
		obs.PrecipitationMmSum,
		obs.WindSpeedMaxKmh,
		obs.WindGustMaxKmh,
		obs.SunshineDurationS,
		obs.CloudCoverPercent,
		obs.HumidityPercent,
		obs.PressureHpa,
		rawResponse,
		obs.FetchedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting weather observation: %w", err)
	}

	d.logger.Debug().
		Str("provider", obs.Provider).
		Str("date", obs.Date.Format("2006-01-02")).
		Float64("latitude", lat).
		Float64("longitude", lon).
		Msg("inserted weather observation record")

	return nil
}

// WeatherExistsForDate checks if a weather observation exists for the given provider, date, and location.
func (d *DB) WeatherExistsForDate(ctx context.Context, provider string, date time.Time, lat, lon float64) (bool, error) {
	query := `
		SELECT COUNT(*) FROM weather_observations
		WHERE provider = $1 AND observation_date = $2 AND latitude = $3 AND longitude = $4
	`

	roundedLat := models.RoundCoord(lat)
	roundedLon := models.RoundCoord(lon)

	var count int
	err := d.db.QueryRowContext(ctx, query,
		provider,
		date.Format("2006-01-02"),
		roundedLat,
		roundedLon,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking weather existence: %w", err)
	}

	return count > 0, nil
}

// GetTotalWeatherCount returns the total number of weather observation records in the database.
func (d *DB) GetTotalWeatherCount(ctx context.Context) (int64, error) {
	var count int64
	err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM weather_observations").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting weather observations: %w", err)
	}
	return count, nil
}
