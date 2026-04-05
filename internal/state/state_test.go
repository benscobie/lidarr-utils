package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoad_FileNotFound_ReturnsEmptyState(t *testing.T) {
	s, err := Load("/tmp/nonexistent-state-file-12345.json")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil state")
	}
	if len(s.MonitoredAlbums) != 0 {
		t.Errorf("expected empty MonitoredAlbums, got %d entries", len(s.MonitoredAlbums))
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	now := time.Now().Truncate(time.Second)
	s.MonitoredAlbums[42] = MonitoredAlbum{
		ArtistName:  "Radiohead",
		AlbumTitle:  "OK Computer",
		MonitoredAt: now,
	}

	if err := s.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load after Save failed: %v", err)
	}

	entry, ok := loaded.MonitoredAlbums[42]
	if !ok {
		t.Fatal("expected album ID 42 to be present after round-trip")
	}
	if entry.ArtistName != "Radiohead" {
		t.Errorf("expected ArtistName 'Radiohead', got %q", entry.ArtistName)
	}
	if entry.AlbumTitle != "OK Computer" {
		t.Errorf("expected AlbumTitle 'OK Computer', got %q", entry.AlbumTitle)
	}
	if !entry.MonitoredAt.Equal(now) {
		t.Errorf("expected MonitoredAt %v, got %v", now, entry.MonitoredAt)
	}
}

func TestLoad_CorruptFile_ReturnsEmptyState(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	if err := os.WriteFile(path, []byte("not valid json{{{"), 0644); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}

	s, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error for corrupt file, got %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil state")
	}
	if len(s.MonitoredAlbums) != 0 {
		t.Errorf("expected empty MonitoredAlbums, got %d entries", len(s.MonitoredAlbums))
	}
}

func TestRecordAlbum(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	before := time.Now()
	s.RecordAlbum(99, "Tool", "Lateralus")
	after := time.Now()

	entry, ok := s.MonitoredAlbums[99]
	if !ok {
		t.Fatal("expected album ID 99 to be present after RecordAlbum")
	}
	if entry.ArtistName != "Tool" {
		t.Errorf("expected ArtistName 'Tool', got %q", entry.ArtistName)
	}
	if entry.AlbumTitle != "Lateralus" {
		t.Errorf("expected AlbumTitle 'Lateralus', got %q", entry.AlbumTitle)
	}
	if entry.MonitoredAt.IsZero() {
		t.Error("expected non-zero MonitoredAt")
	}
	if entry.MonitoredAt.Before(before) || entry.MonitoredAt.After(after) {
		t.Errorf("expected MonitoredAt between %v and %v, got %v", before, after, entry.MonitoredAt)
	}
}

func TestWasPreviouslyMonitored(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	s, err := Load(path)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	s.RecordAlbum(10, "Deftones", "White Pony")

	if !s.WasPreviouslyMonitored(10) {
		t.Error("expected WasPreviouslyMonitored(10) to return true")
	}
	if s.WasPreviouslyMonitored(999) {
		t.Error("expected WasPreviouslyMonitored(999) to return false")
	}
}
