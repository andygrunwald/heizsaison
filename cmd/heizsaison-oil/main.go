// Package main provides the entry point for the heizsaison-oil CLI.
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

var cfg *config.Oil

func main() {
	cfg = config.DefaultOil()
	cfg.LoadFromEnv()

	rootCmd := &cobra.Command{
		Use:   "heizsaison-oil",
		Short: "Heizsaison Oil - Never miss a dip in heating oil prices again",
		Long: `heizsaison-oil is a service that scrapes heating oil prices from various
German APIs and stores them in a PostgreSQL database for analysis and monitoring.

Features:
  - Multiple API providers (HeizOel24, Hoyer)
  - Daily automated scraping with configurable schedule
  - Historical data backfilling
  - Prometheus metrics endpoint
  - Status endpoint for operational visibility`,
	}

	// Global flags
	rootCmd.PersistentFlags().StringVar(&cfg.PostgresDSN, "postgres-dsn", cfg.PostgresDSN, "PostgreSQL connection string")
	rootCmd.PersistentFlags().StringVar(&cfg.LogLevel, "log-level", cfg.LogLevel, "Log level (debug, info, warn, error)")
	rootCmd.PersistentFlags().StringVar(&cfg.LogFormat, "log-format", cfg.LogFormat, "Log format (json, console)")
	rootCmd.PersistentFlags().BoolVar(&cfg.StoreRawResponse, "store-raw-response", cfg.StoreRawResponse, "Store raw API responses in database")
	rootCmd.PersistentFlags().StringVar(&cfg.HTTPAddr, "http-addr", cfg.HTTPAddr, "HTTP server address for /metrics, /status")
	rootCmd.PersistentFlags().StringVar(&cfg.ZipCode, "zip-code", cfg.ZipCode, "Zip code for local price APIs")
	rootCmd.PersistentFlags().IntVar(&cfg.OrderAmount, "order-amount", cfg.OrderAmount, "Order amount in liters")

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

	// Set log level
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Set log format
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
