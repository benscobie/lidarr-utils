package common

import "testing"

func TestIsSingle_ExplicitSingle(t *testing.T) {
	album := Album{AlbumType: "Single", Tracks: []Track{{}, {}, {}}}
	if !IsSingle(album) {
		t.Error("expected album with AlbumType 'Single' to be a single")
	}
}

func TestIsSingle_SmallTrackCountUnknownType(t *testing.T) {
	tests := []struct {
		name       string
		trackCount int
		expected   bool
	}{
		{"1 track", 1, true},
		{"2 tracks", 2, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tracks := make([]Track, tt.trackCount)
			album := Album{AlbumType: "Other", Tracks: tracks}
			if IsSingle(album) != tt.expected {
				t.Errorf("expected IsSingle to be %v for %d tracks with unknown type", tt.expected, tt.trackCount)
			}
		})
	}
}

func TestIsSingle_EP(t *testing.T) {
	album := Album{AlbumType: "EP", Tracks: []Track{{}, {}}}
	if IsSingle(album) {
		t.Error("expected album with AlbumType 'EP' to not be a single even with 2 tracks")
	}
}

func TestIsSingle_Album(t *testing.T) {
	album := Album{AlbumType: "Album", Tracks: []Track{{}, {}}}
	if IsSingle(album) {
		t.Error("expected album with AlbumType 'Album' to not be a single even with 2 tracks")
	}
}

func TestIsEP_Explicit(t *testing.T) {
	album := Album{AlbumType: "EP"}
	if !IsEP(album) {
		t.Error("expected album with AlbumType 'EP' to be an EP")
	}
}

func TestIsEP_Lowercase(t *testing.T) {
	album := Album{AlbumType: "ep"}
	if !IsEP(album) {
		t.Error("expected album with AlbumType 'ep' to be an EP")
	}
}

func TestIsEP_NonEP(t *testing.T) {
	for _, albumType := range []string{"Album", "Single", "Other"} {
		album := Album{AlbumType: albumType}
		if IsEP(album) {
			t.Errorf("expected album with AlbumType %q to not be an EP", albumType)
		}
	}
}

func TestIsAlbum_Explicit(t *testing.T) {
	album := Album{AlbumType: "Album"}
	if !IsAlbum(album) {
		t.Error("expected album with AlbumType 'Album' to be an album")
	}
}

func TestIsAlbum_Lowercase(t *testing.T) {
	album := Album{AlbumType: "album"}
	if !IsAlbum(album) {
		t.Error("expected album with AlbumType 'album' to be an album")
	}
}

func TestIsAlbum_NonAlbum(t *testing.T) {
	for _, albumType := range []string{"EP", "Single", "Other"} {
		album := Album{AlbumType: albumType}
		if IsAlbum(album) {
			t.Errorf("expected album with AlbumType %q to not be an album", albumType)
		}
	}
}

func TestHasSecondaryTypes_Nil(t *testing.T) {
	album := Album{SecondaryTypes: nil}
	if HasSecondaryTypes(album) {
		t.Error("expected nil SecondaryTypes to return false")
	}
}

func TestHasSecondaryTypes_Empty(t *testing.T) {
	album := Album{SecondaryTypes: []string{}}
	if HasSecondaryTypes(album) {
		t.Error("expected empty SecondaryTypes to return false")
	}
}

func TestHasSecondaryTypes_NonEmpty(t *testing.T) {
	album := Album{SecondaryTypes: []string{"Compilation"}}
	if !HasSecondaryTypes(album) {
		t.Error("expected non-empty SecondaryTypes to return true")
	}
}

func TestShouldExcludeBySecondaryType_OfficialOnly_WithTypes(t *testing.T) {
	album := Album{SecondaryTypes: []string{"Compilation"}}
	if !ShouldExcludeBySecondaryType(album, true, nil) {
		t.Error("expected official-only mode to exclude album with secondary types")
	}
}

func TestShouldExcludeBySecondaryType_OfficialOnly_NoTypes(t *testing.T) {
	album := Album{SecondaryTypes: nil}
	if ShouldExcludeBySecondaryType(album, true, nil) {
		t.Error("expected official-only mode to not exclude album without secondary types")
	}
}

func TestShouldExcludeBySecondaryType_ExcludeList_Match(t *testing.T) {
	album := Album{SecondaryTypes: []string{"Compilation", "Live"}}
	if !ShouldExcludeBySecondaryType(album, false, []string{"live"}) {
		t.Error("expected exclude list to match 'Live' case-insensitively")
	}
}

func TestShouldExcludeBySecondaryType_ExcludeList_NoMatch(t *testing.T) {
	album := Album{SecondaryTypes: []string{"Compilation"}}
	if ShouldExcludeBySecondaryType(album, false, []string{"Live"}) {
		t.Error("expected exclude list to not match when types don't overlap")
	}
}

func TestShouldExcludeBySecondaryType_EmptyExcludeList(t *testing.T) {
	album := Album{SecondaryTypes: []string{"Compilation"}}
	if ShouldExcludeBySecondaryType(album, false, []string{}) {
		t.Error("expected empty exclude list to not exclude anything")
	}
}
