# Configuration

Every setting on both scrapers can be given as a command-line flag or an environment variable. This document is the complete reference for `heizsaison-oil` and `heizsaison-weather`.

The two binaries share a common configuration core and add their own settings on top, so most flags below are identical on both. For the logging flags in detail, see [DEBUG_LOGGING.md](DEBUG_LOGGING.md).

---

## Precedence

Settings are resolved in this order, with the last one winning:

1. Built-in default
2. Environment variable
3. Command-line flag, when explicitly passed

---

## Shared Flags

Persistent flags, available on every subcommand of both binaries.

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--postgres-dsn` | `POSTGRES_DSN` | - | PostgreSQL connection string (required) |
| `--log-level` | `LOG_LEVEL` | `info` | Log level (`trace`, `debug`, `info`, `warn`, `error`) |
| `--log-format` | `LOG_FORMAT` | `json` | Log format (`json`, `console`) |
| `--store-raw-response` | `STORE_RAW_RESPONSE` | `false` | Store raw API responses in the database |
| `--http-addr` | `HTTP_ADDR` | `:8080` / `:8081` | HTTP server address. `:8080` for `heizsaison-oil`, `:8081` for `heizsaison-weather` |

## `heizsaison-oil` Flags

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--zip-code` | `ZIP_CODE` | - | Zip code for regional price APIs (required) |
| `--order-amount` | `ORDER_AMOUNT` | `3000` | Order amount in liters |

## `heizsaison-weather` Flags

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--latitude` | `LATITUDE` | `0` | Latitude of the location to observe (required) |
| `--longitude` | `LONGITUDE` | `0` | Longitude of the location to observe (required) |

---

## `run` and `scrape`

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--scrape-hour` | `SCRAPE_HOUR` | `6` / `7` | Hour of day (0-23) to scrape. `run` only. `6` for `heizsaison-oil`, `7` for `heizsaison-weather` |
| `--providers` | `PROVIDERS` | `heizoel24,hoyer` / `openmeteo` | Comma-separated list of providers to enable |

Valid provider names:

| Binary | Providers |
|--------|-----------|
| `heizsaison-oil` | `heizoel24`, `hoyer` |
| `heizsaison-weather` | `openmeteo`, `brightsky`, `visualcrossing`, `openweather`, `dwdcdc` |

See [OIL_PROVIDERS.md](OIL_PROVIDERS.md) and [WEATHER_PROVIDERS.md](WEATHER_PROVIDERS.md) for what each one delivers.

## API Keys (`heizsaison-weather`)

Available on `run`, `scrape` and `backfill`. Only needed for the two providers that require them.

| Flag | Env Variable | Default | Description |
|------|--------------|---------|-------------|
| `--visual-crossing-api-key` | `VISUAL_CROSSING_API_KEY` | - | API key for Visual Crossing |
| `--openweather-api-key` | `OPENWEATHER_API_KEY` | - | API key for OpenWeather One Call 3.0 |

## `backfill`

These five have no environment variable equivalent and must be passed as flags.

| Flag | Default | Description |
|------|---------|-------------|
| `--from` | - | Start date (YYYY-MM-DD, required) |
| `--to` | today | End date (YYYY-MM-DD) |
| `--provider` | `heizoel24` / `openmeteo` | Single provider to backfill from |
| `--min-delay` | `1` | Minimum delay between requests (seconds) |
| `--max-delay` | `5` | Maximum delay between requests (seconds) |

---

## Docker

The published images take the same environment variables:

```bash
docker run -d \
  -e POSTGRES_DSN="postgres://user:password@host:5432/heizsaison?sslmode=disable" \
  -e ZIP_CODE="47259" \
  -p 8080:8080 \
  ghcr.io/andygrunwald/heizsaison-oil:latest
```

`docker-compose.yml` configures both services this way. It is also where the `47259` zip code and the `51.4556` / `6.7623` coordinates come from - those are Compose values, not built-in defaults.

---

## Gotchas

- **`--min-delay` and `--max-delay` only affect OpenWeather.** It is the one provider that backfills day by day and sleeps between requests. Every other provider fetches the whole range in a single call, so both scrapers accept the two values and never use them.
- **`--zip-code` is required even without Hoyer.** `heizsaison-oil` rejects an empty zip code on `run`, `scrape` and `backfill`, whether or not the Hoyer provider is enabled.
- **The coordinate check is an AND.** `heizsaison-weather` only complains when latitude *and* longitude are both `0`, so `--latitude 51.4556` on its own passes validation and silently observes longitude `0`.
- **`STORE_RAW_RESPONSE` is matched case-insensitively against `true`.** Anything else, `1` included, means false.
- **Unparseable numeric environment variables are ignored silently.** A bad `SCRAPE_HOUR`, `LATITUDE`, `LONGITUDE` or `ORDER_AMOUNT` leaves the default in place without a warning. `SCRAPE_HOUR` is additionally ignored outside 0-23.
- **`--store-raw-response` only controls the database column.** The `/status` endpoint reports `last_raw_response` either way - see [HTTP_ENDPOINTS.md](HTTP_ENDPOINTS.md).
