package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/andygrunwald/heizsaison/internal/api/heizoel24"
	"github.com/andygrunwald/heizsaison/internal/api/hoyer"
	"github.com/andygrunwald/heizsaison/internal/database"
	"github.com/andygrunwald/heizsaison/internal/http"
	"github.com/andygrunwald/heizsaison/internal/scheduler"
	"github.com/andygrunwald/heizsaison/internal/scraper"
)

func runCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "run",
		Short: "Start the continuous scraper service",
		Long:  "Starts the oil price scraper with an internal scheduler that runs daily at the specified hour.",
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
				Str("version", Version).
				Str("commit", Commit).
				Str("buildDate", BuildDate).
				Str("httpAddr", cfg.HTTPAddr).
				Int("scrapeHour", cfg.ScrapeHour).
				Strs("providers", cfg.Providers).
				Msg("starting heizsaison-oil")

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

			// Create scheduler
			sched := scheduler.New(s, cfg.ScrapeHour, logger)

			// Create HTTP server
			httpServer := http.NewServer(cfg.HTTPAddr, s, sched, db, logger)

			// Wire Prometheus metrics to scraper
			s.SetPrometheusMetrics(httpServer.Metrics())

			// Setup signal handling
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

			// Start HTTP server in goroutine
			go func() {
				if err := httpServer.Start(); err != nil {
					logger.Error().Err(err).Msg("HTTP server error")
					cancel()
				}
			}()

			// Start scheduler in goroutine
			go func() {
				if err := sched.Start(ctx); err != nil && err != context.Canceled {
					logger.Error().Err(err).Msg("scheduler error")
					cancel()
				}
			}()

			// Wait for signal
			select {
			case sig := <-sigCh:
				logger.Info().Str("signal", sig.String()).Msg("received signal, shutting down")
			case <-ctx.Done():
			}

			// Graceful shutdown
			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer shutdownCancel()

			if err := httpServer.Shutdown(shutdownCtx); err != nil {
				logger.Error().Err(err).Msg("HTTP server shutdown error")
			}

			logger.Info().Msg("shutdown complete")
			return nil
		},
	}

	cmd.Flags().IntVar(&cfg.ScrapeHour, "scrape-hour", cfg.ScrapeHour, "Hour of day (0-23) to scrape")
	cmd.Flags().StringSliceVar(&cfg.Providers, "providers", cfg.Providers, "Comma-separated list of providers")

	return cmd
}
