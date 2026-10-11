package dwdcdc

import (
	"os"
	"testing"

	"github.com/rs/zerolog"
)

func TestProviderInterface(t *testing.T) {
	logger := zerolog.New(os.Stderr).Level(zerolog.Disabled)
	p := New(logger)

	if p.Name() != "dwdcdc" {
		t.Errorf("expected name 'dwdcdc', got '%s'", p.Name())
	}
	if !p.SupportsBackfill() {
		t.Error("expected SupportsBackfill() = true")
	}
	if p.RequiresAPIKey() {
		t.Error("expected RequiresAPIKey() = false")
	}
}

func TestFindHistoricalFilename(t *testing.T) {
	listing := []byte(`<a href="tageswerte_KL_00044_19690101_20251231_hist.zip">tageswerte_KL_00044_19690101_20251231_hist.zip</a>
<a href="tageswerte_KL_01078_19520101_20251231_hist.zip">tageswerte_KL_01078_19520101_20251231_hist.zip</a>
<a href="tageswerte_KL_01079_19370101_19451231_hist.zip">tageswerte_KL_01079_19370101_19451231_hist.zip</a>`)

	tests := []struct {
		name      string
		stationID string
		want      string
		wantErr   bool
	}{
		{"found", "01078", "tageswerte_KL_01078_19520101_20251231_hist.zip", false},
		{"inactive station", "01079", "tageswerte_KL_01079_19370101_19451231_hist.zip", false},
		{"not found", "99999", "", true},
		{"no prefix match", "0107", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findHistoricalFilename(listing, tt.stationID)
			if (err != nil) != tt.wantErr {
				t.Fatalf("unexpected error state: %v", err)
			}
			if got != tt.want {
				t.Errorf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
