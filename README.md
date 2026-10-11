# Heizsaison

```
 _   _      _               _                 
| | | | ___(_)_______  __ _(_)___  ___  _ __  
| |_| |/ _ \ |_  / __|/ _` | / __|/ _ \| '_ \ 
|  _  |  __/ |/ /\__ \ (_| | \__ \ (_) | | | |
|_| |_|\___|_/___|___/\__,_|_|___/\___/|_| |_|
```

**Never miss a dip in heating oil prices again.**

*Heizsaison* is German for "heating season" - the months when what you pay to stay warm depends on the weather.

Two Go services that collect daily data and write it into one PostgreSQL database: `heizsaison-oil` pulls heating oil prices from German APIs, and `heizsaison-weather` pulls weather observations for a single location. Heating oil demand tracks the weather, so keeping both in one database makes the interesting queries a join away.

## Features

- **7 providers**: HeizOel24 and Hoyer for oil prices, Open-Meteo, Bright Sky, Visual Crossing, OpenWeather and DWD CDC for weather
- **Daily automated scraping**: built-in scheduler runs at a configurable hour each day
- **Historical backfilling**: import past data from every provider that offers it
- **Duplicate prevention**: records that already exist are skipped, so re-runs are free
- **Observability**: Prometheus `/metrics`, a JSON `/status` endpoint and structured logging
- **Docker ready**: multi-stage builds, published images and a Compose setup for the whole stack

## Quick Start

```bash
git clone https://github.com/andygrunwald/heizsaison.git
cd heizsaison

docker-compose up -d

curl http://localhost:8080/status  # heizsaison-oil
curl http://localhost:8081/status  # heizsaison-weather
```

## Installation

### Using Docker

```bash
docker run -d \
  -e POSTGRES_DSN="postgres://user:password@host:5432/heizsaison?sslmode=disable" \
  -e ZIP_CODE="47259" \
  -p 8080:8080 \
  ghcr.io/andygrunwald/heizsaison-oil:latest

docker run -d \
  -e POSTGRES_DSN="postgres://user:password@host:5432/heizsaison?sslmode=disable" \
  -e LATITUDE="51.4556" \
  -e LONGITUDE="6.7623" \
  -p 8081:8081 \
  ghcr.io/andygrunwald/heizsaison-weather:latest
```

### Building from Source

```bash
go install github.com/andygrunwald/heizsaison/cmd/heizsaison-oil@latest
go install github.com/andygrunwald/heizsaison/cmd/heizsaison-weather@latest
```

## Usage

Both binaries share the same four subcommands:

| Command | Description |
|---------|-------------|
| `run` | Start the continuous scraper with the daily scheduler |
| `scrape` | Run a one-time scrape and exit |
| `backfill` | Import historical data for a date range |
| `version` | Print version information |

```bash
# Collect oil prices daily
heizsaison-oil run \
  --postgres-dsn "postgres://user:password@localhost:5432/heizsaison?sslmode=disable" \
  --zip-code "47259" \
  --providers heizoel24,hoyer

# Collect weather daily
heizsaison-weather run \
  --postgres-dsn "postgres://user:password@localhost:5432/heizsaison?sslmode=disable" \
  --latitude 51.4556 \
  --longitude 6.7623 \
  --providers openmeteo,brightsky

# Import a year of history
heizsaison-oil backfill \
  --postgres-dsn "postgres://user:password@localhost:5432/heizsaison?sslmode=disable" \
  --zip-code "47259" \
  --provider heizoel24 \
  --from 2024-01-01 --to 2024-12-31
```

Every flag and environment variable is documented in [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

## Documentation

| Document | Contents |
|----------|----------|
| [CONFIGURATION.md](docs/CONFIGURATION.md) | Every flag and environment variable for both scrapers |
| [HTTP_ENDPOINTS.md](docs/HTTP_ENDPOINTS.md) | The `/metrics`, `/status` and `/health` endpoints |
| [OIL_PROVIDERS.md](docs/OIL_PROVIDERS.md) | The oil price APIs, what they return and what is stored |
| [WEATHER_PROVIDERS.md](docs/WEATHER_PROVIDERS.md) | The weather APIs, their coverage, keys and limits |
| [OIL_EXAMPLE_QUERIES.md](docs/OIL_EXAMPLE_QUERIES.md) | SQL recipes for price trends, comparisons and buying signals |
| [WEATHER_EXAMPLE_QUERIES.md](docs/WEATHER_EXAMPLE_QUERIES.md) | SQL recipes for temperature, precipitation and heating degree days |
| [DEBUG_LOGGING.md](docs/DEBUG_LOGGING.md) | Turning on debug output and filtering it |
| [DEVELOPMENT.md](docs/DEVELOPMENT.md) | Building, testing and releasing |
