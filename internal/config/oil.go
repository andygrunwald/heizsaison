package config

import (
	"os"
	"strconv"
)

// Oil holds all configuration for the oil scraper.
type Oil struct {
	Common
	// Zip code for local price APIs
	ZipCode string
	// Order amount in liters
	OrderAmount int
}

// DefaultOil returns an Oil configuration with default values.
func DefaultOil() *Oil {
	return &Oil{
		Common: Common{
			PostgresDSN:      "",
			LogLevel:         "info",
			LogFormat:        "json",
			StoreRawResponse: false,
			HTTPAddr:         ":8080",
			ScrapeHour:       6,
			Providers:        []string{"heizoel24", "hoyer"},
			Backfill: BackfillConfig{
				Provider: "heizoel24",
				MinDelay: 1,
				MaxDelay: 5,
			},
		},
		ZipCode:     "",
		OrderAmount: 3000,
	}
}

// LoadFromEnv loads the oil scraper configuration from environment variables.
func (c *Oil) LoadFromEnv() {
	c.Common.LoadFromEnv()

	if v := os.Getenv("ZIP_CODE"); v != "" {
		c.ZipCode = v
	}
	if v := os.Getenv("ORDER_AMOUNT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.OrderAmount = i
		}
	}
}
