package database

import (
	"context"
	"fmt"
	"time"

	"github.com/andygrunwald/heizsaison/internal/models"
)

// InsertPrice inserts a new oil price record into the database.
func (d *DB) InsertPrice(ctx context.Context, price models.PriceResult, storeRawResponse bool) error {
	query := `
		INSERT INTO oil_prices (provider, product_type, price_date, price_per_100l, currency, scope, zip_code, raw_response, fetched_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		ON CONFLICT (provider, product_type, price_date, zip_code)
		DO UPDATE SET
			price_per_100l = EXCLUDED.price_per_100l,
			raw_response = EXCLUDED.raw_response,
			fetched_at = EXCLUDED.fetched_at
	`

	var rawResponse []byte
	if storeRawResponse {
		rawResponse = price.RawResponse
	}

	var zipCode *string
	if price.ZipCode != "" {
		zipCode = &price.ZipCode
	}

	_, err := d.db.ExecContext(ctx, query,
		price.Provider,
		price.ProductType,
		price.Date.Format("2006-01-02"),
		price.PricePer100L,
		price.Currency,
		string(price.Scope),
		zipCode,
		rawResponse,
		price.FetchedAt,
	)
	if err != nil {
		return fmt.Errorf("inserting price: %w", err)
	}

	d.logger.Debug().
		Str("provider", price.Provider).
		Str("product_type", price.ProductType).
		Str("date", price.Date.Format("2006-01-02")).
		Float64("price", price.PricePer100L).
		Msg("inserted price record")

	return nil
}

// ExistsForDate checks if a price record exists for the given provider, product type, date, and zip code.
func (d *DB) ExistsForDate(ctx context.Context, provider, productType string, date time.Time, zipCode string) (bool, error) {
	query := `
		SELECT COUNT(*) FROM oil_prices
		WHERE provider = $1 AND product_type = $2 AND price_date = $3
		AND (zip_code = $4 OR (zip_code IS NULL AND $4 IS NULL))
	`

	var zipCodePtr *string
	if zipCode != "" {
		zipCodePtr = &zipCode
	}

	var count int
	err := d.db.QueryRowContext(ctx, query,
		provider,
		productType,
		date.Format("2006-01-02"),
		zipCodePtr,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("checking existence: %w", err)
	}

	return count > 0, nil
}

// GetTotalPricesCount returns the total number of price records in the database.
func (d *DB) GetTotalPricesCount(ctx context.Context) (int64, error) {
	var count int64
	err := d.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM oil_prices").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("counting prices: %w", err)
	}
	return count, nil
}
