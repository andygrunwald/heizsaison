package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/andygrunwald/heizsaison/internal/api/heizoel24"
	"github.com/andygrunwald/heizsaison/internal/api/hoyer"
	"github.com/andygrunwald/heizsaison/internal/database"
	"github.com/andygrunwald/heizsaison/internal/scraper"
)

func scrapeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scrape",
		Short: "Run a one-time scrape",
		Long:  "Runs a one-time scrape from the specified providers. Useful for testing.",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := setupLogger()

			if cfg.PostgresDSN == "" {
				return fmt.Errorf("--postgres-dsn is required")
			}

			if cfg.ZipCode == "" {
				return fmt.Errorf("--zip-code is required")
			}

			// Parse providers
			for i := range cfg.Providers {
				cfg.Providers[i] = strings.TrimSpace(cfg.Providers[i])
			}

			logger.Info().
				Strs("providers", cfg.Providers).
				Msg("running one-time scrape")

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
			s := scraper.New(db, cfg.StoreRawResponse, logger)

			// Register providers
			for _, p := range cfg.Providers {
				switch p {
				case "heizoel24":
					s.RegisterProvider(heizoel24.New(logger))
				case "hoyer":
					s.RegisterProvider(hoyer.New(logger, cfg.ZipCode, cfg.OrderAmount))
				default:
					logger.Warn().Str("provider", p).Msg("unknown provider, skipping")
				}
			}

			// Run scrape
			ctx := context.Background()
			if err := s.ScrapeAll(ctx); err != nil {
				return fmt.Errorf("scraping: %w", err)
			}

			logger.Info().Msg("scrape completed")
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&cfg.Providers, "providers", cfg.Providers, "Comma-separated list of providers")

	return cmd
}
