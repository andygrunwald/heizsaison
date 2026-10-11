// Package hoyer provides an API client for the Hoyer oil price service.
package hoyer

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/andygrunwald/heizsaison/internal/models"
	"github.com/andygrunwald/heizsaison/internal/useragent"
	"github.com/rs/zerolog"
)

const (
	// ProviderName is the identifier for this provider.
	ProviderName = "hoyer"
	// baseURL is the API endpoint for Hoyer.
	baseURL = "https://api.hoyer.de/rest/heatingoil"
)

// apiResponse represents the JSON response from Hoyer API.
type apiResponse struct {
	Products []product `json:"products"`
	Settings settings  `json:"settings"`
}

// product represents a single product offering from Hoyer.
type product struct {
	ID               int      `json:"id"`
	Name             string   `json:"name"`
	Description      []string `json:"description"`
	BasePrice        float64  `json:"basePrice"`
	Prices           prices   `json:"prices"`
	IsPremium        bool     `json:"isPremium"`
	IsClimateNeutral bool     `json:"isClimateNeutral"`
	Days             int      `json:"days"`
	DeliveryTimeType string   `json:"deliveryTimeType"`
}

// prices contains pricing details.
type prices struct {
	PriceNet              string  `json:"priceNet"`
	PriceGross            string  `json:"priceGross"`
	Taxes                 string  `json:"taxes"`
	PriceTotalNet         string  `json:"priceTotalNet"`
	PriceTotalGross       string  `json:"priceTotalGross"`
	TaxesTotal            string  `json:"taxesTotal"`
	WithAction            *string `json:"withAction"`
	TotalWithAction       *string `json:"totalWithAction"`
	PriceActionDifference float64 `json:"priceActionDifference"`
}

// settings contains API settings (not currently used but part of response).
type settings struct {
	// We can add specific fields if needed
}

// Provider implements the API provider interface for Hoyer.
type Provider struct {
	client      *http.Client
	logger      zerolog.Logger
	zipCode     string
	orderAmount int
}

// New creates a new Hoyer provider.
func New(logger zerolog.Logger, zipCode string, orderAmount int) *Provider {
	return &Provider{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:      logger.With().Str("provider", ProviderName).Logger(),
		zipCode:     zipCode,
		orderAmount: orderAmount,
	}
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return ProviderName
}

// SupportsBackfill returns false as Hoyer does not support historical data.
func (p *Provider) SupportsBackfill() bool {
	return false
}

// PriceScope returns local as Hoyer provides zip code specific prices.
func (p *Provider) PriceScope() models.PriceScope {
	return models.PriceScopeLocal
}

// FetchCurrentPrices fetches current prices from Hoyer for all available products.
func (p *Provider) FetchCurrentPrices(ctx context.Context) ([]models.PriceResult, error) {
	// Hoyer API: /rest/heatingoil/<PLZ>/<Menge>/<Abladestellen>
	url := fmt.Sprintf("%s/%s/%d/1", baseURL, p.zipCode, p.orderAmount)

	p.logger.Debug().
		Str("url", url).
		Str("zipCode", p.zipCode).
		Int("orderAmount", p.orderAmount).
		Msg("fetching prices from Hoyer")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	// Hoyer requires a browser-like User-Agent
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
	today := time.Now().Truncate(24 * time.Hour)
	results := make([]models.PriceResult, 0, len(apiResp.Products))

	for _, prod := range apiResp.Products {
		// Parse the gross price (with taxes) from German format
		pricePer100L, ok := parseGermanPrice(prod.Prices.PriceGross)
		if !ok {
			p.logger.Warn().
				Str("productName", prod.Name).
				Str("priceGross", prod.Prices.PriceGross).
				Msg("failed to parse priceGross, skipping product")
			continue
		}

		// Normalize product name to lowercase for consistent storage
		productType := normalizeProductType(prod.Name)

		results = append(results, models.PriceResult{
			Date:         today,
			PricePer100L: pricePer100L,
			Currency:     "EUR",
			Provider:     ProviderName,
			ProductType:  productType,
			Scope:        models.PriceScopeLocal,
			ZipCode:      p.zipCode,
			RawResponse:  body,
			FetchedAt:    fetchedAt,
		})
	}

	p.logger.Info().
		Int("productCount", len(results)).
		Str("zipCode", p.zipCode).
		Msg("fetched prices from Hoyer")

	return results, nil
}

// FetchHistoricalPrices returns an error as Hoyer does not support historical data.
func (p *Provider) FetchHistoricalPrices(ctx context.Context, from, to time.Time) ([]models.PriceResult, error) {
	return nil, fmt.Errorf("hoyer does not support historical data")
}

// normalizeProductType converts product names to consistent lowercase identifiers.
func normalizeProductType(name string) string {
	// Convert to lowercase and replace spaces/special chars
	normalized := strings.ToLower(name)
	normalized = strings.ReplaceAll(normalized, " ", "-")
	normalized = strings.ReplaceAll(normalized, "ö", "oe")
	normalized = strings.ReplaceAll(normalized, "ä", "ae")
	normalized = strings.ReplaceAll(normalized, "ü", "ue")
	normalized = strings.ReplaceAll(normalized, "ß", "ss")
	return normalized
}

// parseGermanPrice converts a German-formatted price string (e.g., "90,99") to float64.
// Returns the parsed value and true on success, or 0 and false on failure.
func parseGermanPrice(s string) (float64, bool) {
	// Replace German decimal comma with dot
	normalized := strings.ReplaceAll(s, ",", ".")
	value, err := strconv.ParseFloat(normalized, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
