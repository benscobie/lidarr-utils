# lidarr-utils

`lidarr-utils` is a Go CLI for managing a Lidarr music library:

- `dedupe` removes downloaded singles already covered by albums or EPs.
- `monitor artist` selects useful releases from existing artist catalogues.
- `monitor labels` discovers release groups issued by configured MusicBrainz labels.
- `schedule` runs enabled artist, label, and dedupe jobs on independent schedules.

## Installation

Download a binary from the releases page, or build from source:

```bash
git clone https://github.com/benscobie/lidarr-utils.git
cd lidarr-utils
go build -o lidarr-utils .
```

Docker images can use the same commands and configuration:

```bash
docker build -t lidarr-utils .
docker run --rm \
  -v ./config:/app/config \
  lidarr-utils monitor labels --config=/app/config/config.yaml
```

## Configuration

Copy `config.example.yaml` to `config.yaml`:

```yaml
lidarr:
  url: "http://localhost:8686"
  api_key: "your-api-key-here"

app:
  dry_run: false
  log_level: "info"
  log_file: "lidarr-utils.log"
  # state_file: /home/user/.lidarr-utils/state.json

dedupe:
  add_import_exclusion: false
  schedule:
    enabled: false
    cron: "0 2 * * *"
    run_on_start: false

monitor:
  artists:
    skip_fully_covered_releases: true
    official_only: false
    exclude_secondary_types: []
    exclude_formats: []
    exclude_va_releases: false
    schedule:
      enabled: false
      cron: "0 2 * * *"
      run_on_start: false

  labels:
    ids:
      - "a5bfec28-ea8f-426d-ab23-e14aa692c9b5"
    add_missing_artists: false
    root_folder: ""
    skip_fully_covered_releases: true
    official_only: false
    exclude_secondary_types: []
    exclude_formats: []
    exclude_va_releases: false
    schedule:
      enabled: false
      cron: "0 */6 * * *"
      run_on_start: false
```

Configuration is also available through `LIDARR_UTILS_` environment variables.
Nested keys use underscores, for example:

```bash
export LIDARR_UTILS_LIDARR_URL="http://localhost:8686"
export LIDARR_UTILS_LIDARR_API_KEY="your-api-key"
export LIDARR_UTILS_APP_DRY_RUN="true"
export LIDARR_UTILS_MONITOR_LABELS_ADD_MISSING_ARTISTS="false"
export LIDARR_UTILS_MONITOR_LABELS_SCHEDULE_ENABLED="true"
export LIDARR_UTILS_MONITOR_LABELS_SCHEDULE_CRON="0 */6 * * *"
```

## Usage

```bash
# One artist by Lidarr ID or MusicBrainz artist MBID
lidarr-utils monitor artist 123
lidarr-utils monitor artist <artist-mbid>

# All artists currently in Lidarr
lidarr-utils monitor artist

# Configured MusicBrainz labels
lidarr-utils monitor labels

# One-off label override (does not use configured IDs for this run)
lidarr-utils monitor labels a5bfec28-ea8f-426d-ab23-e14aa692c9b5

# One-off deduplication
lidarr-utils dedupe
lidarr-utils dedupe --add-import-exclusion

# The only daemon/scheduled mode
lidarr-utils schedule
```

Global options include `--config`, `--dry-run`, and `--log-file`.

### Artist monitoring

Artist mode gives releases an Album > EP > Single priority. It matches tracks
by MusicBrainz recording ID, then track ID, then normalized title. When
`skip_fully_covered_releases` is enabled, an EP or single is skipped if its
tracks are already covered by retained releases at the same or a higher tier.
Albums remain eligible regardless of overlap.

Running with no artist arguments uses one bulk Lidarr album snapshot. Supplying
artist arguments fetches only those distinct artists' catalogues.

### Label monitoring

Label mode browses MusicBrainz releases for each label and collapses editions
to release groups, which is the album identity used by Lidarr. Duplicate
release groups across labels are merged before Lidarr work.

For artists already in Lidarr, the full artist catalogue participates in track
coverage. A non-label album can therefore suppress a redundant label EP or
single, but non-label releases never become label monitoring candidates. For a
missing artist, coverage uses only release groups discovered through the
configured labels. Set `monitor.labels.skip_fully_covered_releases: false` to
keep every otherwise eligible label release group.

Existing albums are matched only by MusicBrainz release-group MBID. Missing
albums use an exact Lidarr lookup:

- With `add_missing_artists: false`, releases belonging to absent artists are skipped.
- With `add_missing_artists: true`, only artists needed by selected release groups are added.
- A missing artist uses the selected Lidarr root folder's default quality
  profile, metadata profile, and tags. It starts unmonitored with
  `monitorNewItems: none` and no artist-wide search.
- If more than one accessible root exists, `root_folder` must select one.
  Trailing `/` and `\` separators are ignored; path case is preserved.

Positional label IDs replace configured IDs for that run. They must be valid
MusicBrainz UUIDs.

### Filtering and Various Artists

Each monitor mode has independent `official_only`,
`exclude_secondary_types`, `exclude_formats`, `exclude_va_releases`, and
coverage settings. Track-independent filters run before track hydration.

When enabled, `exclude_va_releases` excludes both release groups credited
directly to MusicBrainz's Various Artists entity and EPs/singles linked to a
Various Artists compilation through a MusicBrainz `single from` relationship.
MusicBrainz relationship results are cached for the run.

### Scheduling

`schedule` registers only modes whose nested `schedule.enabled` value is true.
Cron expressions and scheduled label IDs are validated before it starts.
`run_on_start` is independent for artist monitoring, label monitoring, and
dedupe.

Jobs share a serialized FIFO runner, so Lidarr mutations never overlap.
Repeated triggers for a job that is already queued or running are coalesced.
SIGINT/SIGTERM drops queued work and waits for the active job to finish.

## Safety

- `--dry-run` never adds an artist or album, changes monitoring state, submits
  a search, or writes state. Label summaries still include planned additions
  in their “would monitor/search” counts.
- Selected albums are batch-monitored and searched only after all label
  planning has completed.
- A local state file records albums monitored by the tool. If a user later
  unmonitors one, subsequent runs leave it unmonitored. Delete the state file
  to reset this protection.
- Label import adds albums unmonitored and disables automatic search; the final
  batch is the only monitoring/search mutation.
- `dedupe` can delete downloaded duplicate files. Preview it with `--dry-run`.

## v2 migration

v2 intentionally removes the v1 compatibility surface:

| Removed v1 form | v2 replacement |
|---|---|
| `monitor --artist-id 123` | `monitor artist 123` |
| `monitor --all` | `monitor artist` |
| `dedupe --cron ...` / `dedupe --run-once` | nested schedules plus `schedule`, or one-off `dedupe` |
| global `schedule.*` | `monitor.artists.schedule`, `monitor.labels.schedule`, and `dedupe.schedule` |
| flat `monitor.official_only` / `monitor.exclude_*` | mode-specific values under `monitor.artists` or `monitor.labels` |

## How dedupe works

The deduper scans downloaded singles and compares their tracks with albums and
EPs by MusicBrainz recording ID, track ID, and normalized title. Matches are
unmonitored, their track files are deleted, and they can optionally be added
to Lidarr's import exclusion list.

## Requirements

- Go 1.26.3 or newer when building from source
- Lidarr API access
- MusicBrainz access for label discovery and optional Various Artists filtering

## License

This project is licensed under the MIT License. The tool can modify or delete
library data; use dry-run mode first and keep backups.
