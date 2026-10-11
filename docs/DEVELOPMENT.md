# Development

How to build, test and release the two scrapers. For the settings they accept once running, see [CONFIGURATION.md](CONFIGURATION.md).

---

## Prerequisites

- Go 1.26+
- Docker and Docker Compose
- PostgreSQL 18+, or use the one in `docker-compose.yml`

---

## Make Targets

`make help` lists everything. CI runs `fmt-check`, `vet`, `staticcheck`, `build-all` and `test-ci`.

| Target | Description |
|--------|-------------|
| `make help` | List all targets |
| `make test` | Run all unit tests with the race detector |
| `make test-ci` | Same, writing a coverage profile to `coverage.out` |
| `make code-coverage` | Generate and open an HTML coverage report |
| `make fmt-check` | Fail if any Go file is not gofmt'ed |
| `make vet` | Run `go vet` |
| `make staticcheck` | Run the staticcheck analyzer |
| `make build-oil` | Compile `heizsaison-oil` |
| `make build-weather` | Compile `heizsaison-weather` |
| `make build-all` | Compile both |

---

## Running Locally

Start PostgreSQL on its own, then run either scraper against it. The schema in `database-schema/` is applied automatically on first start of the container.

```bash
docker-compose up -d postgres
```

```bash
# Oil prices
go run ./cmd/heizsaison-oil run \
  --postgres-dsn "postgres://heizsaison:heizsaison@localhost:5432/heizsaison?sslmode=disable" \
  --zip-code "47259" \
  --log-format console \
  --log-level debug
```

```bash
# Weather
go run ./cmd/heizsaison-weather run \
  --postgres-dsn "postgres://heizsaison:heizsaison@localhost:5432/heizsaison?sslmode=disable" \
  --latitude 51.4556 \
  --longitude 6.7623 \
  --log-format console \
  --log-level debug
```

`run` waits for the next scheduled hour. Use `scrape` instead to fetch once and exit. See [DEBUG_LOGGING.md](DEBUG_LOGGING.md) for what the debug level adds.

---

## Docker Development

```bash
# Build and run postgres and both scrapers
docker-compose up --build

# Follow the logs of one service
docker-compose logs -f oil
docker-compose logs -f weather

# Open a database shell
docker exec -it heizsaison-postgres psql -U heizsaison -d heizsaison
```

Both application services already run at `LOG_LEVEL: debug` and `LOG_FORMAT: console`. For queries to run against the collected data, see [OIL_EXAMPLE_QUERIES.md](OIL_EXAMPLE_QUERIES.md) and [WEATHER_EXAMPLE_QUERIES.md](WEATHER_EXAMPLE_QUERIES.md).

---

## Releasing a New Version

1. Ensure all changes are committed and pushed to `main`.
2. Create and push a version tag:

   ```bash
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

3. Goreleaser then builds and publishes:
   - `heizsaison-oil` and `heizsaison-weather` binaries for Linux and macOS on amd64 and arm64
   - A GitHub Release with a generated changelog
   - `ghcr.io/andygrunwald/heizsaison-oil` and `ghcr.io/andygrunwald/heizsaison-weather`, each tagged with the full version, `major.minor` and `latest`
