package config

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

const validMinimalYAML = `
lidarr:
  url: http://lidarr:8686
  api_key: secret
`

func TestLoadConfigDecodesMonitorModes(t *testing.T) {
	path := writeConfig(t, `
lidarr:
  url: http://lidarr:8686
  api_key: secret
monitor:
  artists:
    skip_fully_covered_releases: false
    exclude_formats: [Vinyl]
  labels:
    ids: [a5bfec28-ea8f-426d-ab23-e14aa692c9b5]
    add_missing_artists: true
    root_folder: /music
    schedule:
      enabled: true
      cron: "0 */6 * * *"
dedupe:
  schedule:
    enabled: true
    cron: "0 2 * * *"
`)

	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Monitor.Artists.SkipFullyCoveredReleases {
		t.Fatal("explicit false must be retained")
	}
	if !cfg.Monitor.Labels.AddMissingArtists || cfg.Monitor.Labels.RootFolder != "/music" {
		t.Fatalf("unexpected labels config: %#v", cfg.Monitor.Labels)
	}
	if !cfg.Dedupe.Schedule.Enabled {
		t.Fatal("expected dedupe schedule enabled")
	}
}

func TestLoadConfigDefaultsCoverageSelectionOn(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, validMinimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Monitor.Artists.SkipFullyCoveredReleases ||
		!cfg.Monitor.Labels.SkipFullyCoveredReleases {
		t.Fatal("coverage selection must default on for both monitor modes")
	}
}

func TestLoadConfigBindsNestedMonitorEnvironment(t *testing.T) {
	t.Setenv("LIDARR_UTILS_MONITOR_LABELS_EXCLUDE_VA_RELEASES", "true")
	cfg, err := LoadConfig(writeConfig(t, validMinimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Monitor.Labels.ExcludeVAReleases {
		t.Fatal("nested monitor environment binding was not applied")
	}
}

func TestLoadConfigNormalizesAndDeduplicatesLabelIDs(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, `
lidarr:
  url: http://lidarr:8686
  api_key: secret
monitor:
  labels:
    ids:
      - A5BFEC28-EA8F-426D-AB23-E14AA692C9B5
      - a5bfec28-ea8f-426d-ab23-e14aa692c9b5
`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a5bfec28-ea8f-426d-ab23-e14aa692c9b5"}
	if !slices.Equal(cfg.Monitor.Labels.IDs, want) {
		t.Fatalf("unexpected normalized IDs: %v", cfg.Monitor.Labels.IDs)
	}
}

func TestNormalizeLabelIDsRejectsMalformedID(t *testing.T) {
	if _, err := NormalizeLabelIDs([]string{"not-a-uuid"}); err == nil {
		t.Fatal("expected malformed UUID error")
	}
}

func writeConfig(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
