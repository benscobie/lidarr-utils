# CLAUDE.md

Guidance for working in this repository.

## Project overview

`lidarr-utils` is a Go CLI with four workflows:

- `monitor artist [refs...]`: select useful releases for specific artists, or
  all Lidarr artists when no refs are supplied.
- `monitor labels [label-mbids...]`: discover MusicBrainz label release groups,
  plan against full artist catalogues, import missing albums/artists when
  configured, then batch monitor/search.
- `dedupe`: remove downloaded singles already represented by albums or EPs.
- `schedule`: run enabled artist, label, and dedupe jobs through one serialized queue.

Track matching prefers MusicBrainz recording IDs, then track IDs, then
normalized titles. Release priority is Album > EP > Single.

## Build and test

```bash
go build -o lidarr-utils .
go test ./...
go vet ./...
go test ./internal/monitor -run TestPlanLabels -v
go test ./internal/scheduler -race -v
```

Tests should cover this repository's decisions, mappings, caching, payload
construction, and orchestration. Do not assert third-party controller/parser
internals or depend on live Lidarr/MusicBrainz services.

## Architecture

Entry point: `main.go` → `cmd.Execute()`.

- `cmd/`: Cobra commands and reusable one-shot job functions. `monitor` is a
  parent for `artist` and `labels`; `schedule` invokes the same job functions.
- `internal/config/`: private Viper instance, nested v2 configuration,
  defaults, environment bindings, and label UUID normalization.
- `internal/lidarr/`: API-facing JSON resources and Lidarr API methods.
- `internal/musicbrainz/`: rate-limited/retrying client, label release paging
  and release-group merging, plus VA relationship lookup.
- `internal/common/`: API-independent domain types, album classification,
  format/secondary filters, and track matching.
- `internal/monitor/`: run-scoped catalogue/VA caches, pure release selection,
  artist orchestration, read-only label planning, safe import payloads, state
  protection, and shared batch application.
- `internal/scheduler/`: serialized FIFO runner with duplicate coalescing and
  graceful shutdown.
- `internal/dedupe/`: duplicate detection and cleanup.
- `internal/state/`: JSON record of albums previously monitored by the tool.

## Important invariants

- API resources with JSON tags stay in `internal/lidarr`; domain types stay in
  `internal/common`.
- Caches are scoped to one command/scheduled job. Successful empty reads are
  cached; errors are not.
- All-artist mode uses one bulk album snapshot. Specific-artist and label modes
  fetch only relevant distinct artist catalogues.
- Format and secondary-type filters run before track hydration.
- Existing label release groups bypass album lookup; missing groups use one
  cached exact release-group MBID lookup.
- Label candidate membership and coverage providers are separate: non-label
  albums may provide coverage but must never become mutation candidates.
- `PlanLabels` is read-only. Album/artist creation occurs only after planning.
- New albums are added unmonitored with automatic search disabled. Shared
  `applyAlbums` owns state filtering, dry-run behavior, ID deduplication,
  batch monitoring, state persistence, and batch search.
- Dry run performs no Lidarr mutation and no state write.
- Scheduler jobs never overlap; a same-name queued/running job is coalesced.

## Configuration

Required values are `lidarr.url` and `lidarr.api_key`. Monitor filters and
schedules are mode-specific under `monitor.artists` and `monitor.labels`;
dedupe has its own schedule. `schedule` is the only daemon mode.

See `config.example.yaml` and `README.md` for the complete v2 configuration and
migration notes.
