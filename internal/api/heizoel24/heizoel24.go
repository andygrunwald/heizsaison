// Package heizoel24 provides an API client for the HeizOel24 price service.
package heizoel24

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/andygrunwald/heizsaison/internal/models"
	"github.com/andygrunwald/heizsaison/internal/useragent"
	"github.com/rs/zerolog"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "heizoel24"
	// ProductType is the standard product type for HeizOel24.
	ProductType = "standard"
	// baseURL is the API endpoint for HeizOel24.
	baseURL = "https://www.heizoel24.de/api/chartapi/GetAveragePriceHistory"
	// countryID for Germany.
	countryID = 1
)

// apiResponse represents the JSON response from HeizOel24 API.
type apiResponse struct {
	Values        []priceValue `json:"Values"`
	Currency      string       `json:"Currency"`
	ProductName   string       `json:"ProductName"`
	ChartName     string       `json:"ChartName"`
	ChartUnit     string       `json:"ChartUnit"`
	CurrentPrice  float64      `json:"CurrentPrice"`
	ChangePercent float64      `json:"ChangePercent"`
}

// priceValue represents a single price data point.
type priceValue struct {
	Date  int64   `json:"date"`
	Value float64 `json:"value"`
}

// Provider implements the API provider interface for HeizOel24.
type Provider struct {
	client *http.Client
	logger zerolog.Logger
}

// New creates a new HeizOel24 provider.
func New(logger zerolog.Logger) *Provider {
	return &Provider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger.With().Str("provider", ProviderName).Logger(),
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return ProviderName
}

// SupportsBackfill returns true as HeizOel24 supports historical data.
func (p *Provider) SupportsBackfill() bool {
	return true
}

// PriceScope returns national as HeizOel24 provides nationwide average prices.
func (p *Provider) PriceScope() models.PriceScope {
	return models.PriceScopeNational
}

// FetchCurrentPrices fetches today's price from HeizOel24.
func (p *Provider) FetchCurrentPrices(ctx context.Context) ([]models.PriceResult, error) {
	now := time.Now()
	yesterday := now.Add(-24 * time.Hour)
	return p.FetchHistoricalPrices(ctx, yesterday, now)
}

// FetchHistoricalPrices fetches prices for a date range from HeizOel24.
func (p *Provider) FetchHistoricalPrices(ctx context.Context, from, to time.Time) ([]models.PriceResult, error) {
	fromStr := from.Format("2006-01-02")
	toStr := to.Format("2006-01-02")

	apiURL := fmt.Sprintf("%s?countryId=%d&minDate=%s&maxDate=%s", baseURL, countryID, fromStr, toStr)

	p.logger.Debug().
		Str("url", apiURL).
		Str("from", fromStr).
		Str("to", toStr).
		Msg("fetching prices from HeizOel24")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("User-Agent", useragent.Random())
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			panic(err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response body: %w", err)
	}

	var apiResp apiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing response JSON: %w", err)
	}

	fetchedAt := time.Now()
	results := make([]models.PriceResult, 0, len(apiResp.Values))

	for _, v := range apiResp.Values {
		// Convert milliseconds timestamp to time.Time
		priceDate := time.Unix(v.Date/1000, 0).UTC()

		results = append(results, models.PriceResult{
			Date:         priceDate,
			PricePer100L: v.Value,
			Currency:     "EUR",
			Provider:     ProviderName,
			ProductType:  ProductType,
			Scope:        models.PriceScopeNational,
			ZipCode:      "",
			RawResponse:  body,
			FetchedAt:    fetchedAt,
		})
	}

	p.logger.Info().
		Int("count", len(results)).
		Str("from", fromStr).
		Str("to", toStr).
		Msg("fetched prices from HeizOel24")

	return results, nil
}
