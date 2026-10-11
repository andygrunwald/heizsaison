package dwdcdc

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/andygrunwald/heizsaison/internal/numeric"
)

func TestParseDataFile(t *testing.T) {
	data, err := os.ReadFile("testdata/produkt_klima_tag_sample.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	records, err := parseDataFile(data)
	if err != nil {
		t.Fatalf("parseDataFile error: %v", err)
	}

	if len(records) != 5 {
		t.Fatalf("expected 5 records, got %d", len(records))
	}

	// Verify first record (2024-01-01)
	r := records[0]
	if r.StationID != "2110" {
		t.Errorf("expected station 2110, got %s", r.StationID)
	}
	if r.Date.Format("2006-01-02") != "2024-01-01" {
		t.Errorf("expected date 2024-01-01, got %s", r.Date.Format("2006-01-02"))
	}
	assertFloat64Ptr(t, "FX", r.FX, 13.9)
	assertFloat64Ptr(t, "FM", r.FM, 6.6)
	assertFloat64Ptr(t, "RSK", r.RSK, 2.5)
	assertFloat64Ptr(t, "SDK", r.SDK, 3.5)
	assertFloat64Ptr(t, "TMK", r.TMK, 5.2)
	assertFloat64Ptr(t, "TXK", r.TXK, 8.1)
	assertFloat64Ptr(t, "TNK", r.TNK, 2.3)
	assertFloat64Ptr(t, "UPM", r.UPM, 85.0)
	assertFloat64Ptr(t, "PM", r.PM, 1013.4)
}

func TestMissingValueHandling(t *testing.T) {
	data, err := os.ReadFile("testdata/produkt_klima_tag_sample.txt")
	if err != nil {
		t.Fatalf("failed to read fixture: %v", err)
	}

	records, err := parseDataFile(data)
	if err != nil {
		t.Fatalf("parseDataFile error: %v", err)
	}

	// Second record (2024-01-02) has FX = -999
	r := records[1]
	if r.FX != nil {
		t.Errorf("expected FX to be nil (was -999), got %f", *r.FX)
	}
	// FM should still be valid
	assertFloat64Ptr(t, "FM", r.FM, 4.2)

	// Third record (2024-01-03) has RSK = -999
	r3 := records[2]
	if r3.RSK != nil {
		t.Errorf("expected RSK to be nil (was -999), got %f", *r3.RSK)
	}

	// Fourth record (2024-01-04) has SDK = -999
	r4 := records[3]
	if r4.SDK != nil {
		t.Errorf("expected SDK to be nil (was -999), got %f", *r4.SDK)
	}
}

func TestParseDWDFloat(t *testing.T) {
	tests := []struct {
		input string
		want  *float64
	}{
		{"13.9", numeric.Float64Ptr(13.9)},
		{"-999", nil},
		{"-999.0", nil},
		{"-999.00", nil},
		{"", nil},
		{"   ", nil},
		{"0.0", numeric.Float64Ptr(0.0)},
		{"-2.5", numeric.Float64Ptr(-2.5)},
		{"  7.8  ", numeric.Float64Ptr(7.8)},
	}

	for _, tt := range tests {
		got := parseDWDFloat(tt.input)
		if tt.want == nil {
			if got != nil {
				t.Errorf("parseDWDFloat(%q) = %f, want nil", tt.input, *got)
			}
		} else {
			if got == nil {
				t.Errorf("parseDWDFloat(%q) = nil, want %f", tt.input, *tt.want)
			} else if math.Abs(*got-*tt.want) > 0.01 {
				t.Errorf("parseDWDFloat(%q) = %f, want %f", tt.input, *got, *tt.want)
			}
		}
	}
}

func TestUnitConversions(t *testing.T) {
	rec := dailyRecord{
		StationID: "02110",
		Date:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		FM:        numeric.Float64Ptr(10.0), // 10 m/s
		FX:        numeric.Float64Ptr(20.0), // 20 m/s
		SDK:       numeric.Float64Ptr(5.0),  // 5 hours
		NM:        numeric.Float64Ptr(4.0),  // 4 okta
		TMK:       numeric.Float64Ptr(5.0),
		TXK:       numeric.Float64Ptr(10.0),
		TNK:       numeric.Float64Ptr(0.0),
	}

	result := recordToWeatherResult(rec, 51.4556, 6.7623, nil)

	// Wind: 10 m/s → 36 km/h
	assertFloat64Ptr(t, "WindSpeedMaxKmh", result.WindSpeedMaxKmh, 36.0)

	// Gust: 20 m/s → 72 km/h
	assertFloat64Ptr(t, "WindGustMaxKmh", result.WindGustMaxKmh, 72.0)

	// Sunshine: 5 hours → 18000 seconds
	assertFloat64Ptr(t, "SunshineDurationS", result.SunshineDurationS, 18000.0)

	// Cloud cover: 4 okta → 50%
	assertFloat64Ptr(t, "CloudCoverPercent", result.CloudCoverPercent, 50.0)

	// Direct mappings (no conversion)
	assertFloat64Ptr(t, "TemperatureMeanC", result.TemperatureMeanC, 5.0)
	assertFloat64Ptr(t, "TemperatureMaxC", result.TemperatureMaxC, 10.0)
	assertFloat64Ptr(t, "TemperatureMinC", result.TemperatureMinC, 0.0)
}

func TestUnitConversionsNil(t *testing.T) {
	rec := dailyRecord{
		StationID: "02110",
		Date:      time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		// All fields nil
	}

	result := recordToWeatherResult(rec, 51.4556, 6.7623, nil)

	if result.WindSpeedMaxKmh != nil {
		t.Error("expected WindSpeedMaxKmh to be nil")
	}
	if result.WindGustMaxKmh != nil {
		t.Error("expected WindGustMaxKmh to be nil")
	}
	if result.SunshineDurationS != nil {
		t.Error("expected SunshineDurationS to be nil")
	}
	if result.CloudCoverPercent != nil {
		t.Error("expected CloudCoverPercent to be nil")
	}
}

func TestFilterByDateRange(t *testing.T) {
	records := []dailyRecord{
		{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Date: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)},
		{Date: time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)},
		{Date: time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC)},
		{Date: time.Date(2024, 1, 5, 0, 0, 0, 0, time.UTC)},
	}

	// Filter to Jan 2-4 inclusive
	from := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 1, 4, 0, 0, 0, 0, time.UTC)

	filtered := filterByDateRange(records, from, to)
	if len(filtered) != 3 {
		t.Fatalf("expected 3 records, got %d", len(filtered))
	}
	if filtered[0].Date.Day() != 2 {
		t.Errorf("expected first date day 2, got %d", filtered[0].Date.Day())
	}
	if filtered[2].Date.Day() != 4 {
		t.Errorf("expected last date day 4, got %d", filtered[2].Date.Day())
	}
}

func TestFilterByDateRangeEmpty(t *testing.T) {
	records := []dailyRecord{
		{Date: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)},
	}

	// Filter to a range that doesn't include any records
	from := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)

	filtered := filterByDateRange(records, from, to)
	if len(filtered) != 0 {
		t.Errorf("expected 0 records, got %d", len(filtered))
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
