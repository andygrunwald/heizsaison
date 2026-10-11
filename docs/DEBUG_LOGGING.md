# Debug Logging

Both scrapers log at `info` level in JSON format by default. This document describes how to switch to `debug`, which reveals the outbound API requests, the per-record insert and skip decisions, and other details that are otherwise hidden.

Logging is handled by [zerolog](https://github.com/rs/zerolog). The `heizsaison-oil` and `heizsaison-weather` binaries are configured identically. For every other setting, see [CONFIGURATION.md](CONFIGURATION.md).

---

## Enable It

`--log-level` and `--log-format` are persistent flags, so they work on the `run`, `scrape` and `backfill` subcommands.

```bash
# Via flags
heizsaison-oil scrape --log-level debug --log-format console

# Via environment variables
LOG_LEVEL=debug LOG_FORMAT=console heizsaison-weather run
```

| Setting | Flag | Env Variable | Default | Values |
|---------|------|--------------|---------|--------|
| Level | `--log-level` | `LOG_LEVEL` | `info` | `trace`, `debug`, `info`, `warn`, `error` |
| Format | `--log-format` | `LOG_FORMAT` | `json` | `json`, `console` |

`console` produces colored, human-readable output and is the better choice for local development. `json` is the better choice for log aggregation. All output goes to stdout.

---

## Precedence

Settings are resolved in this order, with the last one winning:

1. Built-in default (`info` and `json`)
2. Environment variable (`LOG_LEVEL`, `LOG_FORMAT`)
3. Command-line flag, when explicitly passed

---

## Docker

Both application services in `docker-compose.yml` already run with `LOG_LEVEL: debug` and `LOG_FORMAT: console`, so `docker-compose up` yields debug output without any change.

For the published images, pass the environment variables:

```bash
docker run \
  -e LOG_LEVEL=debug \
  -e LOG_FORMAT=console \
  ghcr.io/andygrunwald/heizsaison-oil
```

---

## What You Get at Debug

| Source | Additional output |
|--------|-------------------|
| API providers | Outbound request URL and requested date range for HeizOel24, Hoyer and DWD CDC |
| Scraper | `price already exists, skipping` for records that are not written again |
| Database | Details of every inserted price and weather record |
| Visual Crossing | The query cost the API charged for the request |

---

## Filtering

Every component logs with a `component` field (`oil`, `weather`, `database`, `scheduler`, `http`), and every API provider with a `provider` field. In JSON format, filter on them with `jq`:

```bash
heizsaison-oil scrape --log-level debug | jq 'select(.component == "database")'
```

---

## Gotchas

- **Invalid levels fall back silently**: `--log-level verbose` does not raise an error, it logs at `info`.
- **The format check is exact**: only `console` selects the human-readable writer. `Console` or `text` produce JSON.
- **`version` produces no log output**: it prints directly to stdout and never builds a logger, so both flags have no effect there.
