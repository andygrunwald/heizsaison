package dwdcdc

import (
	"math"
	"os"
	"testing"
)

func TestParseStationList(t *testing.T) {
	f, err := os.Open("testdata/stations.txt")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	stations, err := parseStationList(f)
	if err != nil {
		t.Fatalf("parseStationList error: %v", err)
	}

	if len(stations) != 7 {
		t.Fatalf("expected 7 stations, got %d", len(stations))
	}

	// Verify first station
	s := stations[0]
	if s.ID != "00001" {
		t.Errorf("expected ID 00001, got %s", s.ID)
	}
	if s.DateFrom.Format("20060102") != "19370101" {
		t.Errorf("expected DateFrom 19370101, got %s", s.DateFrom.Format("20060102"))
	}
	if s.DateTo.Format("20060102") != "19860630" {
		t.Errorf("expected DateTo 19860630, got %s", s.DateTo.Format("20060102"))
	}
	if math.Abs(s.Elevation-478) > 0.1 {
		t.Errorf("expected elevation 478, got %f", s.Elevation)
	}
	if math.Abs(s.Latitude-47.8413) > 0.001 {
		t.Errorf("expected lat 47.8413, got %f", s.Latitude)
	}
	if math.Abs(s.Longitude-8.8493) > 0.001 {
		t.Errorf("expected lon 8.8493, got %f", s.Longitude)
	}
	if s.Name != "Aach" {
		t.Errorf("expected name 'Aach', got '%s'", s.Name)
	}
	if s.State != "Baden-Württemberg" {
		t.Errorf("expected state 'Baden-Württemberg', got '%s'", s.State)
	}
	if stations[2].Name != "Großenkneten" {
		t.Errorf("expected name 'Großenkneten', got '%s'", stations[2].Name)
	}

	// Verify Heinsberg station
	heinsberg := stations[5]
	if heinsberg.ID != "02110" {
		t.Errorf("expected ID 02110, got %s", heinsberg.ID)
	}
	if heinsberg.Name != "Heinsberg-Schleiden" {
		t.Errorf("expected name 'Heinsberg-Schleiden', got '%s'", heinsberg.Name)
	}
	if heinsberg.State != "Nordrhein-Westfalen" {
		t.Errorf("expected state 'Nordrhein-Westfalen', got '%s'", heinsberg.State)
	}
}

func TestHaversineDistance(t *testing.T) {
	tests := []struct {
		name      string
		lat1      float64
		lon1      float64
		lat2      float64
		lon2      float64
		wantKm    float64
		tolerance float64
	}{
		{
			name: "same point",
			lat1: 51.4556, lon1: 6.7623,
			lat2: 51.4556, lon2: 6.7623,
			wantKm:    0,
			tolerance: 0.01,
		},
		{
			name: "Berlin to Munich",
			lat1: 52.5200, lon1: 13.4050,
			lat2: 48.1351, lon2: 11.5820,
			wantKm:    504,
			tolerance: 10,
		},
		{
			name: "Duisburg to Aachen",
			lat1: 51.4556, lon1: 6.7623,
			lat2: 50.7827, lon2: 6.0941,
			wantKm:    85,
			tolerance: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dist := haversineDistance(tt.lat1, tt.lon1, tt.lat2, tt.lon2)
			if math.Abs(dist-tt.wantKm) > tt.tolerance {
				t.Errorf("haversineDistance = %.1f km, want ~%.0f km (±%.0f)", dist, tt.wantKm, tt.tolerance)
			}
		})
	}
}

func TestFindNearestStation(t *testing.T) {
	f, err := os.Open("testdata/stations.txt")
	if err != nil {
		t.Fatalf("failed to open fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	stations, err := parseStationList(f)
	if err != nil {
		t.Fatalf("parseStationList error: %v", err)
	}

	// Find nearest to Duisburg (51.4556, 6.7623)
	nearest, err := findNearestStation(stations, 51.4556, 6.7623)
	if err != nil {
		t.Fatalf("findNearestStation error: %v", err)
	}

	// Should be Düsseldorf (01078) which is the closest active station
	if nearest.ID != "01078" {
		t.Errorf("expected nearest station 01078 (Düsseldorf), got %s (%s)", nearest.ID, nearest.Name)
	}
}

func TestFindNearestStationEmpty(t *testing.T) {
	_, err := findNearestStation(nil, 51.0, 6.0)
	if err == nil {
		t.Error("expected error for empty station list")
	}
}
