package monitor

import (
	"testing"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/config"
)

func TestReleasePolicyExclusionReason(t *testing.T) {
	tests := []struct {
		name   string
		album  common.Album
		policy config.ReleaseSelectionPolicy
		want   string
	}{
		{
			name:  "secondary types disabled",
			album: common.Album{SecondaryTypes: []string{"Compilation"}},
			policy: config.ReleaseSelectionPolicy{
				VariousArtists:     config.VariousArtistsInclude,
				CompilationSingles: config.CompilationSinglesInclude,
			},
			want: `has secondary type "Compilation" while secondary types are disabled`,
		},
		{
			name:  "named secondary type exclusion is case insensitive",
			album: common.Album{SecondaryTypes: []string{"Live"}},
			policy: config.ReleaseSelectionPolicy{
				IncludeSecondaryTypes: true,
				ExcludeSecondaryTypes: []string{"live"},
				VariousArtists:        config.VariousArtistsInclude,
				CompilationSingles:    config.CompilationSinglesInclude,
			},
			want: `excluded secondary type "Live"`,
		},
		{
			name:  "all formats excluded",
			album: common.Album{Releases: []common.Release{{Format: "Vinyl"}}},
			policy: config.ReleaseSelectionPolicy{
				IncludeSecondaryTypes: true,
				ExcludeFormats:        []string{"Vinyl"},
				VariousArtists:        config.VariousArtistsInclude,
				CompilationSingles:    config.CompilationSinglesInclude,
			},
			want: "no acceptable format (available: Vinyl)",
		},
		{
			name:  "various artists excluded",
			album: common.Album{IsVariousArtists: true},
			policy: config.ReleaseSelectionPolicy{
				IncludeSecondaryTypes: true,
				VariousArtists:        config.VariousArtistsExclude,
				CompilationSingles:    config.CompilationSinglesInclude,
			},
			want: "owned by Various Artists in Lidarr",
		},
		{
			name:  "only various artists",
			album: common.Album{},
			policy: config.ReleaseSelectionPolicy{
				IncludeSecondaryTypes: true,
				VariousArtists:        config.VariousArtistsOnly,
				CompilationSingles:    config.CompilationSinglesInclude,
			},
			want: "not owned by Various Artists in Lidarr",
		},
		{
			name:  "compilation single excluded",
			album: common.Album{CompilationSingleSource: "Compilation Vol. 4"},
			policy: config.ReleaseSelectionPolicy{
				IncludeSecondaryTypes: true,
				VariousArtists:        config.VariousArtistsInclude,
				CompilationSingles:    config.CompilationSinglesExclude,
			},
			want: `compilation single from "Compilation Vol. 4"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := releasePolicyExclusionReason(test.album, test.policy); got != test.want {
				t.Fatalf("reason = %q, want %q", got, test.want)
			}
		})
	}
}

func TestVariousArtistsAndCompilationSinglesAreIndependent(t *testing.T) {
	album := common.Album{CompilationSingleSource: "Compilation Vol. 4"}
	policy := config.ReleaseSelectionPolicy{
		IncludeSecondaryTypes: true,
		VariousArtists:        config.VariousArtistsExclude,
		CompilationSingles:    config.CompilationSinglesInclude,
	}
	if got := releasePolicyExclusionReason(album, policy); got != "" {
		t.Fatalf("ordinary compilation single excluded: %q", got)
	}

	policy.CompilationSingles = config.CompilationSinglesExclude
	if got := releasePolicyExclusionReason(album, policy); got != `compilation single from "Compilation Vol. 4"` {
		t.Fatalf("reason = %q", got)
	}
}
