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
    selection:
      include_secondary_types: true
      exclude_secondary_types: []
      exclude_formats: []
      various_artists: include          # include | exclude | only
      compilation_singles: include      # include | exclude
      skip_fully_covered_releases: true
    schedule:
      enabled: false
      cron: "0 2 * * *"
      run_on_start: false

  labels:
    ids:
      - "a5bfec28-ea8f-426d-ab23-e14aa692c9b5"
    missing_artists:
      enabled: false                    # Set true to add releases when their owning artist is absent
      root_folder: ""
    selection:
      include_secondary_types: true
      exclude_secondary_types: []
      exclude_formats: []
      various_artists: only
      compilation_singles: include
      skip_fully_covered_releases: true
    schedule:
      enabled: false
      cron: "0 */6 * * *"
      run_on_start: false
```

### Environment variables

The following environment variables are explicitly supported:

#### Lidarr and application

| Environment variable | Configuration key |
|---|---|
| `LIDARR_UTILS_LIDARR_URL` | `lidarr.url` |
| `LIDARR_UTILS_LIDARR_API_KEY` | `lidarr.api_key` |
| `LIDARR_UTILS_APP_DRY_RUN` | `app.dry_run` |
| `LIDARR_UTILS_APP_LOG_LEVEL` | `app.log_level` |
| `LIDARR_UTILS_APP_LOG_FILE` | `app.log_file` |
| `LIDARR_UTILS_APP_STATE_FILE` | `app.state_file` |

#### Dedupe

| Environment variable | Configuration key |
|---|---|
| `LIDARR_UTILS_DEDUPE_ADD_IMPORT_EXCLUSION` | `dedupe.add_import_exclusion` |
| `LIDARR_UTILS_DEDUPE_SCHEDULE_ENABLED` | `dedupe.schedule.enabled` |
| `LIDARR_UTILS_DEDUPE_SCHEDULE_CRON` | `dedupe.schedule.cron` |
| `LIDARR_UTILS_DEDUPE_SCHEDULE_RUN_ON_START` | `dedupe.schedule.run_on_start` |

#### Artist monitoring

| Environment variable | Configuration key |
|---|---|
| `LIDARR_UTILS_MONITOR_ARTISTS_SELECTION_INCLUDE_SECONDARY_TYPES` | `monitor.artists.selection.include_secondary_types` |
| `LIDARR_UTILS_MONITOR_ARTISTS_SELECTION_EXCLUDE_SECONDARY_TYPES` | `monitor.artists.selection.exclude_secondary_types` |
| `LIDARR_UTILS_MONITOR_ARTISTS_SELECTION_EXCLUDE_FORMATS` | `monitor.artists.selection.exclude_formats` |
| `LIDARR_UTILS_MONITOR_ARTISTS_SELECTION_VARIOUS_ARTISTS` | `monitor.artists.selection.various_artists` |
| `LIDARR_UTILS_MONITOR_ARTISTS_SELECTION_COMPILATION_SINGLES` | `monitor.artists.selection.compilation_singles` |
| `LIDARR_UTILS_MONITOR_ARTISTS_SELECTION_SKIP_FULLY_COVERED_RELEASES` | `monitor.artists.selection.skip_fully_covered_releases` |
| `LIDARR_UTILS_MONITOR_ARTISTS_SCHEDULE_ENABLED` | `monitor.artists.schedule.enabled` |
| `LIDARR_UTILS_MONITOR_ARTISTS_SCHEDULE_CRON` | `monitor.artists.schedule.cron` |
| `LIDARR_UTILS_MONITOR_ARTISTS_SCHEDULE_RUN_ON_START` | `monitor.artists.schedule.run_on_start` |

#### Label monitoring

| Environment variable | Configuration key |
|---|---|
| `LIDARR_UTILS_MONITOR_LABELS_MISSING_ARTISTS_ENABLED` | `monitor.labels.missing_artists.enabled` |
| `LIDARR_UTILS_MONITOR_LABELS_MISSING_ARTISTS_ROOT_FOLDER` | `monitor.labels.missing_artists.root_folder` |
| `LIDARR_UTILS_MONITOR_LABELS_SELECTION_INCLUDE_SECONDARY_TYPES` | `monitor.labels.selection.include_secondary_types` |
| `LIDARR_UTILS_MONITOR_LABELS_SELECTION_EXCLUDE_SECONDARY_TYPES` | `monitor.labels.selection.exclude_secondary_types` |
| `LIDARR_UTILS_MONITOR_LABELS_SELECTION_EXCLUDE_FORMATS` | `monitor.labels.selection.exclude_formats` |
| `LIDARR_UTILS_MONITOR_LABELS_SELECTION_VARIOUS_ARTISTS` | `monitor.labels.selection.various_artists` |
| `LIDARR_UTILS_MONITOR_LABELS_SELECTION_COMPILATION_SINGLES` | `monitor.labels.selection.compilation_singles` |
| `LIDARR_UTILS_MONITOR_LABELS_SELECTION_SKIP_FULLY_COVERED_RELEASES` | `monitor.labels.selection.skip_fully_covered_releases` |
| `LIDARR_UTILS_MONITOR_LABELS_SCHEDULE_ENABLED` | `monitor.labels.schedule.enabled` |
| `LIDARR_UTILS_MONITOR_LABELS_SCHEDULE_CRON` | `monitor.labels.schedule.cron` |
| `LIDARR_UTILS_MONITOR_LABELS_SCHEDULE_RUN_ON_START` | `monitor.labels.schedule.run_on_start` |

List values such as excluded secondary types and formats use comma-separated
values, for example:

```bash
export LIDARR_UTILS_MONITOR_ARTISTS_EXCLUDE_SECONDARY_TYPES="Live,Compilation"
export LIDARR_UTILS_MONITOR_LABELS_EXCLUDE_FORMATS="Vinyl,Cassette"
```

Label IDs do not have an environment-variable binding. Configure
`monitor.labels.ids` as a YAML list, or supply label MBIDs as positional
arguments to `monitor labels`.

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
`monitor.artists.selection.skip_fully_covered_releases` is enabled, an EP or single is skipped if its
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
configured labels. Set `monitor.labels.selection.skip_fully_covered_releases: false` to
keep every otherwise eligible label release group.

Existing albums are matched only by MusicBrainz release-group MBID. Missing
albums use an exact Lidarr lookup:

- With `missing_artists.enabled: false`, releases whose owning artists are absent from Lidarr are skipped.
- With `missing_artists.enabled: true`, only owners needed by selected release groups are added, including canonical Various Artists when it owns the release.
- A missing artist uses the selected Lidarr root folder's default quality
  profile, metadata profile, and tags. It starts unmonitored with
  `monitorNewItems: none` and no artist-wide search.
- If more than one accessible root exists, `missing_artists.root_folder` must select one.
  Trailing `/` and `\` separators are ignored; path case is preserved.

`missing_artists` shapes a Lidarr artist only after its releases pass selection.
A future Lidarr `Monitor New Albums` option is outside this workflow: albums
monitored by Lidarr through that option bypass this utility's selection policy.

Positional label IDs replace configured IDs for that run. They must be valid
MusicBrainz UUIDs.

### Release selection and Various Artists

Each monitor mode has an independent `selection` policy. Secondary-type,
format, Various Artists, compilation-single, and coverage settings do not
inherit between artist and label monitoring. Track-independent filtering runs
before track hydration.

`include_secondary_types: true` allows secondary types except those listed in
`exclude_secondary_types`. Set it to `false` to exclude every release group
with a secondary type; combining that with a non-empty exclusion list is a
configuration error. `various_artists` accepts `include`, `exclude`, or `only`.

Various Artists (VA) means the catalogue owner Lidarr assigns: MusicBrainz's
canonical Various Artists entity. An ordinary artist's EP or single that is
linked to a VA compilation is still not VA-owned. `compilation_singles: exclude`
is independent of `various_artists`: it checks only otherwise eligible EPs and
singles for MusicBrainz's explicit `single from` relationship to a compilation
directly credited to VA. Relationship results are cached for the run.

### Scheduling

`schedule` registers only modes whose nested `schedule.enabled` value is true.
Cron expressions and scheduled label IDs are validated before it starts.
`run_on_start` is independent for artist monitoring, label monitoring, and
dedupe.

Jobs share a serialized FIFO runner, so Lidarr mutations never overlap.
Repeated triggers for a job that is already queued or running are coalesced.
SIGINT/SIGTERM drops queued work and waits for the active job to finish.
Scheduled monitoring backfills currently eligible releases and adds newly
eligible releases on later runs; it never unmonitors releases that no longer
match the policy. Downloaded releases remain unchanged and can still provide
track coverage.

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

## Major-version configuration migration

This major version intentionally removes compatibility aliases. Move each
mode-specific setting into its mode's `selection` block:

| Removed configuration | Replacement |
|---|---|
| `official_only: false` | `selection.include_secondary_types: true` |
| `official_only: true` | `selection.include_secondary_types: false` |
| `exclude_secondary_types` | `selection.exclude_secondary_types` |
| `exclude_formats` | `selection.exclude_formats` |
| `skip_fully_covered_releases` | `selection.skip_fully_covered_releases` |
| `exclude_va_releases: false` | `selection.various_artists: include` and `selection.compilation_singles: include` |
| `exclude_va_releases: true` | `selection.various_artists: exclude` and `selection.compilation_singles: exclude` |
| `monitor.labels.add_missing_artists` | `monitor.labels.missing_artists.enabled` |
| `monitor.labels.root_folder` | `monitor.labels.missing_artists.root_folder` |

The prior major-version command migrations remain:

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
