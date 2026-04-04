package common

import "testing"

func TestAreTracksTheSame_ByRecordingID(t *testing.T) {
	track1 := Track{ForeignRecordingID: "abc-123", Title: "Song A"}
	track2 := Track{ForeignRecordingID: "abc-123", Title: "Song B"}

	if !AreTracksTheSame(track1, track2) {
		t.Error("expected tracks with same ForeignRecordingID to be the same")
	}
}

func TestAreTracksTheSame_ByTrackID(t *testing.T) {
	track1 := Track{ForeignTrackID: "xyz-456", Title: "Song A"}
	track2 := Track{ForeignTrackID: "xyz-456", Title: "Song B"}

	if !AreTracksTheSame(track1, track2) {
		t.Error("expected tracks with same ForeignTrackID to be the same")
	}
}

func TestAreTracksTheSame_ByNormalizedTitle(t *testing.T) {
	track1 := Track{Title: "Song Title (Remastered)"}
	track2 := Track{Title: "Song Title (Remaster)"}

	if !AreTracksTheSame(track1, track2) {
		t.Error("expected tracks with equivalent normalized titles to be the same")
	}
}

func TestAreTracksTheSame_DifferentTitles(t *testing.T) {
	track1 := Track{Title: "Song A"}
	track2 := Track{Title: "Song B"}

	if AreTracksTheSame(track1, track2) {
		t.Error("expected tracks with different titles to not be the same")
	}
}

func TestAreTracksTheSame_DifferentRecordingIDs(t *testing.T) {
	track1 := Track{ForeignRecordingID: "abc-123", Title: "Same Title"}
	track2 := Track{ForeignRecordingID: "def-456", Title: "Same Title"}

	if AreTracksTheSame(track1, track2) {
		t.Error("expected tracks with different ForeignRecordingIDs to not be the same, even with same title")
	}
}

func TestNormalizeTrackTitle_Remaster(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Song Title (Remaster)", "song title"},
		{"Song Title (Remastered)", "song title"},
		{"Song Title - Remaster", "song title"},
		{"Song Title - Remastered", "song title"},
	}

	for _, tt := range tests {
		result := NormalizeTrackTitle(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeTrackTitle(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeTrackTitle_RadioEdit(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Song Title (Radio Edit)", "song title"},
		{"Song Title - Radio Edit", "song title"},
	}

	for _, tt := range tests {
		result := NormalizeTrackTitle(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeTrackTitle(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeTrackTitle_SingleVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Song Title (Single Version)", "song title"},
		{"Song Title - Single Version", "song title"},
	}

	for _, tt := range tests {
		result := NormalizeTrackTitle(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeTrackTitle(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeTrackTitle_AlbumVersion(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Song Title (Album Version)", "song title"},
		{"Song Title - Album Version", "song title"},
	}

	for _, tt := range tests {
		result := NormalizeTrackTitle(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeTrackTitle(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeTrackTitle_Explicit(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Song Title (Explicit)", "song title"},
		{"Song Title - Explicit", "song title"},
	}

	for _, tt := range tests {
		result := NormalizeTrackTitle(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeTrackTitle(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeTrackTitle_Clean(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Song Title (Clean)", "song title"},
		{"Song Title - Clean", "song title"},
	}

	for _, tt := range tests {
		result := NormalizeTrackTitle(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeTrackTitle(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestNormalizeTrackTitle_Spaces(t *testing.T) {
	result := NormalizeTrackTitle("  Song Title  ")
	if result != "song title" {
		t.Errorf("NormalizeTrackTitle with surrounding spaces = %q, want %q", result, "song title")
	}
}

func TestNormalizeTrackTitle_MultipleSuffixes(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Song Title (Remastered) (Radio Edit)", "song title"},
		{"Song Title (Explicit) (Remastered)", "song title"},
		{"Song Title - Remastered (Single Version)", "song title"},
	}

	for _, tt := range tests {
		result := NormalizeTrackTitle(tt.input)
		if result != tt.expected {
			t.Errorf("NormalizeTrackTitle(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
