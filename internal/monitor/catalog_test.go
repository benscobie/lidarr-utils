package monitor

import (
	"errors"
	"testing"

	"github.com/benscobie/lidarr-utils/internal/lidarr"
)

type countingCatalogClient struct {
	artists             []lidarr.Artist
	allAlbums           []lidarr.Album
	albumsByArtist      map[int][]lidarr.Album
	tracks              map[int][]lidarr.Track
	lookups             map[string][]lidarr.Album
	artistCalls         int
	allAlbumCalls       int
	albumsByArtistCalls map[int]int
	trackCalls          map[int]int
	lookupCalls         map[string]int
	artistErr           error
}

func newCountingCatalogClient() *countingCatalogClient {
	return &countingCatalogClient{
		albumsByArtist:      make(map[int][]lidarr.Album),
		tracks:              make(map[int][]lidarr.Track),
		lookups:             make(map[string][]lidarr.Album),
		albumsByArtistCalls: make(map[int]int),
		trackCalls:          make(map[int]int),
		lookupCalls:         make(map[string]int),
	}
}

func (f *countingCatalogClient) GetArtists() ([]lidarr.Artist, error) {
	f.artistCalls++
	if f.artistErr != nil {
		return nil, f.artistErr
	}
	return f.artists, nil
}

func (f *countingCatalogClient) GetAlbums() ([]lidarr.Album, error) {
	f.allAlbumCalls++
	return f.allAlbums, nil
}

func (f *countingCatalogClient) GetAlbumsByArtist(artistID int) ([]lidarr.Album, error) {
	f.albumsByArtistCalls[artistID]++
	return f.albumsByArtist[artistID], nil
}

func (f *countingCatalogClient) GetTracksByAlbum(albumID int) ([]lidarr.Track, error) {
	f.trackCalls[albumID]++
	return f.tracks[albumID], nil
}

func (f *countingCatalogClient) LookupAlbum(releaseGroupID string) ([]lidarr.Album, error) {
	f.lookupCalls[releaseGroupID]++
	return f.lookups[releaseGroupID], nil
}

func TestCatalogCacheFetchesSnapshotsAndKeysOnce(t *testing.T) {
	fake := newCountingCatalogClient()
	cache := NewCatalogCache(fake)

	_, _ = cache.Artists()
	_, _ = cache.Artists()
	_, _ = cache.AllAlbums()
	_, _ = cache.AllAlbums()
	_, _ = cache.AlbumsForArtist(3)
	_, _ = cache.AlbumsForArtist(3)
	_, _ = cache.Tracks(7)
	_, _ = cache.Tracks(7)
	_, _ = cache.LookupAlbum("rg-1")
	_, _ = cache.LookupAlbum("rg-1")

	if fake.artistCalls != 1 ||
		fake.allAlbumCalls != 1 ||
		fake.albumsByArtistCalls[3] != 1 ||
		fake.trackCalls[7] != 1 ||
		fake.lookupCalls["rg-1"] != 1 {
		t.Fatalf("cache missed: %#v", fake)
	}
}

func TestCatalogCacheDoesNotCacheErrors(t *testing.T) {
	fake := newCountingCatalogClient()
	fake.artistErr = errors.New("temporary")
	cache := NewCatalogCache(fake)

	_, _ = cache.Artists()
	fake.artistErr = nil
	_, err := cache.Artists()

	if err != nil || fake.artistCalls != 2 {
		t.Fatalf("expected a successful retry, calls=%d err=%v", fake.artistCalls, err)
	}
}
