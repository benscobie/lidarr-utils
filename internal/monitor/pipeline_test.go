package monitor

import (
	"testing"

	"github.com/benscobie/lidarr-utils/internal/config"
	"github.com/benscobie/lidarr-utils/internal/lidarr"
	"github.com/benscobie/lidarr-utils/internal/musicbrainz"
)

type fakeMonitorClient struct {
	*countingCatalogClient
	roots        []lidarr.RootFolder
	rootCalls    int
	addRequests  []lidarr.Album
	monitorCalls [][]int
	searchCalls  [][]int
	nextAlbumID  int
}

func (f *fakeMonitorClient) MonitorAlbums(ids []int) error {
	f.monitorCalls = append(f.monitorCalls, append([]int(nil), ids...))
	return nil
}

func (f *fakeMonitorClient) SearchAlbum(ids []int) error {
	f.searchCalls = append(f.searchCalls, append([]int(nil), ids...))
	return nil
}

func (f *fakeMonitorClient) AddAlbum(album lidarr.Album) (*lidarr.Album, error) {
	f.addRequests = append(f.addRequests, album)
	f.nextAlbumID++
	album.ID = f.nextAlbumID
	if album.Artist != nil {
		artist := *album.Artist
		artist.AddOptions = nil
		album.Artist = &artist
	}
	return &album, nil
}

func (f *fakeMonitorClient) GetRootFolders() ([]lidarr.RootFolder, error) {
	f.rootCalls++
	return f.roots, nil
}

type fakeMonitorMBClient struct {
	vaCalls int
}

func (f *fakeMonitorMBClient) LabelReleaseGroups(_ string) (musicbrainz.LabelBrowseResult, error) {
	return musicbrainz.LabelBrowseResult{}, nil
}

func (f *fakeMonitorMBClient) VACompilationSource(_ string) (string, error) {
	f.vaCalls++
	return "", nil
}

func TestMonitorArtistFetchPolicy(t *testing.T) {
	artists := []lidarr.Artist{
		{ID: 1, ArtistName: "One", ForeignID: "artist-one"},
		{ID: 2, ArtistName: "Two", ForeignID: "artist-two"},
	}
	albums := []lidarr.Album{
		{ID: 10, ArtistID: 1, Title: "One Album", AlbumType: "Album", Monitored: true},
		{ID: 20, ArtistID: 2, Title: "Two Album", AlbumType: "Album", Monitored: true},
	}

	t.Run("all artists uses bulk album snapshot", func(t *testing.T) {
		catalog := newCountingCatalogClient()
		catalog.artists = artists
		catalog.allAlbums = albums
		client := &fakeMonitorClient{countingCatalogClient: catalog}
		mon := NewMonitor(MonitorOptions{Client: client, DryRun: true})

		if _, err := mon.RunAllArtists(); err != nil {
			t.Fatal(err)
		}
		if catalog.artistCalls != 1 ||
			catalog.allAlbumCalls != 1 ||
			len(catalog.albumsByArtistCalls) != 0 {
			t.Fatalf(
				"unexpected fetch policy: all=%d byArtist=%v",
				catalog.allAlbumCalls,
				catalog.albumsByArtistCalls,
			)
		}
	})

	t.Run("selected artists use distinct per-artist snapshots", func(t *testing.T) {
		catalog := newCountingCatalogClient()
		catalog.artists = artists
		catalog.albumsByArtist[1] = albums[:1]
		client := &fakeMonitorClient{countingCatalogClient: catalog}
		mon := NewMonitor(MonitorOptions{Client: client, DryRun: true})

		if _, err := mon.RunArtists([]string{"1", "artist-one", "1"}); err != nil {
			t.Fatal(err)
		}
		if catalog.artistCalls != 1 ||
			catalog.allAlbumCalls != 0 ||
			catalog.albumsByArtistCalls[1] != 1 {
			t.Fatalf(
				"unexpected fetch policy: all=%d byArtist=%v",
				catalog.allAlbumCalls,
				catalog.albumsByArtistCalls,
			)
		}
	})
}

func TestMonitorSkipsTrackFetchForFilteredAlbum(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.artists = []lidarr.Artist{{ID: 1, ArtistName: "Artist", ForeignID: "artist"}}
	catalog.albumsByArtist[1] = []lidarr.Album{{
		ID:             10,
		ArtistID:       1,
		Title:          "Vinyl Album",
		AlbumType:      "Album",
		ForeignAlbumID: "rg-vinyl",
		Releases:       []lidarr.Release{{Format: "Vinyl"}},
	}}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mon := NewMonitor(MonitorOptions{
		Client: client,
		DryRun: true,
		Filters: config.MonitorFilters{
			ExcludeFormats: []string{"Vinyl"},
		},
	})

	stats, err := mon.RunArtists([]string{"1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog.trackCalls) != 0 || stats.Excluded != 1 {
		t.Fatalf("tracks=%v excluded=%d", catalog.trackCalls, stats.Excluded)
	}
}

func TestMonitorSkipsVAWorkWhenDisabled(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.artists = []lidarr.Artist{{
		ID:         1,
		ArtistName: "Various Artists",
		ForeignID:  musicbrainz.VariousArtistsID,
	}}
	catalog.albumsByArtist[1] = []lidarr.Album{{
		ID:             10,
		ArtistID:       1,
		Title:          "Compilation Single",
		AlbumType:      "Single",
		ForeignAlbumID: "rg-single",
	}}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mbClient := &fakeMonitorMBClient{}
	mon := NewMonitor(MonitorOptions{
		Client:   client,
		MBClient: mbClient,
		DryRun:   true,
	})

	stats, err := mon.RunArtists([]string{"1"})
	if err != nil {
		t.Fatal(err)
	}
	if mbClient.vaCalls != 0 || stats.SinglesSelected != 1 {
		t.Fatalf("VA calls=%d singles=%d", mbClient.vaCalls, stats.SinglesSelected)
	}
}

func TestResolveArtists(t *testing.T) {
	artists := []lidarr.Artist{
		{ID: 1, ForeignID: "artist-one"},
		{ID: 2, ForeignID: "artist-two"},
	}

	resolved, err := resolveArtists(artists, []string{"1", "artist-two", "1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(resolved) != 2 || resolved[0].ID != 1 || resolved[1].ID != 2 {
		t.Fatalf("unexpected resolution: %#v", resolved)
	}

	_, err = resolveArtists(artists, []string{"missing"})
	if err == nil {
		t.Fatal("expected unknown artist error")
	}
}
