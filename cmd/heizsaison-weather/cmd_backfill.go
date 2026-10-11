package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/andygrunwald/heizsaison/internal/api/brightsky"
	"github.com/andygrunwald/heizsaison/internal/api/dwdcdc"
	"github.com/andygrunwald/heizsaison/internal/api/openmeteo"
	"github.com/andygrunwald/heizsaison/internal/api/openweather"
	"github.com/andygrunwald/heizsaison/internal/api/visualcrossing"
	"github.com/andygrunwald/heizsaison/internal/database"
	"github.com/andygrunwald/heizsaison/internal/scraper"
)

func backfillCmd() *cobra.Command {
	var fromStr, toStr string
	var visualCrossingAPIKey string
	var openWeatherAPIKey string

	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Backfill historical weather data",
		Long:  "Backfills historical weather data from providers that support it.",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := setupLogger()

			if cfg.PostgresDSN == "" {
				return fmt.Errorf("--postgres-dsn is required")
			}

			if cfg.Latitude == 0 && cfg.Longitude == 0 {
				return fmt.Errorf("--latitude and --longitude are required")
			}

			if fromStr == "" {
				return fmt.Errorf("--from is required")
			}

			// Override config from flags
			if visualCrossingAPIKey != "" {
				cfg.VisualCrossingAPIKey = visualCrossingAPIKey
			}
			if openWeatherAPIKey != "" {
				cfg.OpenWeatherAPIKey = openWeatherAPIKey
			}

			var err error
			cfg.Backfill.From, err = time.Parse("2006-01-02", fromStr)
			if err != nil {
				return fmt.Errorf("parsing --from date: %w", err)
			}

			cfg.Backfill.To = time.Now()
			if toStr != "" {
				cfg.Backfill.To, err = time.Parse("2006-01-02", toStr)
				if err != nil {
					return fmt.Errorf("parsing --to date: %w", err)
				}
			}

			logger.Info().
				Str("provider", cfg.Backfill.Provider).
				Str("from", cfg.Backfill.From.Format("2006-01-02")).
				Str("to", cfg.Backfill.To.Format("2006-01-02")).
				Int("minDelay", cfg.Backfill.MinDelay).
				Int("maxDelay", cfg.Backfill.MaxDelay).
				Float64("latitude", cfg.Latitude).
				Float64("longitude", cfg.Longitude).
				Msg("starting weather backfill")

			// Connect to database
			db, err := database.New(cfg.PostgresDSN, logger)
			if err != nil {
				return fmt.Errorf("connecting to database: %w", err)
			}
			defer func() {
				if err := db.Close(); err != nil {
					panic(err)
				}
			}()

			// Create scraper
			s := scraper.NewWeather(db, cfg.StoreRawResponse, cfg.Latitude, cfg.Longitude, logger)

			// Register provider
			switch cfg.Backfill.Provider {
			case "openmeteo":
				s.RegisterProvider(openmeteo.New(logger))
			case "brightsky":
				s.RegisterProvider(brightsky.New(logger))
			case "visualcrossing":
				if cfg.VisualCrossingAPIKey == "" {
					return fmt.Errorf("--visual-crossing-api-key is required for Visual Crossing provider")
				}
				s.RegisterProvider(visualcrossing.New(logger, cfg.VisualCrossingAPIKey))
			case "openweather":
				if cfg.OpenWeatherAPIKey == "" {
					return fmt.Errorf("--openweather-api-key is required for OpenWeather provider")
				}
				s.RegisterProvider(openweather.New(logger, cfg.OpenWeatherAPIKey, cfg.Backfill.MinDelay, cfg.Backfill.MaxDelay))
			case "dwdcdc":
				s.RegisterProvider(dwdcdc.New(logger))
			default:
				return fmt.Errorf("unknown provider: %s", cfg.Backfill.Provider)
			}

			// Run backfill
			ctx := context.Background()
			if err := s.Backfill(ctx, cfg.Backfill.Provider, cfg.Backfill.From, cfg.Backfill.To, cfg.Backfill.MinDelay, cfg.Backfill.MaxDelay); err != nil {
				return fmt.Errorf("backfilling: %w", err)
			}

			logger.Info().Msg("backfill completed")
			return nil
		},
	}

	cmd.Flags().StringVar(&fromStr, "from", "", "Start date (YYYY-MM-DD, required)")
	cmd.Flags().StringVar(&toStr, "to", "", "End date (YYYY-MM-DD, defaults to today)")
	cmd.Flags().StringVar(&cfg.Backfill.Provider, "provider", cfg.Backfill.Provider, "Provider to backfill from")
	cmd.Flags().IntVar(&cfg.Backfill.MinDelay, "min-delay", cfg.Backfill.MinDelay, "Minimum delay between requests (seconds)")
	cmd.Flags().IntVar(&cfg.Backfill.MaxDelay, "max-delay", cfg.Backfill.MaxDelay, "Maximum delay between requests (seconds)")
	cmd.Flags().StringVar(&visualCrossingAPIKey, "visual-crossing-api-key", "", "Visual Crossing API key")
	cmd.Flags().StringVar(&openWeatherAPIKey, "openweather-api-key", "", "OpenWeather API key")

	return cmd
}
