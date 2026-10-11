package dwdcdc

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/andygrunwald/heizsaison/internal/models"
	"github.com/andygrunwald/heizsaison/internal/useragent"
)

// dailyRecord holds one parsed row from the DWD daily KL data file.
type dailyRecord struct {
	StationID string
	Date      time.Time
	FX        *float64 // max wind gust (m/s)
	FM        *float64 // mean wind speed (m/s)
	RSK       *float64 // precipitation (mm)
	SDK       *float64 // sunshine duration (hours)
	NM        *float64 // cloud cover (okta, 1/8)
	PM        *float64 // mean pressure (hPa)
	TMK       *float64 // mean temperature (°C)
	UPM       *float64 // relative humidity (%)
	TXK       *float64 // max temperature (°C)
	TNK       *float64 // min temperature (°C)
}

// downloadAndExtractZIP downloads a ZIP file and extracts the produkt_klima_tag_*.txt data file.
func downloadAndExtractZIP(ctx context.Context, client *http.Client, zipURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, zipURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("User-Agent", useragent.Random())

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading ZIP: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ZIP download returned status %d for %s", resp.StatusCode, zipURL)
	}

	zipData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading ZIP body: %w", err)
	}

	return extractDataFile(zipData)
}

// extractDataFile opens a ZIP archive in memory and returns the contents of the
// produkt_klima_tag_*.txt data file.
func extractDataFile(zipData []byte) ([]byte, error) {
	reader, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		return nil, fmt.Errorf("opening ZIP: %w", err)
	}

	for _, f := range reader.File {
		if strings.HasPrefix(f.Name, "produkt_klima_tag") && strings.HasSuffix(f.Name, ".txt") {
			rc, err := f.Open()
			if err != nil {
				return nil, fmt.Errorf("opening %s in ZIP: %w", f.Name, err)
			}
			defer func() { _ = rc.Close() }()

			data, err := io.ReadAll(rc)
			if err != nil {
				return nil, fmt.Errorf("reading %s: %w", f.Name, err)
			}
			return data, nil
		}
	}

	return nil, fmt.Errorf("no produkt_klima_tag_*.txt file found in ZIP")
}

// parseDataFile parses the semicolon-delimited DWD daily KL data file.
// The file has a header row followed by data rows.
// Format: STATIONS_ID;MESS_DATUM;QN_3;FX;FM;QN_4;RSK;RSKF;SDK;SHK_TAG;NM;VPM;PM;TMK;UPM;TXK;TNK;TGK;eor
func parseDataFile(data []byte) ([]dailyRecord, error) {
	// The file may be latin-1 encoded; handle by treating as bytes (ASCII-compatible fields)
	lines := strings.Split(string(data), "\n")

	var records []dailyRecord
	headerSkipped := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Skip header line
		if !headerSkipped {
			if strings.Contains(line, "STATIONS_ID") {
				headerSkipped = true
				continue
			}
			// If we haven't found the header yet, skip
			continue
		}

		fields := strings.Split(line, ";")
		if len(fields) < 19 {
			continue
		}

		// Trim spaces from all fields
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}

		date, err := time.Parse("20060102", fields[1])
		if err != nil {
			continue
		}

		rec := dailyRecord{
			StationID: fields[0],
			Date:      date,
			FX:        parseDWDFloat(fields[3]),  // max wind gust
			FM:        parseDWDFloat(fields[4]),  // mean wind speed
			RSK:       parseDWDFloat(fields[6]),  // precipitation
			SDK:       parseDWDFloat(fields[8]),  // sunshine
			NM:        parseDWDFloat(fields[10]), // cloud cover
			PM:        parseDWDFloat(fields[12]), // pressure
			TMK:       parseDWDFloat(fields[13]), // mean temp
			UPM:       parseDWDFloat(fields[14]), // humidity
			TXK:       parseDWDFloat(fields[15]), // max temp
			TNK:       parseDWDFloat(fields[16]), // min temp
		}

		records = append(records, rec)
	}

	return records, nil
}

// parseDWDFloat parses a DWD float value, returning nil for missing values (-999).
func parseDWDFloat(s string) *float64 {
	s = strings.TrimSpace(s)
	if s == "" || s == "-999" || s == "-999.0" || s == "-999.00" {
		return nil
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}

	// DWD uses -999 variants for missing data
	if v <= -999 {
		return nil
	}

	return &v
}

// filterByDateRange filters records to include only those within [from, to] inclusive.
func filterByDateRange(records []dailyRecord, from, to time.Time) []dailyRecord {
	fromDate := from.Truncate(24 * time.Hour)
	toDate := to.Truncate(24 * time.Hour)

	var filtered []dailyRecord
	for _, r := range records {
		d := r.Date.Truncate(24 * time.Hour)
		if (d.Equal(fromDate) || d.After(fromDate)) && (d.Equal(toDate) || d.Before(toDate)) {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// recordToWeatherResult converts a DWD daily record to a WeatherResult with unit conversions.
func recordToWeatherResult(rec dailyRecord, lat, lon float64, rawBody []byte) models.WeatherResult {
	result := models.WeatherResult{
		Date:        rec.Date,
		Provider:    providerName,
		Latitude:    models.RoundCoord(lat),
		Longitude:   models.RoundCoord(lon),
		RawResponse: rawBody,
		FetchedAt:   time.Now(),
	}

	result.TemperatureMinC = rec.TNK
	result.TemperatureMaxC = rec.TXK
	result.TemperatureMeanC = rec.TMK
	result.PrecipitationMmSum = rec.RSK
	result.HumidityPercent = rec.UPM
	result.PressureHpa = rec.PM

	// Wind: m/s → km/h (×3.6)
	if rec.FM != nil {
		v := *rec.FM * 3.6
		result.WindSpeedMaxKmh = &v
	}
	if rec.FX != nil {
		v := *rec.FX * 3.6
		result.WindGustMaxKmh = &v
	}

	// Sunshine: hours → seconds (×3600)
	if rec.SDK != nil {
		v := *rec.SDK * 3600
		result.SunshineDurationS = &v
	}

	// Cloud cover: okta (1/8) → percent (×12.5)
	if rec.NM != nil {
		v := *rec.NM * 12.5
		result.CloudCoverPercent = &v
	}

	return result
}
