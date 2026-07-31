package monitor

import (
	"slices"
	"strings"
	"testing"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/config"
)

func testSelectionPolicy(skipFullyCoveredReleases bool) config.ReleaseSelectionPolicy {
	return config.ReleaseSelectionPolicy{
		IncludeSecondaryTypes:    true,
		VariousArtists:           config.VariousArtistsInclude,
		CompilationSingles:       config.CompilationSinglesInclude,
		SkipFullyCoveredReleases: skipFullyCoveredReleases,
	}
}

func TestSelectAlbumsToMonitor_SyntheticCoverageReasonNamesSource(t *testing.T) {
	albums := []common.Album{
		{
			Title:          "Label EP",
			AlbumType:      "EP",
			ForeignAlbumID: "rg-ep",
			Tracks: []common.Track{{
				Title:              "Shared Track",
				ForeignRecordingID: "recording-1",
			}},
		},
		{
			Title:          "Label Single",
			AlbumType:      "Single",
			ForeignAlbumID: "rg-single",
			Tracks: []common.Track{{
				Title:              "Shared Track",
				ForeignRecordingID: "recording-1",
			}},
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{
		Policy: testSelectionPolicy(true),
	})

	if len(result.Skipped) != 1 {
		t.Fatalf("expected one covered release, got %#v", result.Skipped)
	}
	if !strings.Contains(result.Skipped[0].Reason, "ep 'Label EP'") {
		t.Fatalf("coverage reason did not name the source EP: %q", result.Skipped[0].Reason)
	}
}

func TestSelectAlbumsToMonitor_PrefersAlbumsOverEPs(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "Full Album", AlbumType: "Album",
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
				{ID: 2, Title: "Track 2", ForeignRecordingID: "rec-2"},
				{ID: 3, Title: "Track 3", ForeignRecordingID: "rec-3"},
			},
		},
		{
			ID: 2, Title: "My EP", AlbumType: "EP",
			Tracks: []common.Track{
				{ID: 4, Title: "Track 1", ForeignRecordingID: "rec-1"},
				{ID: 5, Title: "Track 2", ForeignRecordingID: "rec-2"},
			},
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: testSelectionPolicy(true)})

	if len(result.ToMonitor) != 1 {
		t.Fatalf("expected 1 album to monitor, got %d", len(result.ToMonitor))
	}
	if result.ToMonitor[0].ID != 1 {
		t.Error("expected the full album to be selected")
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(result.Skipped))
	}
}

func TestSelectAlbumsToMonitor_PrefersEPsOverSingles(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "My EP", AlbumType: "EP",
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
				{ID: 2, Title: "Track 2", ForeignRecordingID: "rec-2"},
			},
		},
		{
			ID: 2, Title: "My Single", AlbumType: "Single",
			Tracks: []common.Track{
				{ID: 3, Title: "Track 1", ForeignRecordingID: "rec-1"},
			},
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: testSelectionPolicy(true)})

	if len(result.ToMonitor) != 1 {
		t.Fatalf("expected 1 to monitor, got %d", len(result.ToMonitor))
	}
	if result.ToMonitor[0].ID != 1 {
		t.Error("expected EP to be selected")
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(result.Skipped))
	}
}

func TestSelectAlbumsToMonitor_EPWithTracksAcrossMultipleAlbums(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "Album A", AlbumType: "Album",
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
			},
		},
		{
			ID: 2, Title: "Album B", AlbumType: "Album",
			Tracks: []common.Track{
				{ID: 2, Title: "Track 2", ForeignRecordingID: "rec-2"},
			},
		},
		{
			ID: 3, Title: "My EP", AlbumType: "EP",
			Tracks: []common.Track{
				{ID: 3, Title: "Track 1", ForeignRecordingID: "rec-1"},
				{ID: 4, Title: "Track 2", ForeignRecordingID: "rec-2"},
			},
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: testSelectionPolicy(true)})

	if len(result.ToMonitor) != 2 {
		t.Fatalf("expected 2 to monitor (both albums), got %d", len(result.ToMonitor))
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("expected 1 skipped (EP), got %d", len(result.Skipped))
	}
}

func TestSelectAlbumsToMonitor_EPWithUniqueTracksIsMonitored(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "Album A", AlbumType: "Album",
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
			},
		},
		{
			ID: 2, Title: "My EP", AlbumType: "EP",
			Tracks: []common.Track{
				{ID: 2, Title: "Track 1", ForeignRecordingID: "rec-1"},
				{ID: 3, Title: "Unique Track", ForeignRecordingID: "rec-unique"},
			},
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: testSelectionPolicy(true)})

	if len(result.ToMonitor) != 2 {
		t.Fatalf("expected 2 to monitor (album + EP), got %d", len(result.ToMonitor))
	}
}

func TestSelectAlbumsToMonitor_SkipsAlreadyMonitored(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "Already Got This", AlbumType: "Album", Monitored: true,
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
			},
		},
		{
			ID: 2, Title: "Redundant Single", AlbumType: "Single",
			Tracks: []common.Track{
				{ID: 2, Title: "Track 1", ForeignRecordingID: "rec-1"},
			},
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: testSelectionPolicy(true)})

	if len(result.ToMonitor) != 0 {
		t.Fatalf("expected 0 to monitor (album already monitored), got %d", len(result.ToMonitor))
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(result.Skipped))
	}
}

func TestSelectAlbumsToMonitor_OfficialOnly(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "Studio Album", AlbumType: "Album",
			Tracks: []common.Track{{ID: 1, Title: "T1", ForeignRecordingID: "r1"}},
		},
		{
			ID: 2, Title: "Live Album", AlbumType: "Album",
			SecondaryTypes: []string{"Live"},
			Tracks:         []common.Track{{ID: 2, Title: "T2", ForeignRecordingID: "r2"}},
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: config.ReleaseSelectionPolicy{VariousArtists: config.VariousArtistsInclude, CompilationSingles: config.CompilationSinglesInclude, SkipFullyCoveredReleases: true}})

	if len(result.ToMonitor) != 1 {
		t.Fatalf("expected 1 to monitor, got %d", len(result.ToMonitor))
	}
	if result.ToMonitor[0].ID != 1 {
		t.Error("expected studio album to be selected")
	}
	if len(result.Excluded) != 1 {
		t.Fatalf("expected 1 excluded, got %d", len(result.Excluded))
	}
}

func TestSelectAlbumsToMonitor_AlbumWithNoTracks(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "No Tracks Album", AlbumType: "Album",
			Tracks: nil,
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: testSelectionPolicy(true)})

	if len(result.ToMonitor) != 1 {
		t.Fatalf("expected 1 to monitor (no tracks = can't prove redundancy), got %d", len(result.ToMonitor))
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning for missing tracks, got %d", len(result.Warnings))
	}
}

func TestSelectAlbumsToMonitor_ExcludeDownloadedAlbum(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "Downloaded Album", AlbumType: "Album", HasFiles: true,
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
				{ID: 2, Title: "Track 2", ForeignRecordingID: "rec-2"},
			},
		},
		{
			ID: 2, Title: "New Album", AlbumType: "Album",
			Tracks: []common.Track{
				{ID: 3, Title: "Track 3", ForeignRecordingID: "rec-3"},
			},
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: testSelectionPolicy(true)})

	if len(result.ToMonitor) != 1 {
		t.Fatalf("expected 1 to monitor (downloaded album excluded), got %d", len(result.ToMonitor))
	}
	if result.ToMonitor[0].ID != 2 {
		t.Error("expected only the new album to be selected")
	}
}

func TestSelectAlbumsToMonitor_ExcludeDownloadedEP(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "Downloaded EP", AlbumType: "EP", HasFiles: true,
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
			},
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: testSelectionPolicy(true)})

	if len(result.ToMonitor) != 0 {
		t.Fatalf("expected 0 to monitor (downloaded EP excluded), got %d", len(result.ToMonitor))
	}
}

func TestSelectAlbumsToMonitor_ExcludeDownloadedSingle(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "Downloaded Single", AlbumType: "Single", HasFiles: true,
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
			},
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: testSelectionPolicy(true)})

	if len(result.ToMonitor) != 0 {
		t.Fatalf("expected 0 to monitor (downloaded single excluded), got %d", len(result.ToMonitor))
	}
}

func TestSelectAlbumsToMonitor_DownloadedAlbumTracksStillDedup(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "Downloaded Album", AlbumType: "Album", HasFiles: true,
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
				{ID: 2, Title: "Track 2", ForeignRecordingID: "rec-2"},
			},
		},
		{
			ID: 2, Title: "Redundant Single", AlbumType: "Single",
			Tracks: []common.Track{
				{ID: 3, Title: "Track 1", ForeignRecordingID: "rec-1"},
			},
		},
	}

	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: testSelectionPolicy(true)})

	// Downloaded album excluded from monitoring, but its tracks should still
	// cause the single to be skipped (tracks already covered).
	if len(result.ToMonitor) != 0 {
		t.Fatalf("expected 0 to monitor, got %d", len(result.ToMonitor))
	}
	if len(result.Skipped) != 1 {
		t.Fatalf("expected 1 skipped (single covered by downloaded album), got %d", len(result.Skipped))
	}
}

func TestSelectAlbumsToMonitor_ExcludeVinylOnly(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "Vinyl Only Album", AlbumType: "Album",
			Releases: []common.Release{
				{ID: 1, Format: "Vinyl"},
				{ID: 2, Format: "12\" Vinyl"},
			},
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
			},
		},
		{
			ID: 2, Title: "CD Album", AlbumType: "Album",
			Releases: []common.Release{
				{ID: 3, Format: "CD"},
				{ID: 4, Format: "Digital Media"},
			},
			Tracks: []common.Track{
				{ID: 2, Title: "Track 2", ForeignRecordingID: "rec-2"},
			},
		},
	}

	policy := testSelectionPolicy(true)
	policy.ExcludeFormats = []string{"Vinyl"}
	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: policy})

	if len(result.ToMonitor) != 1 {
		t.Fatalf("expected 1 to monitor, got %d", len(result.ToMonitor))
	}
	if result.ToMonitor[0].ID != 2 {
		t.Error("expected CD Album to be selected")
	}
	if len(result.Excluded) != 1 {
		t.Fatalf("expected 1 excluded, got %d", len(result.Excluded))
	}
	if result.Excluded[0].Album.ID != 1 {
		t.Error("expected Vinyl Only Album to be excluded")
	}
}

func TestSelectAlbumsToMonitor_VinylAndCDReleasePasses(t *testing.T) {
	albums := []common.Album{
		{
			ID: 1, Title: "Has CD Release Too", AlbumType: "Album",
			Releases: []common.Release{
				{ID: 1, Format: "Vinyl"},
				{ID: 2, Format: "CD"},
			},
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
			},
		},
	}

	policy := testSelectionPolicy(true)
	policy.ExcludeFormats = []string{"Vinyl"}
	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: policy})

	if len(result.ToMonitor) != 1 {
		t.Fatalf("expected 1 to monitor (has CD release), got %d", len(result.ToMonitor))
	}
	if len(result.Excluded) != 0 {
		t.Fatalf("expected 0 excluded, got %d", len(result.Excluded))
	}
}

func TestSelectAlbumsToMonitor_FormatFilterBeforeTrackCoverage(t *testing.T) {
	// Vinyl-only album should be excluded even if it has unique tracks
	albums := []common.Album{
		{
			ID: 1, Title: "CD Album", AlbumType: "Album",
			Releases: []common.Release{{ID: 1, Format: "CD"}},
			Tracks: []common.Track{
				{ID: 1, Title: "Track 1", ForeignRecordingID: "rec-1"},
			},
		},
		{
			ID: 2, Title: "Vinyl Only EP", AlbumType: "EP",
			Releases: []common.Release{{ID: 2, Format: "Vinyl"}},
			Tracks: []common.Track{
				{ID: 2, Title: "Unique Vinyl Track", ForeignRecordingID: "rec-unique"},
			},
		},
	}

	policy := testSelectionPolicy(true)
	policy.ExcludeFormats = []string{"Vinyl"}
	result := SelectAlbumsToMonitor(albums, SelectionOptions{Policy: policy})

	if len(result.ToMonitor) != 1 {
		t.Fatalf("expected 1 to monitor, got %d", len(result.ToMonitor))
	}
	if len(result.Excluded) != 1 {
		t.Fatalf("expected 1 excluded, got %d", len(result.Excluded))
	}
	// The EP should be excluded by format, NOT appear in Skipped
	if len(result.Skipped) != 0 {
		t.Fatalf("expected 0 skipped (format filter runs before coverage), got %d", len(result.Skipped))
	}
}

func TestSelectAlbumsToMonitor_NonCandidateAlbumProvidesCoverage(t *testing.T) {
	catalog := []common.Album{
		{
			ID: 1, ForeignAlbumID: "non-label-album",
			Title: "Non-label Album", AlbumType: "Album",
			Tracks: []common.Track{{ForeignRecordingID: "recording-1"}},
		},
		{
			ForeignAlbumID: "label-single",
			Title:          "Label Single", AlbumType: "Single",
			Tracks: []common.Track{{ForeignRecordingID: "recording-1"}},
		},
	}
	result := SelectAlbumsToMonitor(catalog, SelectionOptions{
		Policy:                 testSelectionPolicy(true),
		CandidateReleaseGroups: map[string]struct{}{"label-single": {}},
	})
	if len(result.ToMonitor) != 0 || len(result.Skipped) != 1 {
		t.Fatalf("expected only label single to be skipped: %#v", result)
	}
}

func TestSelectAlbumsToMonitor_CoverageDisabledKeepsRedundantCandidate(t *testing.T) {
	catalog := []common.Album{
		{
			ForeignAlbumID: "non-label-album",
			Title:          "Non-label Album", AlbumType: "Album",
			Tracks: []common.Track{{ForeignRecordingID: "recording-1"}},
		},
		{
			ForeignAlbumID: "label-single",
			Title:          "Label Single", AlbumType: "Single",
			Tracks: []common.Track{{ForeignRecordingID: "recording-1"}},
		},
	}
	result := SelectAlbumsToMonitor(catalog, SelectionOptions{
		Policy:                 testSelectionPolicy(false),
		CandidateReleaseGroups: map[string]struct{}{"label-single": {}},
	})
	var got []string
	for _, album := range result.ToMonitor {
		got = append(got, album.ForeignAlbumID)
	}
	if !slices.Equal(got, []string{"label-single"}) {
		t.Fatalf("unexpected candidates: %v", got)
	}
}
