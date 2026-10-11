package http

import (
	"slices"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestMetricNames pins the exact metric names and labels both scrapers expose.
//
// The shared metrics are built from a namespace plus a bare name, so a typo in
// either half silently renames a metric and breaks every dashboard and alert
// built on it. This test is also the only place both metric sets are created in
// one process: promauto registers into the default registry and panics on a
// duplicate, so it proves the two sets can coexist after the merge.
//
// NewMetrics and NewWeatherMetrics must be called exactly once in this test
// binary, which is why both live in a single test function.
func TestMetricNames(t *testing.T) {
	// A *Vec with no children reports nothing, so record one sample per metric.
	m := NewMetrics()
	m.RecordAPIRequest("heizoel24", "success", 0.1)
	m.RecordLastScrape("heizoel24", 1)
	m.RecordDBOperation("insert", "success")
	m.RecordCurrentPrice("heizoel24", "national", "standard", 99.5)
	m.RecordPricesStored("heizoel24", 1)

	w := NewWeatherMetrics()
	w.RecordAPIRequest("openmeteo", "success", 0.1)
	w.RecordLastScrape("openmeteo", 1)
	w.RecordDBOperation("insert", "success")
	w.RecordCurrentTemperature("openmeteo", 21.5)
	w.RecordObservationsStored("openmeteo", 1)

	families, err := prometheus.DefaultGatherer.Gather()
	if err != nil {
		t.Fatalf("gathering metrics: %v", err)
	}

	labels := make(map[string][]string, len(families))
	for _, f := range families {
		if len(f.GetMetric()) == 0 {
			continue
		}
		names := make([]string, 0, len(f.GetMetric()[0].GetLabel()))
		for _, l := range f.GetMetric()[0].GetLabel() {
			names = append(names, l.GetName())
		}
		slices.Sort(names)
		labels[f.GetName()] = names
	}

	want := map[string][]string{
		"heizsaison_oil_api_requests_total":               {"provider", "status"},
		"heizsaison_oil_api_request_duration_seconds":     {"provider"},
		"heizsaison_oil_last_scrape_timestamp":            {"provider"},
		"heizsaison_oil_db_operations_total":              {"operation", "status"},
		"heizsaison_oil_current_price_eur":                {"product_type", "provider", "scope"},
		"heizsaison_oil_prices_stored_total":              {"provider"},
		"heizsaison_weather_api_requests_total":           {"provider", "status"},
		"heizsaison_weather_api_request_duration_seconds": {"provider"},
		"heizsaison_weather_last_scrape_timestamp":        {"provider"},
		"heizsaison_weather_db_operations_total":          {"operation", "status"},
		"heizsaison_weather_current_temperature_celsius":  {"provider"},
		"heizsaison_weather_observations_stored_total":    {"provider"},
	}

	for name, wantLabels := range want {
		gotLabels, ok := labels[name]
		if !ok {
			t.Errorf("metric %q is not registered", name)
			continue
		}
		if !slices.Equal(gotLabels, wantLabels) {
			t.Errorf("metric %q has labels %v, want %v", name, gotLabels, wantLabels)
		}
	}
}
