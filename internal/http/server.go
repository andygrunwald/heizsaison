// Package http provides HTTP server functionality for the oil and weather
// scrapers.
//
// baseServer, commonMetrics and the small helpers in status.go hold everything
// the two scrapers share. Server and WeatherServer, in oil.go and weather.go,
// add the parts that are genuinely domain-specific: the /status payload and the
// price- or temperature-shaped metrics.
package http

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
)

// baseServer holds the parts of the HTTP server that are identical for both
// scrapers. Server and WeatherServer embed it, which promotes Start and
// Shutdown onto both.
type baseServer struct {
	server *http.Server
	logger zerolog.Logger
}

// newBaseServer wires the /metrics, /health and /status routes into an
// http.Server. status is the caller's domain-specific /status handler.
func newBaseServer(addr string, status http.Handler, logger zerolog.Logger) baseServer {
	mux := http.NewServeMux()

	// Register handlers
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/status", status)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("OK")); err != nil {
			panic(err)
		}
	})

	return baseServer{
		server: &http.Server{
			Addr:         addr,
			Handler:      mux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
		logger: logger.With().Str("component", "http").Logger(),
	}
}

// Start starts the HTTP server.
func (s *baseServer) Start() error {
	s.logger.Info().Str("addr", s.server.Addr).Msg("starting HTTP server")
	if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Shutdown gracefully shuts down the HTTP server.
func (s *baseServer) Shutdown(ctx context.Context) error {
	s.logger.Info().Msg("shutting down HTTP server")
	return s.server.Shutdown(ctx)
}
