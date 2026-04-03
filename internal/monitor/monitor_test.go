package monitor

import (
	"testing"

	"github.com/lidarr-utils/internal/common"
)

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

	result := SelectAlbumsToMonitor(albums, false, nil)

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

	result := SelectAlbumsToMonitor(albums, false, nil)

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

	result := SelectAlbumsToMonitor(albums, false, nil)

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

	result := SelectAlbumsToMonitor(albums, false, nil)

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

	result := SelectAlbumsToMonitor(albums, false, nil)

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
			Tracks: []common.Track{{ID: 2, Title: "T2", ForeignRecordingID: "r2"}},
		},
	}

	result := SelectAlbumsToMonitor(albums, true, nil)

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

	result := SelectAlbumsToMonitor(albums, false, nil)

	if len(result.ToMonitor) != 1 {
		t.Fatalf("expected 1 to monitor (no tracks = can't prove redundancy), got %d", len(result.ToMonitor))
	}
	if len(result.Warnings) != 1 {
		t.Fatalf("expected 1 warning for missing tracks, got %d", len(result.Warnings))
	}
}
