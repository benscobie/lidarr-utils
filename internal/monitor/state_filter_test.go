package monitor

import (
	"testing"

	"github.com/lidarr-utils/internal/common"
	"github.com/lidarr-utils/internal/state"
)

func TestFilterUserUnmonitored_SkipsPreviouslyMonitoredAlbums(t *testing.T) {
	s := &state.State{MonitoredAlbums: map[int]state.MonitoredAlbum{
		1: {ArtistName: "Artist", AlbumTitle: "Album 1"},
		2: {ArtistName: "Artist", AlbumTitle: "Album 2"},
	}}

	candidates := []common.Album{
		{ID: 1, Title: "Album 1", ArtistName: "Artist", Monitored: false},  // in state + unmonitored = skip
		{ID: 2, Title: "Album 2", ArtistName: "Artist", Monitored: true},   // in state + still monitored = pass through
		{ID: 3, Title: "Album 3", ArtistName: "Artist", Monitored: false},  // not in state = keep
	}

	kept, skipped := filterUserUnmonitored(candidates, s)

	if len(kept) != 2 {
		t.Fatalf("expected 2 kept, got %d", len(kept))
	}
	if kept[0].ID != 2 || kept[1].ID != 3 {
		t.Errorf("expected albums 2 and 3 kept, got IDs %d and %d", kept[0].ID, kept[1].ID)
	}
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(skipped))
	}
	if skipped[0].ID != 1 {
		t.Errorf("expected album 1 skipped, got %d", skipped[0].ID)
	}
}

func TestFilterUserUnmonitored_NilState_KeepsAll(t *testing.T) {
	candidates := []common.Album{
		{ID: 1, Title: "Album 1", Monitored: false},
		{ID: 2, Title: "Album 2", Monitored: false},
	}

	kept, skipped := filterUserUnmonitored(candidates, nil)

	if len(kept) != 2 {
		t.Fatalf("expected 2 kept, got %d", len(kept))
	}
	if len(skipped) != 0 {
		t.Fatalf("expected 0 skipped, got %d", len(skipped))
	}
}
