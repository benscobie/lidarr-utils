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

func TestLoadConfigDecodesIndependentSelectionPolicies(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, `
lidarr:
  url: http://lidarr:8686
  api_key: secret
monitor:
  artists:
    selection:
      include_secondary_types: false
      various_artists: exclude
      compilation_singles: include
      skip_fully_covered_releases: false
  labels:
    ids: [a5bfec28-ea8f-426d-ab23-e14aa692c9b5]
    missing_artists:
      enabled: true
      root_folder: /music
    selection:
      include_secondary_types: true
      exclude_formats: [Vinyl]
      various_artists: only
      compilation_singles: exclude
`))
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Monitor.Artists.Selection.VariousArtists != VariousArtistsExclude ||
		cfg.Monitor.Artists.Selection.SkipFullyCoveredReleases {
		t.Fatalf("unexpected artist policy: %#v", cfg.Monitor.Artists.Selection)
	}
	if cfg.Monitor.Labels.Selection.VariousArtists != VariousArtistsOnly ||
		cfg.Monitor.Labels.Selection.CompilationSingles != CompilationSinglesExclude {
		t.Fatalf("unexpected label policy: %#v", cfg.Monitor.Labels.Selection)
	}
	if !cfg.Monitor.Labels.MissingArtists.Enabled ||
		cfg.Monitor.Labels.MissingArtists.RootFolder != "/music" {
		t.Fatalf("unexpected missing-artist config: %#v", cfg.Monitor.Labels.MissingArtists)
	}
}

func TestLoadConfigDefaultsBothSelectionPolicies(t *testing.T) {
	cfg, err := LoadConfig(writeConfig(t, validMinimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	for name, policy := range map[string]ReleaseSelectionPolicy{
		"artists": cfg.Monitor.Artists.Selection,
		"labels":  cfg.Monitor.Labels.Selection,
	} {
		if !policy.IncludeSecondaryTypes ||
			policy.VariousArtists != VariousArtistsInclude ||
			policy.CompilationSingles != CompilationSinglesInclude ||
			!policy.SkipFullyCoveredReleases {
			t.Fatalf("%s defaults are wrong: %#v", name, policy)
		}
	}
}

func TestLoadConfigBindsSelectionEnvironment(t *testing.T) {
	t.Setenv("LIDARR_UTILS_MONITOR_ARTISTS_SELECTION_EXCLUDE_SECONDARY_TYPES", "Live,Compilation")
	t.Setenv("LIDARR_UTILS_MONITOR_LABELS_SELECTION_VARIOUS_ARTISTS", "only")
	cfg, err := LoadConfig(writeConfig(t, validMinimalYAML))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(
		cfg.Monitor.Artists.Selection.ExcludeSecondaryTypes,
		[]string{"Live", "Compilation"},
	) {
		t.Fatalf("unexpected artist environment policy: %#v", cfg.Monitor.Artists.Selection)
	}
	if cfg.Monitor.Labels.Selection.VariousArtists != VariousArtistsOnly {
		t.Fatalf("unexpected label environment policy: %#v", cfg.Monitor.Labels.Selection)
	}
}

func TestLoadConfigRejectsInvalidSelection(t *testing.T) {
	tests := []struct {
		name    string
		monitor string
		wantErr string
	}{
		{
			name: "invalid artist various artists policy",
			monitor: `
  artists:
    selection:
      various_artists: sometimes`,
			wantErr: `monitor.artists.selection.various_artists has invalid value "sometimes"; accepted values: include, exclude, only`,
		},
		{
			name: "invalid label compilation singles policy",
			monitor: `
  labels:
    selection:
      compilation_singles: sometimes`,
			wantErr: `monitor.labels.selection.compilation_singles has invalid value "sometimes"; accepted values: include, exclude`,
		},
		{
			name: "label secondary exclusions require secondary types",
			monitor: `
  labels:
    selection:
      include_secondary_types: false
      exclude_secondary_types: [Live]`,
			wantErr: "monitor.labels.selection.exclude_secondary_types must be empty when monitor.labels.selection.include_secondary_types is false",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := LoadConfig(writeConfig(t, `
lidarr:
  url: http://lidarr:8686
  api_key: secret
monitor:`+test.monitor+`
`))
			if err == nil || err.Error() != test.wantErr {
				t.Fatalf("error = %v, want %q", err, test.wantErr)
			}
		})
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
