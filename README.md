# Lidarr Deduper

A Go application that automatically detects and removes duplicate singles from your Lidarr music library. The tool identifies singles that are already included in downloaded albums or EPs by the same artist and removes them to clean up your library.

## Features

- **Duplicate Detection**: Identifies singles that are duplicated in albums/EPs using multiple matching methods:
  - MusicBrainz Recording IDs (most reliable)
  - MusicBrainz Track IDs
  - Normalized title matching (fallback)
- **Import Exclusion Option**: Optionally add removed singles to Lidarr's import exclusion list to prevent future re-imports
- **Dry Run Support**: Preview what would be deleted before making changes
- **Scheduled Execution**: Run once or on a cron schedule
- **Configuration Options**: Support for both config files and environment variables
- **Docker Support**: Includes Dockerfile and docker-compose for easy deployment
- **Progress Tracking**: Real-time logging of processing status

## Installation

### Binary Release

Download the latest release from the [GitHub releases page](https://github.com/benscobie/lidarr-deduper/releases).

### From Source

```bash
git clone https://github.com/benscobie/lidarr-deduper.git
cd lidarr-deduper
go build -o lidarr-deduper .
```

### Docker

```bash
docker build -t lidarr-deduper .
# or
docker-compose up
```

## Configuration

### Config File

Create a `config.yaml` file:

```yaml
# Lidarr connection settings
lidarr:
  url: "http://lidarr.home.arpa"
  api_key: "your-api-key-here"

# Application settings
app:
  dry_run: true                   # Set to false to actually process duplicates
  add_import_exclusion: false     # Set to true to add removed singles to import exclusion list
  log_level: "info"

# Scheduling settings
schedule:
  enabled: false                  # Enable scheduled runs
  cron: "0 2 * * *"              # Run daily at 2 AM
  run_once: true                  # Run once and exit
```

### Environment Variables

All configuration options can be set via environment variables:

```bash
export LIDARR_DEDUPE_LIDARR_URL="http://lidarr.home.arpa"
export LIDARR_DEDUPE_LIDARR_API_KEY="your-api-key-here"
export LIDARR_DEDUPE_APP_DRY_RUN="true"
export LIDARR_DEDUPE_APP_ADD_IMPORT_EXCLUSION="false"
export LIDARR_DEDUPE_SCHEDULE_ENABLED="false"
export LIDARR_DEDUPE_SCHEDULE_CRON="0 2 * * *"
```

## Usage

### Command Line Options

```bash
# Run with dry-run (preview only)
./lidarr-deduper --dry-run=true

# Actually process duplicates (unmonitor and delete files)
./lidarr-deduper --dry-run=false

# Also add removed singles to import exclusion list
./lidarr-deduper --dry-run=false --add-import-exclusion=true

# Run on a schedule
./lidarr-deduper --cron="0 2 * * *"

# Use custom config file
./lidarr-deduper --config=/path/to/config.yaml
```

### Docker


#### One-time run (dry-run):
```bash
docker run --rm \
  -e LIDARR_DEDUPE_LIDARR_URL="http://your-lidarr:8686" \
  -e LIDARR_DEDUPE_LIDARR_API_KEY="your-api-key" \
  -e LIDARR_DEDUPE_APP_DRY_RUN="true" \
  lidarr-deduper
```

#### Actually process duplicates (unmonitor and delete files):
```bash
docker run --rm \
  -e LIDARR_DEDUPE_LIDARR_URL="http://your-lidarr:8686" \
  -e LIDARR_DEDUPE_LIDARR_API_KEY="your-api-key" \
  -e LIDARR_DEDUPE_APP_DRY_RUN="false" \
  lidarr-deduper
```

#### Also add removed singles to import exclusion list:
```bash
docker run --rm \
  -e LIDARR_DEDUPE_LIDARR_URL="http://your-lidarr:8686" \
  -e LIDARR_DEDUPE_LIDARR_API_KEY="your-api-key" \
  -e LIDARR_DEDUPE_APP_DRY_RUN="false" \
  -e LIDARR_DEDUPE_APP_ADD_IMPORT_EXCLUSION="true" \
  lidarr-deduper
```

#### Docker Compose:
```bash
# Edit docker-compose.yml with your settings
docker-compose up -d
```

## How It Works

1. **Artist Processing**: Scans all artists in your Lidarr library
2. **Album Filtering**: Identifies albums with downloaded files
3. **Single Detection**: Determines which albums are singles based on:
   - Album type metadata
   - Track count (1-2 tracks)
   - Exclusion of EPs and full albums
4. **Duplicate Matching**: Compares single tracks against tracks in albums/EPs using:
   - MusicBrainz Recording IDs (preferred)
   - MusicBrainz Track IDs
   - Normalized title matching
5. **Cleanup**: Optionally removes duplicate singles and adds to exclusion list

## Deletion Behavior

- **Default**: Unmonitors the duplicate single and deletes its files, but keeps the record in Lidarr
- **Import Exclusion Option**: If `--add-import-exclusion` is set, also adds the single to Lidarr's import exclusion list to prevent future re-imports

## Safety Features

- **Dry Run Default**: Always defaults to dry-run mode to prevent accidental deletions
- **READ-ONLY Test Mode**: Includes test mode against your actual Lidarr instance
- **Detailed Logging**: Shows exactly what will be deleted and why
- **Progressive Processing**: Adds delays between API calls to avoid overwhelming Lidarr

## Example Output

```
=== DUPLICATE DETECTION SUMMARY ===
Found 3 duplicate singles

1. Single: 'Bohemian Rhapsody' by Queen
   Reason: Track 'Bohemian Rhapsody' found in album 'A Night at the Opera'
   Found in 1 other album(s):
     - Album: 'A Night at the Opera'
  Action: [DRY RUN] Would unmonitor and delete files

2. Single: 'Imagine' by John Lennon
  Reason: Track 'Imagine' found in album 'Imagine'
  Found in 1 other album(s):
    - Album: 'Imagine'
  Action: [DRY RUN] Would unmonitor and delete files and add to exclusion list

This was a dry run. To actually process duplicates, run with --dry-run=false
```

## Requirements

- Go 1.24+ (if building from source)
- Access to Lidarr API
- Lidarr v1.0+

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Disclaimer

This tool modifies your Lidarr library by unmonitoring and deleting files for duplicate singles. Always run with `--dry-run=true` first to preview changes. The authors are not responsible for any data loss. Use at your own risk and ensure you have backups of your music library.

## Support

- Create an issue on GitHub for bug reports or feature requests
- Check existing issues before creating new ones
- Provide logs and configuration details when reporting issues
