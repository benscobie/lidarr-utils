package cmd

import (
	"fmt"
	"testing"

	"github.com/benscobie/lidarr-utils/internal/lidarr"
)

// mockArtistResolver implements the interface needed by resolveArtistIDs
// so we can test without a real Lidarr server.
type mockArtistResolver struct {
	artists []lidarr.Artist
	err     error
}

func (m *mockArtistResolver) GetArtists() ([]lidarr.Artist, error) {
	return m.artists, m.err
}

func TestResolveArtistIDs_SingleNumeric(t *testing.T) {
	resolver := &mockArtistResolver{}
	ids, err := resolveArtistIDs(resolver, []string{"42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != 42 {
		t.Errorf("expected [42], got %v", ids)
	}
}

func TestResolveArtistIDs_MultipleNumeric(t *testing.T) {
	resolver := &mockArtistResolver{}
	ids, err := resolveArtistIDs(resolver, []string{"1", "2", "3"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []int{1, 2, 3}
	if len(ids) != len(expected) {
		t.Fatalf("expected %v, got %v", expected, ids)
	}
	for i, id := range ids {
		if id != expected[i] {
			t.Errorf("ids[%d] = %d, want %d", i, id, expected[i])
		}
	}
}

func TestResolveArtistIDs_SingleForeignID(t *testing.T) {
	resolver := &mockArtistResolver{
		artists: []lidarr.Artist{
			{ID: 10, ForeignID: "mbid-abc-123", ArtistName: "Artist A"},
			{ID: 20, ForeignID: "mbid-def-456", ArtistName: "Artist B"},
		},
	}
	ids, err := resolveArtistIDs(resolver, []string{"mbid-abc-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 1 || ids[0] != 10 {
		t.Errorf("expected [10], got %v", ids)
	}
}

func TestResolveArtistIDs_MultipleForeignIDs(t *testing.T) {
	resolver := &mockArtistResolver{
		artists: []lidarr.Artist{
			{ID: 10, ForeignID: "mbid-abc-123", ArtistName: "Artist A"},
			{ID: 20, ForeignID: "mbid-def-456", ArtistName: "Artist B"},
		},
	}
	ids, err := resolveArtistIDs(resolver, []string{"mbid-abc-123", "mbid-def-456"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %v", ids)
	}
	if ids[0] != 10 || ids[1] != 20 {
		t.Errorf("expected [10, 20], got %v", ids)
	}
}

func TestResolveArtistIDs_MixedNumericAndForeign(t *testing.T) {
	resolver := &mockArtistResolver{
		artists: []lidarr.Artist{
			{ID: 10, ForeignID: "mbid-abc-123", ArtistName: "Artist A"},
		},
	}
	ids, err := resolveArtistIDs(resolver, []string{"42", "mbid-abc-123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %v", ids)
	}
	if ids[0] != 42 || ids[1] != 10 {
		t.Errorf("expected [42, 10], got %v", ids)
	}
}

func TestResolveArtistIDs_ForeignIDNotFound(t *testing.T) {
	resolver := &mockArtistResolver{
		artists: []lidarr.Artist{
			{ID: 10, ForeignID: "mbid-abc-123", ArtistName: "Artist A"},
		},
	}
	_, err := resolveArtistIDs(resolver, []string{"mbid-not-found"})
	if err == nil {
		t.Fatal("expected error for unknown foreign ID")
	}
}

func TestResolveArtistIDs_NumericOnly_NoGetArtistsCall(t *testing.T) {
	// When all IDs are numeric, GetArtists() should NOT be called.
	// We verify by making GetArtists() return an error — if it's called, the test fails.
	resolver := &mockArtistResolver{
		err: fmt.Errorf("should not be called"),
	}
	ids, err := resolveArtistIDs(resolver, []string{"1", "2"})
	if err != nil {
		t.Fatalf("unexpected error (GetArtists should not have been called): %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %v", ids)
	}
}
