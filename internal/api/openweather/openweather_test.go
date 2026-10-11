package openweather

import (
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/rs/zerolog"

	"github.com/andygrunwald/heizsaison/internal/numeric"
)

func TestMapResponse(t *testing.T) {
	responseJSON := `{
		"lat": 51.4556,
		"lon": 6.7623,
		"tz": "+01:00",
		"date": "2024-03-22",
		"units": "metric",
		"cloud_cover": {"afternoon": 75.0},
		"humidity": {"afternoon": 65.0},
		"precipitation": {"total": 2.4},
		"temperature": {
			"min": 8.1,
			"max": 15.2,
			"morning": 10.5,
			"afternoon": 14.8,
			"evening": 12.3,
			"night": 9.1
		},
		"pressure": {"afternoon": 1013.0},
		"wind": {"max": {"speed": 7.5, "direction": 240}}
	}`

	var resp daySummaryResponse
	if err := json.Unmarshal([]byte(responseJSON), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	p := New(logger, "test-key", 1, 5)

	result := p.mapResponse(resp, 51.4556, 6.7623)

	if result.Date.Format("2006-01-02") != "2024-03-22" {
		t.Errorf("expected date 2024-03-22, got %s", result.Date.Format("2006-01-02"))
	}
	if result.Provider != "openweather" {
		t.Errorf("expected provider openweather, got %s", result.Provider)
	}

	assertFloat64Ptr(t, "TemperatureMinC", result.TemperatureMinC, 8.1)
	assertFloat64Ptr(t, "TemperatureMaxC", result.TemperatureMaxC, 15.2)
	assertFloat64Ptr(t, "TemperatureMeanC", result.TemperatureMeanC, 11.65) // (8.1 + 15.2) / 2
	assertFloat64Ptr(t, "PrecipitationMmSum", result.PrecipitationMmSum, 2.4)

	// Wind speed: 7.5 m/s * 3.6 = 27 km/h
	assertFloat64Ptr(t, "WindSpeedMaxKmh", result.WindSpeedMaxKmh, 27.0)

	assertFloat64Ptr(t, "CloudCoverPercent", result.CloudCoverPercent, 75.0)
	assertFloat64Ptr(t, "HumidityPercent", result.HumidityPercent, 65.0)
	assertFloat64Ptr(t, "PressureHpa", result.PressureHpa, 1013.0)
}

func TestMapResponseWithNulls(t *testing.T) {
	responseJSON := `{
		"lat": 51.4556,
		"lon": 6.7623,
		"tz": "+01:00",
		"date": "2024-03-22",
		"units": "metric",
		"cloud_cover": {"afternoon": null},
		"humidity": {"afternoon": null},
		"precipitation": {"total": null},
		"temperature": {"min": 8.1, "max": 15.2},
		"pressure": {"afternoon": null},
		"wind": {"max": {"speed": null, "direction": null}}
	}`

	var resp daySummaryResponse
	if err := json.Unmarshal([]byte(responseJSON), &resp); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	p := New(logger, "test-key", 1, 5)

	result := p.mapResponse(resp, 51.4556, 6.7623)

	assertFloat64Ptr(t, "TemperatureMinC", result.TemperatureMinC, 8.1)
	assertFloat64Ptr(t, "TemperatureMaxC", result.TemperatureMaxC, 15.2)

	if result.PrecipitationMmSum != nil {
		t.Errorf("expected PrecipitationMmSum to be nil")
	}
	if result.WindSpeedMaxKmh != nil {
		t.Errorf("expected WindSpeedMaxKmh to be nil")
	}
	if result.CloudCoverPercent != nil {
		t.Errorf("expected CloudCoverPercent to be nil")
	}
}

func TestWindSpeedConversion(t *testing.T) {
	// 10 m/s = 36 km/h
	resp := daySummaryResponse{
		Date: "2024-03-22",
		Temperature: temperature{
			Min: numeric.Float64Ptr(5.0),
			Max: numeric.Float64Ptr(10.0),
		},
		Wind: wind{Max: windMax{Speed: numeric.Float64Ptr(10.0)}},
	}

	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	p := New(logger, "test-key", 1, 5)

	result := p.mapResponse(resp, 51.4556, 6.7623)
	assertFloat64Ptr(t, "WindSpeedMaxKmh", result.WindSpeedMaxKmh, 36.0)
}

func assertFloat64Ptr(t *testing.T, name string, got *float64, want float64) {
	t.Helper()
	if got == nil {
		t.Errorf("%s: expected %f, got nil", name, want)
		return
	}
	if math.Abs(*got-want) > 0.01 {
		t.Errorf("%s: expected %f, got %f", name, want, *got)
	}
}
