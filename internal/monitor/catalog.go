package monitor

import "github.com/benscobie/lidarr-utils/internal/lidarr"

type catalogClient interface {
	GetArtists() ([]lidarr.Artist, error)
	GetAlbums() ([]lidarr.Album, error)
	GetAlbumsByArtist(artistID int) ([]lidarr.Album, error)
	GetTracksByAlbum(albumID int) ([]lidarr.Track, error)
	LookupAlbum(releaseGroupID string) ([]lidarr.Album, error)
}

type CatalogCache struct {
	client catalogClient

	artists       []lidarr.Artist
	artistsLoaded bool

	allAlbums       []lidarr.Album
	allAlbumsLoaded bool

	albumsByArtist map[int][]lidarr.Album
	artistLoaded   map[int]bool
	tracks         map[int][]lidarr.Track
	tracksLoaded   map[int]bool
	lookups        map[string][]lidarr.Album
	lookupsLoaded  map[string]bool
}

func NewCatalogCache(client catalogClient) *CatalogCache {
	return &CatalogCache{
		client:         client,
		albumsByArtist: make(map[int][]lidarr.Album),
		artistLoaded:   make(map[int]bool),
		tracks:         make(map[int][]lidarr.Track),
		tracksLoaded:   make(map[int]bool),
		lookups:        make(map[string][]lidarr.Album),
		lookupsLoaded:  make(map[string]bool),
	}
}

func (c *CatalogCache) Artists() ([]lidarr.Artist, error) {
	if c.artistsLoaded {
		return c.artists, nil
	}
	artists, err := c.client.GetArtists()
	if err != nil {
		return nil, err
	}
	c.artists = artists
	c.artistsLoaded = true
	return c.artists, nil
}

func (c *CatalogCache) AllAlbums() ([]lidarr.Album, error) {
	if c.allAlbumsLoaded {
		return c.allAlbums, nil
	}
	albums, err := c.client.GetAlbums()
	if err != nil {
		return nil, err
	}
	c.allAlbums = albums
	c.allAlbumsLoaded = true
	return c.allAlbums, nil
}

func (c *CatalogCache) AlbumsForArtist(artistID int) ([]lidarr.Album, error) {
	if c.artistLoaded[artistID] {
		return c.albumsByArtist[artistID], nil
	}
	albums, err := c.client.GetAlbumsByArtist(artistID)
	if err != nil {
		return nil, err
	}
	c.albumsByArtist[artistID] = albums
	c.artistLoaded[artistID] = true
	return albums, nil
}

func (c *CatalogCache) Tracks(albumID int) ([]lidarr.Track, error) {
	if c.tracksLoaded[albumID] {
		return c.tracks[albumID], nil
	}
	tracks, err := c.client.GetTracksByAlbum(albumID)
	if err != nil {
		return nil, err
	}
	c.tracks[albumID] = tracks
	c.tracksLoaded[albumID] = true
	return tracks, nil
}

func (c *CatalogCache) LookupAlbum(releaseGroupID string) ([]lidarr.Album, error) {
	if c.lookupsLoaded[releaseGroupID] {
		return c.lookups[releaseGroupID], nil
	}
	albums, err := c.client.LookupAlbum(releaseGroupID)
	if err != nil {
		return nil, err
	}
	c.lookups[releaseGroupID] = albums
	c.lookupsLoaded[releaseGroupID] = true
	return albums, nil
}
