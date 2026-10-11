package brightsky

import (
	"math"
	"os"
	"testing"

	"github.com/rs/zerolog"

	"github.com/andygrunwald/heizsaison/internal/numeric"
)

func TestAggregateHourlyToDaily(t *testing.T) {
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	p := New(logger)

	// Simulate 3 hourly records for one day
	records := []weatherRecord{
		{
			Timestamp:        "2024-03-22T00:00:00+01:00",
			Temperature:      numeric.Float64Ptr(5.0),
			Precipitation:    numeric.Float64Ptr(0.0),
			WindSpeed:        numeric.Float64Ptr(10.0),
			WindGustSpeed:    numeric.Float64Ptr(15.0),
			Sunshine:         numeric.Float64Ptr(0.0),
			CloudCover:       numeric.Float64Ptr(80.0),
			RelativeHumidity: numeric.Float64Ptr(90.0),
			PressureMSL:      numeric.Float64Ptr(1013.0),
		},
		{
			Timestamp:        "2024-03-22T12:00:00+01:00",
			Temperature:      numeric.Float64Ptr(15.0),
			Precipitation:    numeric.Float64Ptr(2.0),
			WindSpeed:        numeric.Float64Ptr(20.0),
			WindGustSpeed:    numeric.Float64Ptr(30.0),
			Sunshine:         numeric.Float64Ptr(30.0),
			CloudCover:       numeric.Float64Ptr(40.0),
			RelativeHumidity: numeric.Float64Ptr(60.0),
			PressureMSL:      numeric.Float64Ptr(1015.0),
		},
		{
			Timestamp:        "2024-03-22T18:00:00+01:00",
			Temperature:      numeric.Float64Ptr(10.0),
			Precipitation:    numeric.Float64Ptr(1.5),
			WindSpeed:        numeric.Float64Ptr(15.0),
			WindGustSpeed:    numeric.Float64Ptr(25.0),
			Sunshine:         numeric.Float64Ptr(20.0),
			CloudCover:       numeric.Float64Ptr(60.0),
			RelativeHumidity: numeric.Float64Ptr(75.0),
			PressureMSL:      numeric.Float64Ptr(1014.0),
		},
	}

	results, err := p.aggregateHourlyToDaily(records, 51.4556, 6.7623, []byte("{}"))
	if err != nil {
		t.Fatalf("aggregateHourlyToDaily error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 daily result, got %d", len(results))
	}

	r := results[0]

	if r.Date.Format("2006-01-02") != "2024-03-22" {
		t.Errorf("expected date 2024-03-22, got %s", r.Date.Format("2006-01-02"))
	}
	if r.Provider != "brightsky" {
		t.Errorf("expected provider brightsky, got %s", r.Provider)
	}

	// Temperature: min=5, max=15, mean=10
	assertFloat64Ptr(t, "TemperatureMinC", r.TemperatureMinC, 5.0)
	assertFloat64Ptr(t, "TemperatureMaxC", r.TemperatureMaxC, 15.0)
	assertFloat64Ptr(t, "TemperatureMeanC", r.TemperatureMeanC, 10.0)

	// Precipitation: sum = 0 + 2 + 1.5 = 3.5
	assertFloat64Ptr(t, "PrecipitationMmSum", r.PrecipitationMmSum, 3.5)

	// Wind: max = 20
	assertFloat64Ptr(t, "WindSpeedMaxKmh", r.WindSpeedMaxKmh, 20.0)

	// Gusts: max = 30
	assertFloat64Ptr(t, "WindGustMaxKmh", r.WindGustMaxKmh, 30.0)

	// Sunshine: sum = (0 + 30 + 20) minutes * 60 = 3000 seconds
	assertFloat64Ptr(t, "SunshineDurationS", r.SunshineDurationS, 3000.0)

	// Cloud cover: mean = (80 + 40 + 60) / 3 = 60
	assertFloat64Ptr(t, "CloudCoverPercent", r.CloudCoverPercent, 60.0)

	// Humidity: mean = (90 + 60 + 75) / 3 = 75
	assertFloat64Ptr(t, "HumidityPercent", r.HumidityPercent, 75.0)

	// Pressure: mean = (1013 + 1015 + 1014) / 3 = 1014
	assertFloat64Ptr(t, "PressureHpa", r.PressureHpa, 1014.0)
}

func TestAggregateMultipleDays(t *testing.T) {
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	p := New(logger)

	records := []weatherRecord{
		{Timestamp: "2024-03-22T12:00:00+01:00", Temperature: numeric.Float64Ptr(10.0)},
		{Timestamp: "2024-03-23T12:00:00+01:00", Temperature: numeric.Float64Ptr(15.0)},
	}

	results, err := p.aggregateHourlyToDaily(records, 51.4556, 6.7623, []byte("{}"))
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("expected 2 daily results, got %d", len(results))
	}

	if results[0].Date.Format("2006-01-02") != "2024-03-22" {
		t.Errorf("expected first date 2024-03-22, got %s", results[0].Date.Format("2006-01-02"))
	}
	if results[1].Date.Format("2006-01-02") != "2024-03-23" {
		t.Errorf("expected second date 2024-03-23, got %s", results[1].Date.Format("2006-01-02"))
	}
}

func TestAggregateWithNilValues(t *testing.T) {
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	p := New(logger)

	records := []weatherRecord{
		{Timestamp: "2024-03-22T12:00:00+01:00", Temperature: numeric.Float64Ptr(10.0), Precipitation: nil},
	}

	results, err := p.aggregateHourlyToDaily(records, 51.4556, 6.7623, []byte("{}"))
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if results[0].PrecipitationMmSum != nil {
		t.Errorf("expected PrecipitationMmSum to be nil when no data")
	}
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
