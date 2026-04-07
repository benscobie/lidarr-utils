package monitor

import (
	"path/filepath"
	"testing"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/state"
)

func TestStateFilterIntegration_SkipsUserUnmonitored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Simulate first run: tool monitors albums 1 and 2
	st, err := state.Load(path)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	st.RecordAlbum(1, "Artist A", "Album 1")
	st.RecordAlbum(2, "Artist A", "Album 2")
	if err := st.Save(); err != nil {
		t.Fatalf("failed to save state: %v", err)
	}

	// Simulate second run: user has unmonitored album 1 in Lidarr
	st2, err := state.Load(path)
	if err != nil {
		t.Fatalf("failed to load state: %v", err)
	}
	candidates := []common.Album{
		{ID: 1, Title: "Album 1", ArtistName: "Artist A", Monitored: false}, // user unmonitored
		{ID: 2, Title: "Album 2", ArtistName: "Artist A", Monitored: true},  // still monitored
		{ID: 3, Title: "Album 3", ArtistName: "Artist A", Monitored: false}, // new album
	}

	kept, skipped := filterUserUnmonitored(candidates, st2)

	// Album 1: in state + unmonitored = skipped
	if len(skipped) != 1 {
		t.Fatalf("expected 1 skipped, got %d", len(skipped))
	}
	if skipped[0].ID != 1 {
		t.Errorf("expected album 1 skipped, got ID %d", skipped[0].ID)
	}

	// Albums 2 and 3: kept
	if len(kept) != 2 {
		t.Fatalf("expected 2 kept, got %d", len(kept))
	}
	if kept[0].ID != 2 || kept[1].ID != 3 {
		t.Errorf("expected albums 2 and 3 kept, got IDs %d and %d", kept[0].ID, kept[1].ID)
	}
}
