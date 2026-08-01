package dedupe

import (
	"slices"
	"strings"
	"testing"

	"github.com/benscobie/lidarr-utils/internal/common"
)

func TestFindSingleInOtherAlbumsRequiresEveryDownloadedTrackToBeCovered(t *testing.T) {
	single := testAlbum(1, "Two Track Single", "Single",
		downloadedTrack("First", "recording-1", "track-1"),
		downloadedTrack("Second", "recording-2", "track-2"),
	)
	album := testAlbum(2, "Long Player", "Album",
		downloadedTrack("First", "recording-1", "different-track-id"),
	)

	foundIn, reason := (&Deduper{}).findSingleInOtherAlbums(single, []common.Album{single, album})

	if len(foundIn) != 0 || reason != "" {
		t.Fatalf("partially covered single reported as duplicate: found in %#v, reason %q", foundIn, reason)
	}
}

func TestFindSingleInOtherAlbumsCombinesCoverageAcrossAlbumAndEP(t *testing.T) {
	single := testAlbum(1, "Split Single", "Single",
		downloadedTrack("First", "recording-1", "track-1"),
		downloadedTrack("Second", "recording-2", "track-2"),
	)
	album := testAlbum(2, "Long Player", "Album",
		downloadedTrack("First", "recording-1", "album-track-1"),
	)
	ep := testAlbum(3, "Companion EP", "EP",
		downloadedTrack("Second", "recording-2", "ep-track-2"),
	)

	foundIn, reason := (&Deduper{}).findSingleInOtherAlbums(single, []common.Album{single, album, ep})

	assertAlbumIDs(t, foundIn, 2, 3)
	for _, want := range []string{
		"Track 'First' found in album 'Long Player'",
		"Track 'Second' found in ep 'Companion EP'",
	} {
		if !strings.Contains(reason, want) {
			t.Errorf("reason %q does not contain %q", reason, want)
		}
	}
}

func TestFindSingleInOtherAlbumsDoesNotUseIneligibleCoverage(t *testing.T) {
	single := testAlbum(1, "Candidate", "Single",
		downloadedTrack("Song", "recording-1", "track-1"),
	)

	tests := []struct {
		name   string
		albums []common.Album
	}{
		{
			name:   "candidate itself",
			albums: []common.Album{single},
		},
		{
			name: "another single",
			albums: []common.Album{
				single,
				testAlbum(2, "Other Single", "Single", downloadedTrack("Song", "recording-1", "track-2")),
			},
		},
		{
			name: "album track without a file",
			albums: []common.Album{
				single,
				testAlbum(2, "Long Player", "Album", common.Track{
					Title:              "Song",
					ForeignRecordingID: "recording-1",
					HasFile:            false,
				}),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			foundIn, reason := (&Deduper{}).findSingleInOtherAlbums(single, tt.albums)
			if len(foundIn) != 0 || reason != "" {
				t.Fatalf("single reported as covered by %s: found in %#v, reason %q", tt.name, foundIn, reason)
			}
		})
	}
}

func TestFindSingleInOtherAlbumsIgnoresSingleTracksWithoutFiles(t *testing.T) {
	single := testAlbum(1, "Single With Bonus Track", "Single",
		downloadedTrack("Downloaded Song", "recording-1", "track-1"),
		common.Track{
			Title:              "Unavailable Bonus",
			ForeignRecordingID: "recording-2",
			HasFile:            false,
		},
	)
	album := testAlbum(2, "Long Player", "Album",
		downloadedTrack("Downloaded Song", "recording-1", "album-track-1"),
	)

	foundIn, _ := (&Deduper{}).findSingleInOtherAlbums(single, []common.Album{single, album})

	assertAlbumIDs(t, foundIn, 2)
}

func TestFindSingleInOtherAlbumsUsesTrackIdentityFallbacks(t *testing.T) {
	tests := []struct {
		name        string
		singleTrack common.Track
		albumTrack  common.Track
	}{
		{
			name:        "MusicBrainz recording ID",
			singleTrack: downloadedTrack("Single Title", "same-recording", "single-track"),
			albumTrack:  downloadedTrack("Different Album Title", "same-recording", "album-track"),
		},
		{
			name:        "track ID when recording IDs are absent",
			singleTrack: downloadedTrack("Single Title", "", "same-track"),
			albumTrack:  downloadedTrack("Different Album Title", "", "same-track"),
		},
		{
			name:        "normalized title when IDs are absent",
			singleTrack: downloadedTrack("The Song (Radio Edit)", "", ""),
			albumTrack:  downloadedTrack("  the song  ", "", ""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			single := testAlbum(1, "Candidate", "Single", tt.singleTrack)
			album := testAlbum(2, "Long Player", "Album", tt.albumTrack)

			foundIn, _ := (&Deduper{}).findSingleInOtherAlbums(single, []common.Album{single, album})

			assertAlbumIDs(t, foundIn, 2)
		})
	}
}

func TestFindSingleInOtherAlbumsReturnsEachCoveringAlbumOnce(t *testing.T) {
	single := testAlbum(1, "Two Track Single", "Single",
		downloadedTrack("First", "recording-1", "track-1"),
		downloadedTrack("Second", "recording-2", "track-2"),
	)
	album := testAlbum(2, "Long Player", "Album",
		downloadedTrack("First", "recording-1", "album-track-1"),
		downloadedTrack("Second", "recording-2", "album-track-2"),
	)

	foundIn, _ := (&Deduper{}).findSingleInOtherAlbums(single, []common.Album{single, album})

	assertAlbumIDs(t, foundIn, 2)
}

func TestFindSingleInOtherAlbumsRejectsSingleWithNoDownloadedTracks(t *testing.T) {
	single := testAlbum(1, "Undownloaded Single", "Single", common.Track{
		Title:              "Song",
		ForeignRecordingID: "recording-1",
		HasFile:            false,
	})
	album := testAlbum(2, "Long Player", "Album",
		downloadedTrack("Song", "recording-1", "album-track-1"),
	)

	foundIn, reason := (&Deduper{}).findSingleInOtherAlbums(single, []common.Album{single, album})

	if len(foundIn) != 0 || reason != "" {
		t.Fatalf("single with no downloaded tracks reported as duplicate: found in %#v, reason %q", foundIn, reason)
	}
}

func testAlbum(id int, title, albumType string, tracks ...common.Track) common.Album {
	return common.Album{
		ID:        id,
		Title:     title,
		AlbumType: albumType,
		Tracks:    tracks,
	}
}

func downloadedTrack(title, recordingID, trackID string) common.Track {
	return common.Track{
		Title:              title,
		ForeignRecordingID: recordingID,
		ForeignTrackID:     trackID,
		HasFile:            true,
	}
}

func assertAlbumIDs(t *testing.T, albums []common.Album, want ...int) {
	t.Helper()

	got := make([]int, len(albums))
	for i, album := range albums {
		got[i] = album.ID
	}
	slices.Sort(got)
	slices.Sort(want)

	if !slices.Equal(got, want) {
		t.Fatalf("covering album IDs = %v, want %v", got, want)
	}
}
