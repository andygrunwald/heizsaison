// Package main provides the entry point for the heizsaison-weather CLI.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/spf13/cobra"

	"github.com/andygrunwald/heizsaison/internal/config"
)

var (
	// Version is set at build time.
	Version = "dev"
	// Commit is set at build time.
	Commit = "none"
	// BuildDate is set at build time.
	BuildDate = "unknown"
)

var cfg *config.Weather

func main() {
	cfg = config.DefaultWeather()
	cfg.LoadFromEnv()

	rootCmd := &cobra.Command{
		Use:   "heizsaison-weather",
		Short: "Heizsaison Weather - Daily weather data collection for your location",
		Long: `heizsaison-weather is a service that scrapes daily weather data from various
APIs and stores them in a PostgreSQL database for analysis and monitoring.

Providers:
  - Open-Meteo (free, no API key)
  - Bright Sky / DWD (free, no API key)
  - Visual Crossing (API key required)
  - OpenWeather One Call 3.0 (API key required)`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfg.PostgresDSN, "postgres-dsn", cfg.PostgresDSN, "PostgreSQL connection string")
	rootCmd.PersistentFlags().StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format (json, console)")
	rootCmd.PersistentFlags().BoolVar(&cfg.StoreRawResponse, "store-raw-response", cfg.StoreRawResponse, "Store raw API responses in database")
	rootCmd.PersistentFlags().StringVar(&cfg.HTTPAddr, "http-addr", cfg.HTTPAddr, "HTTP server address for /metrics, /status")
	rootCmd.PersistentFlags().Float64Var(&cfg.Latitude, "latitude", cfg.Latitude, "Latitude for weather location")
	rootCmd.PersistentFlags().Float64Var(&cfg.Longitude, "longitude", cfg.Longitude, "Longitude for weather location")

	// Add subcommands
	rootCmd.AddCommand(runCmd())
	rootCmd.AddCommand(scrapeCmd())
	rootCmd.AddCommand(backfillCmd())
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func setupLogger() zerolog.Logger {
	var logger zerolog.Logger

	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	if cfg.LogFormat == "console" {
		logger = zerolog.New(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339}).
			With().
			Timestamp().
			Logger()
	} else {
		logger = zerolog.New(os.Stdout).
			With().
			Timestamp().
			Logger()
	}

	return logger
}
