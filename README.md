# lidarr-utils

A collection of Go-based utilities for managing your Lidarr music library. Currently provides two commands:

- **dedupe** -- detect and remove duplicate singles that already exist in albums or EPs
- **monitor** -- intelligently monitor and trigger searches for an artist's catalogue

## Features

### dedupe

- Identifies singles duplicated in albums/EPs using multiple matching methods:
  - MusicBrainz Recording IDs (most reliable)
  - MusicBrainz Track IDs
  - Normalized title matching (fallback)
- Optionally adds removed singles to Lidarr's import exclusion list
- Supports one-time and cron-scheduled execution

### monitor

- Intelligently selects which releases to monitor, avoiding redundant downloads:

  > **Example:** An artist has a studio album containing tracks A, B, and C. They also
  > released an EP with tracks A and B, and a single for track C. Since all EP and single
  > tracks already appear on the album, only the album is monitored — the EP and single are
  > skipped. If the EP also contained a bonus track D, it would be monitored too, because
  > that track isn't covered by any higher-priority release.
  >
  > Priority order: **Albums > EPs > Singles**. Tracks are matched by MusicBrainz Recording
  > ID when available, falling back to normalised title comparison.

- Filters by secondary album types (e.g. exclude Live, Compilation)
- Batch monitor and search via Lidarr's API
- Process a single artist or all artists at once

### Common

- Dry run mode
- Config file and environment variable support
- Structured logging to stdout and optional log file
- Docker support with Dockerfile and docker-compose

## Installation

### Binary Release

Download the latest release from the releases page.

### From Source

```bash
git clone https://github.com/benscobie/lidarr-utils.git
cd lidarr-utils
go build -o lidarr-utils .
```

### Docker

```bash
docker build -t lidarr-utils .
# or
docker-compose up
```

## Configuration

### Config File

Create a `config.yaml` file (or copy from `config.example.yaml`):

```yaml
# Lidarr connection settings
lidarr:
  url: "http://localhost:8686"
  api_key: "your-api-key-here"

# Application settings
app:
  dry_run: false
  log_level: "info"
  log_file: "lidarr-utils.log"

# Dedupe command settings
dedupe:
  add_import_exclusion: false

# Monitor command settings
monitor:
  official_only: false
  exclude_secondary_types: []
  exclude_formats: []

# Scheduling settings (dedupe only)
schedule:
  enabled: false
  cron: "0 2 * * *"
  run_once: true
```

### Environment Variables

All configuration options can be set via environment variables with the `LIDARR_UTILS_` prefix:

```bash
# Lidarr connection
export LIDARR_UTILS_LIDARR_URL="http://localhost:8686"
export LIDARR_UTILS_LIDARR_API_KEY="your-api-key-here"

# App settings
export LIDARR_UTILS_APP_DRY_RUN="true"
export LIDARR_UTILS_APP_LOG_LEVEL="info"
export LIDARR_UTILS_APP_LOG_FILE="lidarr-utils.log"

# Dedupe settings
export LIDARR_UTILS_DEDUPE_ADD_IMPORT_EXCLUSION="false"

# Monitor settings
export LIDARR_UTILS_MONITOR_OFFICIAL_ONLY="false"
export LIDARR_UTILS_MONITOR_EXCLUDE_SECONDARY_TYPES=""

# Schedule settings
export LIDARR_UTILS_SCHEDULE_ENABLED="false"
export LIDARR_UTILS_SCHEDULE_CRON="0 2 * * *"
export LIDARR_UTILS_SCHEDULE_RUN_ONCE="true"
```

## Usage

### dedupe

```bash
# Preview duplicate detection (dry run)
./lidarr-utils dedupe --dry-run

# Remove duplicate singles
./lidarr-utils dedupe

# Also add removed singles to the import exclusion list
./lidarr-utils dedupe --add-import-exclusion

# Run on a cron schedule
./lidarr-utils dedupe --cron="0 2 * * *"

# Use a custom config file
./lidarr-utils dedupe --config=/path/to/config.yaml
```

### monitor

```bash
# Preview what would be monitored (dry run)
./lidarr-utils monitor --artist-id 123 --dry-run

# Monitor and search a single artist
./lidarr-utils monitor --artist-id 123

# Monitor and search all artists
./lidarr-utils monitor --all

# Only process official albums (no secondary types)
./lidarr-utils monitor --all --official-only

# Exclude specific secondary types
./lidarr-utils monitor --all --exclude-secondary-types=Live,Compilation
```

### Docker

#### dedupe -- one-time dry run:

```bash
docker run --rm \
  -e LIDARR_UTILS_LIDARR_URL="http://your-lidarr:8686" \
  -e LIDARR_UTILS_LIDARR_API_KEY="your-api-key" \
  -e LIDARR_UTILS_APP_DRY_RUN="true" \
  lidarr-utils dedupe
```

#### dedupe -- remove duplicates:

```bash
docker run --rm \
  -e LIDARR_UTILS_LIDARR_URL="http://your-lidarr:8686" \
  -e LIDARR_UTILS_LIDARR_API_KEY="your-api-key" \
  -e LIDARR_UTILS_APP_DRY_RUN="false" \
  lidarr-utils dedupe
```

#### monitor -- process all artists:

```bash
docker run --rm \
  -e LIDARR_UTILS_LIDARR_URL="http://your-lidarr:8686" \
  -e LIDARR_UTILS_LIDARR_API_KEY="your-api-key" \
  -e LIDARR_UTILS_APP_DRY_RUN="false" \
  lidarr-utils monitor --all
```

#### Docker Compose:

```bash
# Edit docker-compose.yml with your settings
docker-compose up -d
```

## How dedupe Works

1. **Artist Processing** -- scans all artists in your Lidarr library
2. **Album Filtering** -- identifies albums with downloaded files
3. **Single Detection** -- determines which albums are singles based on album type metadata, track count (1-2 tracks), and exclusion of EPs and full albums
4. **Duplicate Matching** -- compares single tracks against tracks in albums/EPs using MusicBrainz Recording IDs (preferred), MusicBrainz Track IDs, and normalized title matching
5. **Cleanup** -- unmonitors duplicate singles, deletes their files, and optionally adds them to the import exclusion list

## How monitor Works

1. **Album Selection** -- retrieves all albums for each artist and groups them by priority: Album > EP > Single
2. **Track Coverage** -- builds a set of MusicBrainz Recording IDs already covered by higher-priority releases. Lower-priority releases whose tracks are fully covered are skipped.
3. **Filtering** -- applies secondary type filters (`--official-only`, `--exclude-secondary-types`) to remove unwanted release types
4. **Monitor & Search** -- batch monitors selected albums and submits a single album search to Lidarr, which processes them sequentially

## Safety Features

- **Dry Run Mode** -- use `--dry-run` to preview changes before applying them
- **Detailed Logging** -- shows exactly what will be changed and why
- **Batch Operations** -- uses Lidarr's native command queue for search processing

## Requirements

- Go 1.25+ (if building from source)
- Access to Lidarr API
- Lidarr v1.0+

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Disclaimer

This tool modifies your Lidarr library. The dedupe command unmonitors and deletes files for duplicate singles. The monitor command changes album monitoring state and triggers searches. Use `--dry-run` first to preview changes. The authors are not responsible for any data loss. Use at your own risk and ensure you have backups of your music library.
