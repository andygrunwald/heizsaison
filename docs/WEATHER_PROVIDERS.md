# Weather Providers

This document describes each weather provider integrated into the weather scraper, including API details, data coverage, authentication requirements, and a detailed breakdown of which data points are scraped and stored versus available but not captured.

All providers return daily weather observations that are stored in the `weather_observations` PostgreSQL table. See [WEATHER_EXAMPLE_QUERIES.md](WEATHER_EXAMPLE_QUERIES.md) for SQL query examples.

## Table of Contents

- [Data Points Stored](#data-points-stored)
- [Provider Comparison](#provider-comparison)
- [Open-Meteo](#open-meteo)
- [Bright Sky (DWD)](#bright-sky-dwd)
- [Visual Crossing](#visual-crossing)
- [OpenWeather One Call 3.0](#openweather-one-call-30)
- [DWD CDC-OpenData](#dwd-cdc-opendata)
- [General Limitations](#general-limitations)

---

## Data Points Stored

Every provider maps its response into the same unified schema. The following columns are stored in the `weather_observations` table for each daily observation:

| Column | Type | Unit | Description |
|--------|------|------|-------------|
| `provider` | VARCHAR(50) | — | Provider identifier (e.g., `openmeteo`, `brightsky`) |
| `observation_date` | DATE | — | The date the observation covers |
| `latitude` | DECIMAL(9,6) | degrees | Location latitude (rounded to 4 decimals) |
| `longitude` | DECIMAL(9,6) | degrees | Location longitude (rounded to 4 decimals) |
| `temperature_min_c` | DECIMAL(6,2) | °C | Daily minimum air temperature |
| `temperature_max_c` | DECIMAL(6,2) | °C | Daily maximum air temperature |
| `temperature_mean_c` | DECIMAL(6,2) | °C | Daily mean air temperature |
| `precipitation_mm_sum` | DECIMAL(8,2) | mm | Total daily precipitation |
| `wind_speed_max_kmh` | DECIMAL(6,2) | km/h | Maximum wind speed |
| `wind_gust_max_kmh` | DECIMAL(6,2) | km/h | Maximum wind gust speed |
| `sunshine_duration_s` | DECIMAL(10,2) | seconds | Total sunshine duration |
| `cloud_cover_percent` | DECIMAL(5,2) | % (0–100) | Mean daily cloud cover |
| `humidity_percent` | DECIMAL(5,2) | % (0–100) | Mean daily relative humidity |
| `pressure_hpa` | DECIMAL(7,2) | hPa | Mean sea-level air pressure |
| `raw_response` | JSONB | — | Original API response (optional, controlled by `--store-raw-response`) |

All weather fields are nullable — a `NULL` value means the provider did not supply that data point for the given day.

---

## Provider Comparison

### Data Availability

| Data Point | Open-Meteo | Bright Sky | Visual Crossing | OpenWeather | DWD CDC |
|------------|:----------:|:----------:|:---------------:|:-----------:|:-------:|
| Temperature min | ✅ | ✅ | ✅ | ✅ | ✅ |
| Temperature max | ✅ | ✅ | ✅ | ✅ | ✅ |
| Temperature mean | ✅ | ✅ | ✅ | ✅ (calculated) | ✅ |
| Precipitation | ✅ | ✅ | ✅ | ✅ | ✅ |
| Wind speed max | ✅ | ✅ | ✅ | ✅ | ✅ |
| Wind gust max | ✅ | ✅ | ✅ | ❌ | ✅ |
| Sunshine duration | ✅ | ✅ | ❌ | ❌ | ✅ |
| Cloud cover | ✅ | ✅ | ✅ | ✅* | ✅ |
| Humidity | ✅ | ✅ | ✅ | ✅* | ✅ |
| Pressure | ✅ | ✅ | ✅ | ✅* | ✅ |

*\* OpenWeather provides only afternoon snapshot values, not daily averages.*

### Provider Features

| Feature | Open-Meteo | Bright Sky | Visual Crossing | OpenWeather | DWD CDC |
|---------|:----------:|:----------:|:---------------:|:-----------:|:-------:|
| API Key Required | No | No | Yes | Yes | No |
| Free Tier | 10,000 calls/day | Unlimited | 1,000 records/day | 1,000 calls/day | Unlimited |
| Historical Depth | 1940+ | 2010+ | 2015+ | 1979+ | 50+ years |
| Backfill Support | ✅ | ✅ | ✅ | ✅ | ✅ |
| Data Source | ERA5 reanalysis | DWD stations | Multiple | Multiple | DWD stations |
| Daily Aggregates | Direct | Hourly → daily | Direct | Direct | Direct |
| Response Format | JSON | JSON | JSON | JSON | ZIP/CSV |
| License | CC BY 4.0 | DWD terms | Commercial | Commercial | CC BY 4.0 |

---

## Open-Meteo

- **Website**: [https://open-meteo.com](https://open-meteo.com)
- **API Docs**: [https://open-meteo.com/en/docs](https://open-meteo.com/en/docs)
- **Historical API Docs**: [https://open-meteo.com/en/docs/historical-weather-api](https://open-meteo.com/en/docs/historical-weather-api)
- **Provider name**: `openmeteo`
- **API Key**: Not required (free tier for non-commercial use)
- **Rate Limits**: 10,000 calls/day, 5,000/hour, 600/minute
- **License**: CC BY 4.0

### Endpoints

| Purpose | URL |
|---------|-----|
| Current/Forecast | `https://api.open-meteo.com/v1/forecast` |
| Historical Archive | `https://archive-api.open-meteo.com/v1/archive` |

### Data Points

The API returns daily aggregates directly via the `daily` parameter. The following daily variables are requested:

| API Parameter | Stored As | Unit Conversion |
|---------------|-----------|-----------------|
| `temperature_2m_max` | `temperature_max_c` | None (°C) |
| `temperature_2m_min` | `temperature_min_c` | None (°C) |
| `temperature_2m_mean` | `temperature_mean_c` | None (°C) |
| `precipitation_sum` | `precipitation_mm_sum` | None (mm) |
| `wind_speed_10m_max` | `wind_speed_max_kmh` | None (km/h) |
| `wind_gusts_10m_max` | `wind_gust_max_kmh` | None (km/h) |
| `sunshine_duration` | `sunshine_duration_s` | None (seconds) |
| `cloud_cover_mean` | `cloud_cover_percent` | None (%) |
| `relative_humidity_2m_mean` | `humidity_percent` | None (%) |
| `surface_pressure_mean` | `pressure_hpa` | None (hPa) |

All units match the storage format — no conversions needed. Open-Meteo is the most straightforward provider.

### Additional API Parameters Available (Not Stored)

The Open-Meteo API offers many more daily variables that are not currently scraped:

- `apparent_temperature_max`, `apparent_temperature_min`, `apparent_temperature_mean` — Feels-like temperature
- `precipitation_hours` — Number of hours with precipitation
- `precipitation_probability_max` — Probability of precipitation (forecast only)
- `rain_sum`, `showers_sum`, `snowfall_sum` — Precipitation broken down by type
- `wind_direction_10m_dominant` — Dominant wind direction
- `shortwave_radiation_sum` — Solar radiation
- `et0_fao_evapotranspiration` — Reference evapotranspiration
- `weather_code` — WMO weather interpretation code
- `uv_index_max`, `uv_index_clear_sky_max` — UV index

### Notes

- Uses ERA5 reanalysis data for historical records, providing global coverage back to 1940
- ERA5 data is model-based (not direct station measurements) with ~10km resolution
- Data updates with ~5 days delay for ERA5
- The `timezone` parameter is set to `Europe/Berlin` to ensure correct day boundaries

### Known Limitations

- **Model data, not measurements**: ERA5 reanalysis data is produced by a weather model, not direct station observations. It may diverge from local ground truth, especially for precipitation and microclimate effects.
- **~10km spatial resolution**: ERA5 grid cells are approximately 10×10 km. Fine-grained local variations (urban heat islands, valley effects) are smoothed out.
- **~5-day data delay**: ERA5 archive data lags about 5 days behind real-time. The most recent days may not be available in the historical archive.
- **Timezone hardcoded**: The `timezone` parameter is set to `Europe/Berlin`. Using this provider for locations in other timezones would produce incorrect day boundaries.
- **Free tier fair-use limits**: While no API key is required, the free tier has fair-use limits (10,000 calls/day). No rate limiting or backoff is implemented in the scraper.
- **No retry logic**: A single HTTP request is made. Network failures cause immediate failure with no automatic retry.

---

## Bright Sky (DWD)

- **Website**: [https://brightsky.dev](https://brightsky.dev)
- **API Docs**: [https://brightsky.dev/docs](https://brightsky.dev/docs)
- **GitHub**: [https://github.com/jdemaeyer/brightsky](https://github.com/jdemaeyer/brightsky)
- **Provider name**: `brightsky`
- **API Key**: Not required
- **Rate Limits**: No explicit per-user limits
- **License**: DWD data usage terms apply

### Endpoints

| Purpose | URL |
|---------|-----|
| Weather data | `https://api.brightsky.dev/weather` |

### Data Points

Bright Sky returns **hourly** observations. The scraper aggregates them into daily values using the following methods:

| API Field (Hourly) | Aggregation | Stored As | Unit Conversion |
|---------------------|-------------|-----------|-----------------|
| `temperature` | min of day | `temperature_min_c` | None (°C) |
| `temperature` | max of day | `temperature_max_c` | None (°C) |
| `temperature` | mean of day | `temperature_mean_c` | None (°C) |
| `precipitation` | sum of day | `precipitation_mm_sum` | None (mm) |
| `wind_speed` | max of day | `wind_speed_max_kmh` | None (km/h) |
| `wind_gust_speed` | max of day | `wind_gust_max_kmh` | None (km/h) |
| `sunshine` | sum of day | `sunshine_duration_s` | minutes → seconds (×60) |
| `cloud_cover` | mean of day | `cloud_cover_percent` | None (%) |
| `relative_humidity` | mean of day | `humidity_percent` | None (%) |
| `pressure_msl` | mean of day | `pressure_hpa` | None (hPa) |

### Additional API Fields Available (Not Stored)

- `wind_direction` — Wind direction in degrees (0–360)
- `dew_point` — Dew point temperature in °C
- `visibility` — Visibility in meters
- `condition` — Human-readable condition string (e.g., "dry", "rain", "snow")
- `icon` — Weather icon identifier (e.g., "clear-night", "cloudy")
- `source_id` — Reference to the DWD station providing the data

The `/sources` endpoint also returns station metadata (DWD station ID, station name, coordinates, elevation) which is not stored.

### Notes

- Data comes from official DWD weather stations across Germany
- Historical data available from January 2010 onwards
- The API returns the nearest DWD station's data for the given coordinates
- Hourly data is aggregated to daily by the scraper — this means daily values may differ slightly from DWD's own daily aggregates due to different aggregation methods or timezone handling
- For large backfill ranges, requests are chunked into 30-day windows to avoid timeouts

### Known Limitations

- **Hourly-to-daily aggregation**: The API returns hourly observations which the scraper aggregates to daily values (min, max, mean, sum). This may produce slightly different results than DWD's own official daily aggregates, especially around day boundaries and timezone transitions.
- **Germany-only**: Only covers German territory (DWD station network). No international data.
- **Data from 2010 only**: Historical data starts from January 2010. Older data is not available through Bright Sky.
- **30-day request chunking**: Large backfill ranges are split into 30-day chunks to avoid API timeouts. This increases the number of HTTP requests for multi-year backfills.
- **60-second HTTP timeout**: May be insufficient for very large date ranges within a single chunk.
- **No retry logic**: A single HTTP request per chunk. Network failures cause immediate failure of that chunk.
- **Wind direction dropped**: The API provides hourly wind direction, but it is not aggregated or stored (daily wind direction aggregation is ambiguous).
- **Station selection opaque**: Bright Sky automatically selects the nearest DWD station. The user has no control over which station is used, and the selected station is not logged.

---

## Visual Crossing

- **Website**: [https://www.visualcrossing.com](https://www.visualcrossing.com)
- **API Docs**: [https://www.visualcrossing.com/resources/documentation/weather-api/timeline-weather-api/](https://www.visualcrossing.com/resources/documentation/weather-api/timeline-weather-api/)
- **Provider name**: `visualcrossing`
- **API Key**: Required (free signup at [https://www.visualcrossing.com/sign-up](https://www.visualcrossing.com/sign-up))
- **Rate Limits**: 1,000 records/day on free tier
- **License**: Commercial (free tier available)

### Endpoints

| Purpose | URL |
|---------|-----|
| Timeline API | `https://weather.visualcrossing.com/VisualCrossingWebServices/rest/services/timeline/{location}/{date1}/{date2}` |

Parameters: `key={API_KEY}&include=days&unitGroup=metric&contentType=json`

### Data Points

| API Field | Stored As | Unit Conversion |
|-----------|-----------|-----------------|
| `tempmin` | `temperature_min_c` | None (°C) |
| `tempmax` | `temperature_max_c` | None (°C) |
| `temp` | `temperature_mean_c` | None (°C) |
| `precip` | `precipitation_mm_sum` | None (mm) |
| `windspeed` | `wind_speed_max_kmh` | None (km/h) |
| `windgust` | `wind_gust_max_kmh` | None (km/h) |
| `cloudcover` | `cloud_cover_percent` | None (%) |
| `humidity` | `humidity_percent` | None (%) |
| `pressure` | `pressure_hpa` | None (hPa) |

**Not available**: Sunshine duration is not provided by the Visual Crossing daily API.

### Additional API Fields Available (Not Stored)

- `feelslike`, `feelslikemax`, `feelslikemin` — Apparent temperature
- `winddir` — Wind direction in degrees
- `uvindex` — UV index
- `sunrise`, `sunset` — Sunrise/sunset times
- `moonphase` — Moon phase (0–1)
- `conditions` — Short text condition (e.g., "Partly cloudy")
- `description` — Detailed text description of the day
- `icon` — Weather icon identifier
- `stations` — List of contributing weather stations
- `severerisk` — Severe weather risk percentage
- `snowdepth` — Snow depth
- `snow` — Snowfall amount
- `preciptype` — Precipitation type(s)
- `precipprob` — Precipitation probability
- `precipcover` — Percentage of time with precipitation
- `visibility` — Visibility distance
- `solarradiation`, `solarenergy` — Solar radiation data
- `source` — Data source identifier
- `queryCost` — API units consumed (logged at debug level)

### Notes

- The `queryCost` response field tells you how many of your daily free records were consumed
- Supports address-based queries, but we use coordinates for consistency
- `unitGroup=metric` is set to receive all values in metric units
- The Timeline API can return forecast and historical data in a single unified endpoint
- Free tier tracks records (1 day = 1 record), not API calls

### Known Limitations

- **API key required**: Requires free registration. The key is passed in the URL query string.
- **No sunshine duration**: The Visual Crossing daily API does not provide sunshine duration data. This field will always be `NULL`.
- **Cost-based rate limiting**: The free tier allows 1,000 records per day. Each day of data counts as one record. A backfill of 1,000 days consumes the entire daily quota. There is no rate limiting or quota tracking in the scraper.
- **No backoff on rate limit errors**: If the daily quota is exceeded, the API returns an error but the scraper does not implement exponential backoff or pause-and-resume.
- **No retry logic**: A single HTTP request per scrape. Network failures cause immediate failure.
- **API key in URL**: The API key is passed as a query parameter, which may appear in server logs and monitoring tools.
- **Commercial terms**: Subject to Visual Crossing's terms of service. The free tier may have restrictions on commercial use.

---

## OpenWeather One Call 3.0

- **Website**: [https://openweathermap.org](https://openweathermap.org)
- **API Docs**: [https://openweathermap.org/api/one-call-3](https://openweathermap.org/api/one-call-3)
- **Day Summary Docs**: [https://openweathermap.org/api/one-call-3#day_summary](https://openweathermap.org/api/one-call-3#day_summary)
- **Provider name**: `openweather`
- **API Key**: Required (sign up at [https://home.openweathermap.org/users/sign_up](https://home.openweathermap.org/users/sign_up))
- **Rate Limits**: 1,000 calls/day free
- **License**: Commercial (free tier available)

### Endpoints

| Purpose | URL |
|---------|-----|
| Day Summary | `https://api.openweathermap.org/data/3.0/onecall/day_summary` |

Parameters: `lat={LAT}&lon={LON}&date={YYYY-MM-DD}&appid={API_KEY}&units=metric`

**Important**: This endpoint returns data for **one day per request**. Backfilling a year requires 365 API calls. The provider includes configurable delays between requests to respect rate limits.

### Data Points

| API Field | Stored As | Unit Conversion |
|-----------|-----------|-----------------|
| `temperature.min` | `temperature_min_c` | None (°C) |
| `temperature.max` | `temperature_max_c` | None (°C) |
| (`min` + `max`) / 2 | `temperature_mean_c` | Calculated average |
| `precipitation.total` | `precipitation_mm_sum` | None (mm) |
| `wind.max.speed` | `wind_speed_max_kmh` | m/s → km/h (×3.6) |
| `cloud_cover.afternoon` | `cloud_cover_percent` | None (%) |
| `humidity.afternoon` | `humidity_percent` | None (%) |
| `pressure.afternoon` | `pressure_hpa` | None (hPa) |

**Not available**: Sunshine duration and wind gust speed are not provided by this endpoint.

**Caveat**: Cloud cover, humidity, and pressure are afternoon-only snapshots, not daily averages. This may cause these values to differ from other providers that return true daily means.

### Additional API Fields Available (Not Stored)

- `temperature.morning`, `temperature.afternoon`, `temperature.evening`, `temperature.night` — Temperature at specific times of day
- `wind.max.direction` — Wind direction in degrees
- `tz` — Timezone offset
- `units` — Unit system used in response

### Notes

- Historical data available from January 2, 1979 onwards
- Mean temperature is calculated as `(min + max) / 2` since the API does not provide a true daily mean
- The `units=metric` parameter ensures temperatures in °C and wind in m/s
- Wind speed is converted from m/s to km/h (×3.6) for consistency with other providers
- Backfill operations use `--min-delay` and `--max-delay` flags for rate limiting (random sleep between requests). `--min-delay` must be at least `1` to stay under the 60 calls/minute limit
- A warning is logged if the backfill date range exceeds 900 days for rate-limited providers

### Known Limitations

- **One day per request**: The `day_summary` endpoint only returns data for a single day. Backfilling N days requires N separate HTTP requests. A 1-year backfill requires 365 API calls.
- **API key required**: Requires registration and a One Call 3.0 subscription (free tier available).
- **1,000 calls/day free limit**: The free tier allows 1,000 API calls per day. A multi-year backfill cannot be completed in a single day. The scraper logs a warning for ranges exceeding 900 days.
- **Afternoon-only values**: Cloud cover, humidity, and pressure are snapshot values from the afternoon, not true daily averages. These may differ significantly from providers that report daily means.
- **No sunshine duration**: The day_summary endpoint does not provide sunshine duration data. This field will always be `NULL`.
- **No wind gust data**: The endpoint does not provide separate wind gust speed. Only maximum wind speed is available.
- **Calculated mean temperature**: The API does not provide a true daily mean temperature. It is calculated as `(min + max) / 2`, which can differ from the standard meteorological daily mean (typically based on multiple readings throughout the day).
- **Rate limiting via sleep**: Backfill uses random sleep delays between requests (`--min-delay` / `--max-delay`). This is simple but not adaptive — it cannot detect or respond to 429 rate limit responses.
- **No retry on individual day failures**: If fetching a single day fails during backfill, the day is skipped with an error log. It is not retried.

---

## DWD CDC-OpenData

- **Website**: [https://www.dwd.de/EN/ourservices/cdc/cdc.html](https://www.dwd.de/EN/ourservices/cdc/cdc.html)
- **Data Portal**: [https://opendata.dwd.de/climate_environment/CDC/observations_germany/climate/daily/kl/](https://opendata.dwd.de/climate_environment/CDC/observations_germany/climate/daily/kl/)
- **Dataset Description**: [https://opendata.dwd.de/climate_environment/CDC/observations_germany/climate/daily/kl/recent/DESCRIPTION_obsgermany_climate_daily_kl_recent_en.pdf](https://opendata.dwd.de/climate_environment/CDC/observations_germany/climate/daily/kl/recent/DESCRIPTION_obsgermany_climate_daily_kl_recent_en.pdf)
- **Provider name**: `dwdcdc`
- **API Key**: Not required
- **Rate Limits**: None
- **License**: CC BY 4.0 (attribution required)

### How It Works

Unlike the other providers, DWD CDC-OpenData is not a REST API. Instead, data is distributed as ZIP files on an HTTP file server. The scraper:

1. **Downloads the station list** to find the nearest weather station to the configured coordinates
2. **Downloads ZIP archives** containing semicolon-delimited daily data files
3. **Parses the text files** and converts units to match the unified schema

### Endpoints

| Purpose | URL Pattern |
|---------|-------------|
| Station list | `https://opendata.dwd.de/.../daily/kl/recent/KL_Tageswerte_Beschreibung_Stationen.txt` |
| Recent data (~500 days) | `https://opendata.dwd.de/.../daily/kl/recent/tageswerte_KL_{STATION_ID}_akt.zip` |
| Historical data | `https://opendata.dwd.de/.../daily/kl/historical/tageswerte_KL_{STATION_ID}_{FROM}_{TO}_hist.zip` |

### Data Points

The data file (`produkt_klima_tag_*.txt`) contains 19 semicolon-delimited columns. The following are extracted and stored:

| File Column | DWD Name | Stored As | Unit Conversion |
|-------------|----------|-----------|-----------------|
| `TNK` | Min temperature 2m | `temperature_min_c` | None (°C) |
| `TXK` | Max temperature 2m | `temperature_max_c` | None (°C) |
| `TMK` | Mean temperature | `temperature_mean_c` | None (°C) |
| `RSK` | Precipitation height | `precipitation_mm_sum` | None (mm) |
| `FM` | Mean wind speed | `wind_speed_max_kmh` | m/s → km/h (×3.6) |
| `FX` | Max wind gust | `wind_gust_max_kmh` | m/s → km/h (×3.6) |
| `SDK` | Sunshine duration | `sunshine_duration_s` | hours → seconds (×3600) |
| `NM` | Mean cloud cover | `cloud_cover_percent` | okta → percent (×12.5) |
| `UPM` | Mean relative humidity | `humidity_percent` | None (%) |
| `PM` | Mean air pressure | `pressure_hpa` | None (hPa) |

### File Columns Available But Not Stored

| File Column | DWD Name | Description | Why Not Stored |
|-------------|----------|-------------|----------------|
| `STATIONS_ID` | Station ID | DWD station identifier | Used internally for station lookup |
| `MESS_DATUM` | Measurement date | Date in YYYYMMDD format | Mapped to `observation_date` |
| `QN_3` | Quality level (wind) | Data quality flag for wind columns | Quality metadata, not a measurement |
| `QN_4` | Quality level (other) | Data quality flag for other columns | Quality metadata, not a measurement |
| `RSKF` | Precipitation form | Code indicating rain/snow/mixed | Not mapped to schema |
| `SHK_TAG` | Snow depth | Daily snow depth in cm | Not mapped to schema |
| `VPM` | Vapor pressure | Mean vapor pressure in hPa | Not mapped to schema |
| `TGK` | Ground min temperature | Min temperature at 5cm above ground in °C | Not mapped to schema |
| `eor` | End of record | Record terminator marker | File format artifact |

### Unit Conversion Details

- **Wind (m/s → km/h)**: The DWD reports wind speed in meters per second. Multiply by 3.6 to convert to km/h.
- **Sunshine (hours → seconds)**: The DWD reports sunshine duration in decimal hours. Multiply by 3600 to convert to seconds.
- **Cloud cover (okta → percent)**: The DWD uses the okta scale (0–8, where 0 = clear sky, 8 = fully overcast). Multiply by 12.5 to convert to percentage (0–100%).
- **Missing values**: The DWD uses `-999` to indicate missing data. These are converted to `NULL` in the database.

### Notes

- The scraper automatically discovers the nearest DWD station using Haversine distance
- Station selection is cached — only the first request triggers the station list download
- Data has a ~1 day lag: `FetchCurrentWeather` returns the most recent available day (typically yesterday)
- The `recent/` directory is updated daily but data has not yet passed final quality control
- The `historical/` directory contains quality-controlled data, updated annually
- For backfill, both historical and recent ZIPs are downloaded and merged, with duplicates removed (preferring later entries)
- The station list file and data files use Latin-1 encoding
- Germany-only coverage (~560 active stations)

### Known Limitations

- **Germany-only**: Only covers German territory (~560 active weather stations). No international data.
- **Station-based, not coordinate-based**: Data comes from the nearest DWD station, which may be several kilometers from the requested coordinates. Rural areas with sparse station coverage may have less representative data.
- **~1 day data lag**: The most recent data available is typically from yesterday. `FetchCurrentWeather` returns the most recent available day, not today.
- **Two data quality tiers**: The `recent/` data (last ~500 days) has not yet passed final quality control. The `historical/` data is quality-controlled but only updated annually. Recent data values may be revised when they move to the historical archive.
- **ZIP file downloads**: Unlike REST/JSON APIs, this provider downloads entire ZIP archives. Each scrape downloads the full recent ZIP (~5–50 KB) even if only one day is needed.
- **Latin-1 encoding**: Data files use Latin-1 encoding, not UTF-8. Station names with umlauts (e.g., "Würzburg") require special handling.
- **Historical ZIP URL construction**: The historical ZIP filename includes the station's date range, which is derived from the station list. If the station list data is stale or the date range has changed, the download may fail.
- **No retry logic**: ZIP downloads are single-attempt. Network failures cause immediate failure.
- **Missing value encoding**: DWD uses `-999` to indicate missing data. Any value ≤ -999 is treated as missing. In rare cases, this could theoretically mask extreme negative values (though none of the measured parameters reach -999 in practice).
- **Station selection is fixed**: The nearest station is cached after the first lookup. If a closer station becomes available (e.g., a new station is commissioned), the scraper will continue using the originally selected station until restarted.
- **Snow depth, vapor pressure, ground temperature dropped**: These DWD measurements are available in the data file but not stored in the database schema.

---

## General Limitations

These limitations apply to the weather scraper system as a whole, regardless of provider:

### No Retry Logic
All providers make a single HTTP request per scrape attempt. If the request fails (network error, timeout, HTTP error), the failure is logged but not retried. The next scrape attempt occurs at the next scheduled time (default: daily at 07:00).

### Sequential Provider Execution
Providers are scraped one after another, not in parallel. If one provider is slow or fails, it delays scraping of subsequent providers.

### No Circuit Breaker
There is no circuit breaker pattern. If a provider API is consistently failing, the scraper will continue attempting requests at every scheduled interval without any backoff.

### No Request Caching
Each scrape makes fresh HTTP requests. There is no caching layer to reduce API load or provide fallback data during outages.

### No Data Validation
Weather values returned by APIs are stored as-is without sanity checks. There is no validation for physically reasonable ranges (e.g., detecting a temperature of 100°C due to an API error).

### Coordinate Rounding
All coordinates are rounded to 4 decimal places (~11m precision) before storage and uniqueness checks. This prevents near-duplicate entries but means that two very close but distinct locations would be treated as the same point.

### Raw Response Storage
Raw API responses are only stored if `--store-raw-response` is enabled (default: `false`). Without this, there is no way to audit or re-parse historical API responses. Note that for DWD CDC, the "raw response" is the extracted text file contents, not the ZIP archive.

### Single Location
The weather scraper is configured with a single latitude/longitude pair. Scraping weather for multiple locations requires running multiple instances of the scraper with different configurations.
