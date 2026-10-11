-- Heizsaison - Oil Price Schema
-- Creates the oil_prices table for PostgreSQL

CREATE TABLE IF NOT EXISTS oil_prices (
    id              BIGSERIAL PRIMARY KEY,
    provider        VARCHAR(50) NOT NULL,
    product_type    VARCHAR(50) NOT NULL DEFAULT 'standard',
    price_date      DATE NOT NULL,
    price_per_100l  DECIMAL(10, 4) NOT NULL,
    currency        VARCHAR(10) NOT NULL DEFAULT 'EUR',
    scope           VARCHAR(10) NOT NULL CHECK (scope IN ('local', 'national')),
    zip_code        VARCHAR(10) DEFAULT NULL,
    raw_response    JSONB DEFAULT NULL,
    fetched_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    -- Unique constraint to prevent duplicate entries
    -- Note: PostgreSQL treats NULLs as distinct in unique constraints by default
    CONSTRAINT unique_provider_product_date UNIQUE NULLS NOT DISTINCT (provider, product_type, price_date, zip_code)
);

-- Indexes for common queries
CREATE INDEX IF NOT EXISTS idx_price_date ON oil_prices (price_date);
CREATE INDEX IF NOT EXISTS idx_provider ON oil_prices (provider);
CREATE INDEX IF NOT EXISTS idx_product_type ON oil_prices (product_type);
CREATE INDEX IF NOT EXISTS idx_scope ON oil_prices (scope);

-- Column comments
COMMENT ON COLUMN oil_prices.provider IS 'Source API (e.g., heizoel24, hoyer)';
COMMENT ON COLUMN oil_prices.product_type IS 'Product variant (e.g., standard, bestpreis, eco)';
COMMENT ON COLUMN oil_prices.price_date IS 'Date the price is valid for';
COMMENT ON COLUMN oil_prices.price_per_100l IS 'Price in EUR per 100 liters';
COMMENT ON COLUMN oil_prices.currency IS 'Currency code';
COMMENT ON COLUMN oil_prices.scope IS 'Geographical scope of the price';
COMMENT ON COLUMN oil_prices.zip_code IS 'Zip code for local prices (NULL for national)';
COMMENT ON COLUMN oil_prices.raw_response IS 'Original JSON response from API';
COMMENT ON COLUMN oil_prices.fetched_at IS 'When the API was called';
COMMENT ON COLUMN oil_prices.created_at IS 'Row creation timestamp';
