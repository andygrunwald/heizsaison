# HTTP Endpoints

Both scrapers start an HTTP server alongside the `run` command and serve the same three routes: `/metrics`, `/status` and `/health`. Only the `/status` payload and the metric name prefix differ between them.

The address is set with `--http-addr` / `HTTP_ADDR` and defaults to `:8080` for `heizsaison-oil` and `:8081` for `heizsaison-weather`. See [CONFIGURATION.md](CONFIGURATION.md) for the full flag reference.

---

## `/metrics` - Prometheus Metrics

Prometheus exposition format, served by `promhttp`. Every metric is prefixed with the scraper's namespace: `heizsaison_oil_` or `heizsaison_weather_`.

### Shared Metrics

Both scrapers export these four with identical types, labels and help text:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `<ns>_api_requests_total` | Counter | `provider`, `status` | Total number of API requests by provider and status |
| `<ns>_api_request_duration_seconds` | Histogram | `provider` | API request duration in seconds (default buckets) |
| `<ns>_last_scrape_timestamp` | Gauge | `provider` | Unix timestamp of the last successful scrape |
| `<ns>_db_operations_total` | Counter | `operation`, `status` | Total number of database operations by type and status |

### Scraper-Specific Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `heizsaison_oil_current_price_eur` | Gauge | `provider`, `scope`, `product_type` | Current oil price in EUR per 100L |
| `heizsaison_oil_prices_stored_total` | Gauge | `provider` | Total number of prices stored in the database by provider |
| `heizsaison_weather_current_temperature_celsius` | Gauge | `provider` | Current temperature in Celsius |
| `heizsaison_weather_observations_stored_total` | Gauge | `provider` | Total number of observations stored in the database by provider |

### Example

```
heizsaison_oil_api_requests_total{provider="heizoel24",status="success"} 365
heizsaison_oil_api_request_duration_seconds_bucket{provider="heizoel24",le="0.25"} 340
heizsaison_oil_last_scrape_timestamp{provider="heizoel24"} 1.7683968e+09
heizsaison_oil_db_operations_total{operation="insert",status="success"} 1234
heizsaison_oil_current_price_eur{provider="heizoel24",scope="national",product_type="standard"} 97.81
heizsaison_oil_prices_stored_total{provider="heizoel24"} 1234
```

The standard Go runtime and process collectors (`go_goroutines`, `go_memstats_*`, `process_*`) are registered by default and exported alongside these.

---

## `/status` - Status Endpoint

Returns `200 OK` with a JSON snapshot of the scheduler, every registered provider and the database. Provider statistics are in-memory counters, so they reset when the process restarts.

### `heizsaison-oil`

```json
{
  "status": "healthy",
  "uptime_seconds": 86400,
  "scheduler_running": true,
  "next_scrape_at": "2026-01-13T06:00:00Z",
  "last_scheduled_scrape_at": "2026-01-12T06:00:00Z",
  "providers": {
    "heizoel24": {
      "enabled": true,
      "last_scrape_at": "2026-01-12T06:00:00Z",
      "last_scrape_success": true,
      "last_response_time_ms": 245,
      "last_price": 97.81,
      "last_error": null,
      "total_requests": 365,
      "total_errors": 2
    }
  },
  "database": {
    "connected": true,
    "total_prices_stored": 1234
  }
}
```

### `heizsaison-weather`

```json
{
  "status": "healthy",
  "uptime_seconds": 86400,
  "scheduler_running": true,
  "next_scrape_at": "2026-01-13T07:00:00Z",
  "last_scheduled_scrape_at": "2026-01-12T07:00:00Z",
  "providers": {
    "openmeteo": {
      "enabled": true,
      "last_scrape_at": "2026-01-12T07:00:00Z",
      "last_scrape_success": true,
      "last_response_time_ms": 132,
      "last_temperature": 4.7,
      "last_error": null,
      "total_requests": 365,
      "total_errors": 0
    }
  },
  "database": {
    "connected": true,
    "total_observations_stored": 1234
  }
}
```

### Fields

| Field | Description |
|-------|-------------|
| `status` | Always `healthy` while the server responds |
| `uptime_seconds` | Seconds since the HTTP server started |
| `scheduler_running` | Whether the daily scheduler loop is currently running |
| `next_scrape_at` | Next scheduled scrape. Omitted until the scheduler has computed one |
| `last_scheduled_scrape_at` | Last scrape the scheduler triggered. Omitted before the first run |
| `providers.<name>.last_scrape_success` | Outcome of the most recent attempt |
| `providers.<name>.last_response_time_ms` | Duration of the most recent API call |
| `providers.<name>.last_price` | Latest price in EUR per 100L (`heizsaison-oil`) |
| `providers.<name>.last_temperature` | Latest temperature in Celsius (`heizsaison-weather`). Omitted when unset |
| `providers.<name>.last_error` | Error message of the most recent failure, `null` after a success |
| `providers.<name>.last_raw_response` | Raw API response of the most recent scrape, truncated at 10000 characters. Omitted when the provider returned none |
| `database.connected` | Result of a live `Ping()` against PostgreSQL |
| `database.total_prices_stored` / `total_observations_stored` | Row count of the respective table |

---

## `/health` - Health Check

Returns `200 OK` with the body `OK` whenever the process is running.

The handler performs no checks of its own, so this is a **liveness** probe, not a readiness probe. Use `/status` and its `database.connected` field to decide whether the service is actually able to work.

---

## Gotchas

- **The HTTP server only runs under `run`.** `scrape`, `backfill` and `version` exit when finished and never bind a port, so there is nothing to scrape metrics from during a one-off run.
- **`last_raw_response` ignores `--store-raw-response`.** That flag only controls whether the raw payload is written to the `raw_response` database column; the `/status` field is populated either way and can make the response large.
- **Provider counters are in-memory.** `total_requests` and `total_errors` count since process start, not since the beginning of time. Use `<ns>_prices_stored_total` / `<ns>_observations_stored_total` for persistent counts.
- **Both scrapers default to different ports for a reason.** Running them on one host with the same `HTTP_ADDR` makes the second one fail to bind.
