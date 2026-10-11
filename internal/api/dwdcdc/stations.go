// Package dwdcdc provides a weather provider using DWD CDC-OpenData (German Weather Service).
package dwdcdc

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/andygrunwald/heizsaison/internal/useragent"
)

const (
	stationListURL = "https://opendata.dwd.de/climate_environment/CDC/observations_germany/climate/daily/kl/recent/KL_Tageswerte_Beschreibung_Stationen.txt"
)

// Station represents a DWD weather station.
type Station struct {
	ID        string
	Name      string
	Latitude  float64
	Longitude float64
	Elevation float64
	DateFrom  time.Time
	DateTo    time.Time
	State     string
}

// fetchStationList downloads and parses the DWD station list.
func fetchStationList(ctx context.Context, client *http.Client) ([]Station, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, stationListURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating station list request: %w", err)
	}
	req.Header.Set("User-Agent", useragent.Random())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching station list: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("station list returned status %d", resp.StatusCode)
	}

	return parseStationList(resp.Body)
}

// parseStationList parses the fixed-width DWD station list file.
// The file has a 2-line header (column names + dashes separator), then data lines.
// Format is fixed-width with fields at known character positions.
func parseStationList(r io.Reader) ([]Station, error) {
	scanner := bufio.NewScanner(r)
	// Increase buffer size for potentially long lines
	scanner.Buffer(make([]byte, 0, 4096), 4096)

	lineNum := 0
	var stations []Station

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Skip header lines (first 2 lines)
		if lineNum <= 2 {
			continue
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		station, err := parseStationLine(line)
		if err != nil {
			continue // skip unparseable lines
		}

		stations = append(stations, station)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading station list: %w", err)
	}

	return stations, nil
}

// parseStationLine parses a single fixed-width station line.
// Format based on actual DWD file:
//
//	Stations_id von_datum bis_datum Stationshoehe geoBreite geoLaenge Stationsname                             Bundesland
//	----------- --------- --------- ------------- --------- --------- ----------------------------------------- ----------
//	00001 19370101 19860630            478     47.8413    8.8493 Aach                                     Baden-Württemberg
func parseStationLine(line string) (Station, error) {
	// The file uses fixed-width columns. We parse by splitting on whitespace
	// for the numeric fields, then handle the rest.
	// Approach: extract fields by their known positions based on the actual file format.
	// The numeric fields are space-separated at the start, station name and state are at the end.

	fields := strings.Fields(line)
	if len(fields) < 7 {
		return Station{}, fmt.Errorf("too few fields: %d", len(fields))
	}

	id := fields[0]

	dateFrom, err := time.Parse("20060102", fields[1])
	if err != nil {
		return Station{}, fmt.Errorf("parsing von_datum: %w", err)
	}

	dateTo, err := time.Parse("20060102", fields[2])
	if err != nil {
		return Station{}, fmt.Errorf("parsing bis_datum: %w", err)
	}

	elevation, err := strconv.ParseFloat(fields[3], 64)
	if err != nil {
		return Station{}, fmt.Errorf("parsing elevation: %w", err)
	}

	lat, err := strconv.ParseFloat(fields[4], 64)
	if err != nil {
		return Station{}, fmt.Errorf("parsing latitude: %w", err)
	}

	lon, err := strconv.ParseFloat(fields[5], 64)
	if err != nil {
		return Station{}, fmt.Errorf("parsing longitude: %w", err)
	}

	// The remaining fields are station name and state.
	// Station name may contain spaces (e.g., "Donaueschingen (Landeplatz)").
	// The state is at the very end. We use the known column positions from the file.
	// Find station name and state from the original line using fixed positions.
	// Based on real data, the station name starts around column 61 and the state
	// follows after the station name field.
	//
	// However, since the column positions can vary slightly, a more robust approach:
	// After the 6 numeric fields, join remaining fields, and split the last known
	// German state name from the end.
	//
	// Simplest reliable approach: rejoin remaining fields, then try to extract state.
	remaining := fields[6:]
	name, state := extractNameAndState(remaining)

	return Station{
		ID:        id,
		Name:      name,
		Latitude:  lat,
		Longitude: lon,
		Elevation: elevation,
		DateFrom:  dateFrom,
		DateTo:    dateTo,
		State:     state,
	}, nil
}

// germanStates lists all German federal states for extraction from station lines.
var germanStates = []string{
	"Baden-Württemberg",
	"Bayern",
	"Berlin",
	"Brandenburg",
	"Bremen",
	"Hamburg",
	"Hessen",
	"Mecklenburg-Vorpommern",
	"Niedersachsen",
	"Nordrhein-Westfalen",
	"Rheinland-Pfalz",
	"Saarland",
	"Sachsen",
	"Sachsen-Anhalt",
	"Schleswig-Holstein",
	"Thüringen",
}

// extractNameAndState splits remaining fields into station name and state.
// The state is always one of the known German federal states at the end.
func extractNameAndState(fields []string) (string, string) {
	joined := strings.Join(fields, " ")

	// Try to find a known state at the end (after trimming trailing "Frei" and spaces)
	joined = strings.TrimSpace(joined)
	joined = strings.TrimSuffix(joined, "Frei")
	joined = strings.TrimSpace(joined)

	for _, state := range germanStates {
		if before, found := strings.CutSuffix(joined, state); found {
			return strings.TrimSpace(before), state
		}
	}

	// Fallback: no state found, entire remainder is the name
	return joined, ""
}

// findNearestStation returns the station closest to the given coordinates.
// Only considers stations with data up to at least the current year.
func findNearestStation(stations []Station, lat, lon float64) (Station, error) {
	if len(stations) == 0 {
		return Station{}, fmt.Errorf("no stations available")
	}

	var nearest Station
	minDist := math.MaxFloat64
	now := time.Now()
	cutoff := now.AddDate(-1, 0, 0) // station must have data within the last year

	for _, s := range stations {
		// Skip stations that haven't reported recently
		if s.DateTo.Before(cutoff) {
			continue
		}

		dist := haversineDistance(lat, lon, s.Latitude, s.Longitude)
		if dist < minDist {
			minDist = dist
			nearest = s
		}
	}

	if nearest.ID == "" {
		return Station{}, fmt.Errorf("no active station found near %.4f, %.4f", lat, lon)
	}

	return nearest, nil
}

// haversineDistance calculates the great-circle distance in km between two points.
func haversineDistance(lat1, lon1, lat2, lon2 float64) float64 {
	const earthRadiusKm = 6371.0

	dLat := degreesToRadians(lat2 - lat1)
	dLon := degreesToRadians(lon2 - lon1)

	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(degreesToRadians(lat1))*math.Cos(degreesToRadians(lat2))*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return earthRadiusKm * c
}

func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180
}
