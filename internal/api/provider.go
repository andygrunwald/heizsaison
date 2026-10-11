// Package api provides the interfaces and types for oil price and weather API
// providers.
//
// Provider and WeatherProvider both embed commonProvider, which is everything a
// provider must offer regardless of what it fetches. Everything below that is
// genuinely domain-specific: the two fetch pairs return unrelated result types,
// and the fifth method has no counterpart on the other side.
package api

import (
	"context"
	"time"

	"github.com/andygrunwald/heizsaison/internal/models"
)

// commonProvider is the part of the provider contract that does not depend on
// what is being fetched.
type commonProvider interface {
	// Name returns the provider identifier.
	Name() string

	// SupportsBackfill returns true if the provider supports historical data.
	SupportsBackfill() bool
}

// Provider defines the interface for oil price API providers.
type Provider interface {
	commonProvider

	// FetchCurrentPrices fetches today's prices (may return multiple for different product types).
	FetchCurrentPrices(ctx context.Context) ([]models.PriceResult, error)

	// FetchHistoricalPrices fetches prices for a date range (if supported).
	FetchHistoricalPrices(ctx context.Context, from, to time.Time) ([]models.PriceResult, error)

	// PriceScope returns whether the price is local (zip code) or nationwide.
	PriceScope() models.PriceScope
}

// WeatherProvider defines the interface for weather API providers.
type WeatherProvider interface {
	commonProvider

	// FetchCurrentWeather fetches today's weather observation for the given location.
	FetchCurrentWeather(ctx context.Context, lat, lon float64) ([]models.WeatherResult, error)

	// FetchHistoricalWeather fetches weather observations for a date range (if supported).
	FetchHistoricalWeather(ctx context.Context, lat, lon float64, from, to time.Time) ([]models.WeatherResult, error)

	// RequiresAPIKey returns true if the provider requires an API key.
	RequiresAPIKey() bool
}
