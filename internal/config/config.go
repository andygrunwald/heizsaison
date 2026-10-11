// Package config provides configuration structures and loading for the oil
// and weather scrapers.
//
// Common holds everything both scrapers need. Oil and Weather embed it and add
// their own settings, so a shared setting is declared and parsed exactly once.
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Common holds the configuration shared by both scrapers.
type Common struct {
	// PostgreSQL connection string
	PostgresDSN string
	// Log level (debug, info, warn, error)
	LogLevel string
	// Log format (json, console)
	LogFormat string
	// Store raw API responses in database
	StoreRawResponse bool
	// HTTP server address
	HTTPAddr string
	// Scrape hour (0-23)
	ScrapeHour int
	// Enabled providers
	Providers []string
	// Backfill settings
	Backfill BackfillConfig
}

// BackfillConfig holds configuration for backfilling historical data.
type BackfillConfig struct {
	// Start date for backfill
	From time.Time
	// End date for backfill
	To time.Time
	// Provider to backfill from
	Provider string
	// Minimum delay between requests in seconds
	MinDelay int
	// Maximum delay between requests in seconds
	MaxDelay int
}

// LoadFromEnv loads the shared configuration from environment variables.
// Values that fail to parse are ignored, leaving the existing value in place.
func (c *Common) LoadFromEnv() {
	if v := os.Getenv("POSTGRES_DSN"); v != "" {
		c.PostgresDSN = v
	}
	if v := os.Getenv("LOG_LEVEL"); v != "" {
		c.LogLevel = v
	}
	if v := os.Getenv("LOG_FORMAT"); v != "" {
		c.LogFormat = v
	}
	if v := os.Getenv("STORE_RAW_RESPONSE"); v != "" {
		c.StoreRawResponse = strings.ToLower(v) == "true"
	}
	if v := os.Getenv("HTTP_ADDR"); v != "" {
		c.HTTPAddr = v
	}
	if v := os.Getenv("SCRAPE_HOUR"); v != "" {
		if i, err := strconv.Atoi(v); err == nil && i >= 0 && i <= 23 {
			c.ScrapeHour = i
		}
	}
	if v := os.Getenv("PROVIDERS"); v != "" {
		c.Providers = strings.Split(v, ",")
	}
}
