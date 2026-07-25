package monitor

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/config"
	"github.com/benscobie/lidarr-utils/internal/lidarr"
	"github.com/benscobie/lidarr-utils/internal/musicbrainz"
	"github.com/benscobie/lidarr-utils/internal/state"
)

type lidarrClient interface {
	catalogClient
	MonitorAlbums(albumIDs []int) error
	SearchAlbum(albumIDs []int) error
	AddAlbum(album lidarr.Album) (*lidarr.Album, error)
	GetRootFolders() ([]lidarr.RootFolder, error)
}

type musicBrainzClient interface {
	LabelReleaseGroups(labelID string) (musicbrainz.LabelBrowseResult, error)
	VACompilationSource(releaseGroupID string) (string, error)
}

type MonitorOptions struct {
	Client                   lidarrClient
	DryRun                   bool
	Filters                  config.MonitorFilters
	SkipFullyCoveredReleases bool
	MBClient                 musicBrainzClient
	State                    *state.State
}

type Monitor struct {
	opts MonitorOptions
}

type Stats struct {
	ArtistsProcessed  int
	AlbumsSelected    int
	EPsSelected       int
	SinglesSelected   int
	EPsSkipped        int
	SinglesSkipped    int
	Excluded          int
	UserUnmonitored   int
	SearchesSubmitted int
	Warnings          int
}

func NewMonitor(opts MonitorOptions) *Monitor {
	return &Monitor{opts: opts}
}

type artistCatalog struct {
	artist lidarr.Artist
	albums []lidarr.Album
}

func (m *Monitor) RunArtists(artistRefs []string) (*Stats, error) {
	cache := NewCatalogCache(m.opts.Client)
	artists, err := cache.Artists()
	if err != nil {
		return nil, fmt.Errorf("failed to get artists: %w", err)
	}
	resolved, err := resolveArtists(artists, artistRefs)
	if err != nil {
		return nil, err
	}

	catalogs := make([]artistCatalog, 0, len(resolved))
	for _, artist := range resolved {
		albums, err := cache.AlbumsForArtist(artist.ID)
		if err != nil {
			log.Printf("ERROR: Failed to get albums for %s: %v", artist.ArtistName, err)
			continue
		}
		catalogs = append(catalogs, artistCatalog{artist: artist, albums: albums})
	}
	return m.runArtistCatalogs(catalogs, cache)
}

func (m *Monitor) RunAllArtists() (*Stats, error) {
	cache := NewCatalogCache(m.opts.Client)
	artists, err := cache.Artists()
	if err != nil {
		return nil, fmt.Errorf("failed to get artists: %w", err)
	}
	albums, err := cache.AllAlbums()
	if err != nil {
		return nil, fmt.Errorf("failed to get albums: %w", err)
	}

	byArtist := make(map[int][]lidarr.Album)
	for _, album := range albums {
		byArtist[album.ArtistID] = append(byArtist[album.ArtistID], album)
	}
	catalogs := make([]artistCatalog, 0, len(artists))
	for _, artist := range artists {
		catalogs = append(catalogs, artistCatalog{
			artist: artist,
			albums: byArtist[artist.ID],
		})
	}
	return m.runArtistCatalogs(catalogs, cache)
}

func resolveArtists(artists []lidarr.Artist, refs []string) ([]lidarr.Artist, error) {
	byID := make(map[int]lidarr.Artist, len(artists))
	byForeignID := make(map[string]lidarr.Artist, len(artists))
	for _, artist := range artists {
		byID[artist.ID] = artist
		if artist.ForeignID != "" {
			byForeignID[artist.ForeignID] = artist
		}
	}

	seen := make(map[int]struct{}, len(refs))
	resolved := make([]lidarr.Artist, 0, len(refs))
	for _, ref := range refs {
		var (
			artist lidarr.Artist
			ok     bool
		)
		if id, err := strconv.Atoi(ref); err == nil {
			artist, ok = byID[id]
		} else {
			artist, ok = byForeignID[ref]
		}
		if !ok {
			return nil, fmt.Errorf("artist %q not found in Lidarr", ref)
		}
		if _, duplicate := seen[artist.ID]; duplicate {
			continue
		}
		seen[artist.ID] = struct{}{}
		resolved = append(resolved, artist)
	}
	return resolved, nil
}

func (m *Monitor) runArtistCatalogs(catalogs []artistCatalog, cache *CatalogCache) (*Stats, error) {
	stats := &Stats{}
	var allAlbums []common.Album
	var vaFilter *VAFilter
	if m.opts.Filters.ExcludeVAReleases {
		vaFilter = NewVAFilter(m.opts.MBClient)
	}

	for i, catalog := range catalogs {
		log.Printf(
			"Processing artist %d/%d: %s",
			i+1,
			len(catalogs),
			catalog.artist.ArtistName,
		)

		result, err := m.processArtist(catalog.artist, catalog.albums, cache, vaFilter)
		if err != nil {
			log.Printf("ERROR: Failed to process artist %s: %v", catalog.artist.ArtistName, err)
			continue
		}

		stats.ArtistsProcessed++
		stats.Warnings += len(result.Warnings)
		for _, album := range result.ToMonitor {
			switch {
			case common.IsAlbum(album):
				stats.AlbumsSelected++
			case common.IsEP(album):
				stats.EPsSelected++
			default:
				stats.SinglesSelected++
			}
			log.Printf("  Selected: %s (%s)", album.Title, album.AlbumType)
		}
		for _, skipped := range result.Skipped {
			if common.IsEP(skipped.Album) {
				stats.EPsSkipped++
			} else {
				stats.SinglesSkipped++
			}
			log.Printf("  Skip %s: %s (%s)", skipped.Album.AlbumType, skipped.Album.Title, skipped.Reason)
		}
		for _, excluded := range result.Excluded {
			stats.Excluded++
			if excluded.IsVACompilation {
				continue
			}
			if len(excluded.Releases) > 0 &&
				common.ShouldExcludeByFormat(excluded, m.opts.Filters.ExcludeFormats) {
				formats := make([]string, 0, len(excluded.Releases))
				for _, release := range excluded.Releases {
					formats = append(formats, release.Format)
				}
				log.Printf(
					"  Skip: %s (%s) — no acceptable format (available: %s)",
					excluded.Title,
					excluded.AlbumType,
					strings.Join(formats, ", "),
				)
			} else {
				log.Printf(
					"  Exclude: %s (%s) — secondary types: %v",
					excluded.Title,
					excluded.AlbumType,
					excluded.SecondaryTypes,
				)
			}
		}
		for _, warning := range result.Warnings {
			log.Printf("  WARNING: %s", warning)
		}
		allAlbums = append(allAlbums, result.ToMonitor...)
	}

	var userSkipped []common.Album
	allAlbums, userSkipped = filterUserUnmonitored(allAlbums, m.opts.State)
	for _, album := range userSkipped {
		stats.UserUnmonitored++
		log.Printf("  Skipping %s - %s — previously unmonitored by user", album.ArtistName, album.Title)
	}

	if len(allAlbums) == 0 {
		log.Println("No albums to monitor or search")
		return stats, nil
	}
	if m.opts.DryRun {
		log.Printf("[DRY RUN] Would monitor and search %d albums", len(allAlbums))
		stats.SearchesSubmitted = len(allAlbums)
		return stats, nil
	}

	albumIDs := make([]int, len(allAlbums))
	for i, album := range allAlbums {
		albumIDs[i] = album.ID
	}
	if err := m.opts.Client.MonitorAlbums(albumIDs); err != nil {
		return stats, fmt.Errorf("failed to monitor albums: %w", err)
	}
	log.Printf("Monitored %d albums", len(albumIDs))

	if m.opts.State != nil {
		for _, album := range allAlbums {
			m.opts.State.RecordAlbum(album.ID, album.ArtistName, album.Title)
		}
		if err := m.opts.State.Save(); err != nil {
			log.Printf("WARNING: failed to save state file: %v", err)
		}
	}

	if err := m.opts.Client.SearchAlbum(albumIDs); err != nil {
		return stats, fmt.Errorf("failed to search albums: %w", err)
	}
	log.Printf("Submitted search for %d albums", len(albumIDs))
	stats.SearchesSubmitted = len(albumIDs)
	return stats, nil
}

func filterUserUnmonitored(candidates []common.Album, s *state.State) (kept []common.Album, skipped []common.Album) {
	if s == nil {
		return candidates, nil
	}
	for _, album := range candidates {
		if s.WasPreviouslyMonitored(album.ID) && !album.Monitored {
			skipped = append(skipped, album)
		} else {
			kept = append(kept, album)
		}
	}
	return kept, skipped
}

func (m *Monitor) processArtist(
	artist lidarr.Artist,
	lidarrAlbums []lidarr.Album,
	cache *CatalogCache,
	vaFilter *VAFilter,
) (*SelectionResult, error) {
	albums := prepareLidarrCatalogue(artist, lidarrAlbums, cache, m.opts.Filters, vaFilter)
	result := SelectAlbumsToMonitor(albums, SelectionOptions{
		Filters:                  m.opts.Filters,
		SkipFullyCoveredReleases: m.opts.SkipFullyCoveredReleases,
	})
	return &result, nil
}

func prepareLidarrCatalogue(
	artist lidarr.Artist,
	lidarrAlbums []lidarr.Album,
	cache *CatalogCache,
	filters config.MonitorFilters,
	vaFilter *VAFilter,
) []common.Album {
	albums := make([]common.Album, len(lidarrAlbums))
	for i, album := range lidarrAlbums {
		albums[i] = albumFromLidarr(album, artist)
	}
	markVAAlbums(albums, filters, vaFilter)

	for i := range albums {
		kept, _ := partitionAlbumsByFilters(albums[i:i+1], filters)
		if len(kept) == 0 {
			continue
		}
		tracks, err := cache.Tracks(albums[i].ID)
		if err != nil {
			log.Printf("Warning: failed to get tracks for album %s: %v", albums[i].Title, err)
			continue
		}
		albums[i].Tracks = lidarr.ConvertTracks(tracks)
	}
	return albums
}

func markVAAlbums(
	albums []common.Album,
	filters config.MonitorFilters,
	vaFilter *VAFilter,
) {
	if vaFilter == nil {
		return
	}
	standardFilters := filters
	standardFilters.ExcludeVAReleases = false
	for i := range albums {
		kept, _ := partitionAlbumsByFilters(albums[i:i+1], standardFilters)
		if len(kept) == 0 {
			continue
		}
		reason, err := vaFilter.ExclusionReason(albums[i])
		if err != nil {
			log.Printf(
				"  Warning: MusicBrainz lookup failed for %s: %v",
				albums[i].Title,
				err,
			)
			continue
		}
		if reason != "" {
			albums[i].IsVACompilation = true
			log.Printf(
				"  Exclude: %s (%s) — %s",
				albums[i].Title,
				albums[i].AlbumType,
				reason,
			)
		}
	}
}

func albumFromLidarr(album lidarr.Album, artist lidarr.Artist) common.Album {
	return common.Album{
		ID:               album.ID,
		Title:            album.Title,
		AlbumType:        album.AlbumType,
		SecondaryTypes:   album.SecondaryTypes,
		ArtistID:         album.ArtistID,
		ArtistName:       artist.ArtistName,
		ForeignArtistIDs: []string{artist.ForeignID},
		ForeignAlbumID:   album.ForeignAlbumID,
		Releases:         lidarr.ConvertReleases(album.Releases),
		HasFiles:         album.Statistics != nil && album.Statistics.TrackFileCount > 0,
		Monitored:        album.Monitored,
	}
}

func (m *Monitor) PrintSummary(stats *Stats, duration time.Duration) {
	fmt.Printf("\n=== MONITOR SUMMARY ===\n")
	fmt.Printf("Completed in %v\n", duration)
	fmt.Printf("Artists processed: %d\n", stats.ArtistsProcessed)
	fmt.Printf("Albums selected: %d\n", stats.AlbumsSelected)
	fmt.Printf("EPs selected: %d\n", stats.EPsSelected)
	fmt.Printf("Singles selected: %d\n", stats.SinglesSelected)
	fmt.Printf("EPs skipped: %d\n", stats.EPsSkipped)
	fmt.Printf("Singles skipped: %d\n", stats.SinglesSkipped)
	fmt.Printf("Excluded: %d\n", stats.Excluded)
	if stats.UserUnmonitored > 0 {
		fmt.Printf("User unmonitored (skipped): %d\n", stats.UserUnmonitored)
	}
	fmt.Printf("Searches submitted: %d\n", stats.SearchesSubmitted)
	if stats.Warnings > 0 {
		fmt.Printf("Warnings: %d\n", stats.Warnings)
	}
	fmt.Println()

	if m.opts.DryRun {
		fmt.Println("This was a dry run. To run for real, remove the --dry-run flag.")
	}
}
