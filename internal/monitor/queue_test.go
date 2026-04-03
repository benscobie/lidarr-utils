package monitor

import "testing"

func TestCountActiveSearches(t *testing.T) {
	commands := []commandInfo{
		{Name: "AlbumSearch", Status: "started"},
		{Name: "AlbumSearch", Status: "queued"},
		{Name: "AlbumSearch", Status: "completed"},
		{Name: "RefreshArtist", Status: "started"},
	}

	count := countActiveSearches(commands)
	if count != 2 {
		t.Errorf("expected 2 active searches, got %d", count)
	}
}
