package main

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/andygrunwald/heizsaison/internal/api/heizoel24"
	"github.com/andygrunwald/heizsaison/internal/api/hoyer"
	"github.com/andygrunwald/heizsaison/internal/database"
	"github.com/andygrunwald/heizsaison/internal/scraper"
)

func backfillCmd() *cobra.Command {
	var fromStr, toStr string

	cmd := &cobra.Command{
		Use:   "backfill",
		Short: "Backfill historical data",
		Long:  "Backfills historical data from APIs that support it (e.g., HeizOel24).",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := setupLogger()

			if cfg.PostgresDSN == "" {
				return fmt.Errorf("--postgres-dsn is required")
			}

			if cfg.ZipCode == "" {
				return fmt.Errorf("--zip-code is required")
			}

			if fromStr == "" {
				return fmt.Errorf("--from is required")
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
				Msg("starting backfill")

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

			// Register provider
			switch cfg.Backfill.Provider {
			case "heizoel24":
				s.RegisterProvider(heizoel24.New(logger))
			case "hoyer":
				s.RegisterProvider(hoyer.New(logger, cfg.ZipCode, cfg.OrderAmount))
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

	return cmd
}
