# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

lidarr-utils is a Go CLI tool for managing Lidarr music libraries. Two commands:
- **dedupe**: Removes duplicate singles already present in albums/EPs
- **monitor**: Selects optimal releases to monitor using track coverage analysis (Albums > EPs > Singles priority)

Track matching uses MusicBrainz Recording IDs (preferred), Track IDs, then normalized title comparison as fallback.

## Build & Test Commands

```bash
go build -o lidarr-utils .         # Build binary
go test ./...                       # Run all tests
go test ./internal/common -v        # Run tests in a specific package
go test ./internal/monitor -run TestSelectAlbumsToMonitor  # Run a single test
```

## Architecture

Entry point: `main.go` -> `cmd.Execute()`

### Package Structure

- `cmd/` - Cobra CLI commands (root, dedupe, monitor). Config is loaded via Viper in root.go.
- `internal/config/` - Config struct definitions. Loaded from `config.yaml` or `LIDARR_UTILS_*` env vars.
- `internal/lidarr/` - HTTP client wrapping Lidarr API v1. Contains both Lidarr-specific types (with JSON tags) and API methods.
- `internal/common/` - Domain types (`Album`, `Track`, `Release`) and pure-function business logic for album classification and track matching.
- `internal/dedupe/` - Deduplication: finds singles whose tracks already exist in albums/EPs, then unmonitors and deletes.
- `internal/monitor/` - Album selection pipeline and orchestration. `monitor.go` has the `Monitor` struct that drives the pipeline; `pipeline.go` has the pure `SelectAlbumsToMonitor` function.

### Key Design Patterns

- **Two type systems**: `internal/lidarr/` has API-facing types with JSON tags; `internal/common/` has domain types without. The `monitor.processArtist()` method converts between them.
- **Pure selection logic**: `SelectAlbumsToMonitor` in `internal/monitor/pipeline.go` is a pure function (no API calls) that takes albums and filter config, returns `SelectionResult` with `ToMonitor`, `Skipped`, and `Excluded` lists.
- **Album classification helpers**: `common.IsAlbum()`, `common.IsEP()`, `common.IsSingle()` classify by `AlbumType` field. `common.ShouldExcludeByFormat()` and `common.ShouldExcludeBySecondaryType()` handle filtering.
- **Batch API operations**: Monitor and search use batch Lidarr endpoints (`PUT /api/v1/album/monitor`, `POST /api/v1/command`) rather than per-album calls.

## Configuration

Required: `lidarr.url` and `lidarr.api_key` (via `config.yaml` or env vars `LIDARR_UTILS_LIDARR_URL`, `LIDARR_UTILS_LIDARR_API_KEY`).

See `config.example.yaml` for all options.
