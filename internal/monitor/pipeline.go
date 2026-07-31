package monitor

import (
	"fmt"
	"log"
	"strconv"
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
	Client   lidarrClient
	DryRun   bool
	Policy   config.ReleaseSelectionPolicy
	MBClient musicBrainzClient
	State    *state.State
}

type Monitor struct {
	opts MonitorOptions
}

type Stats struct {
	ArtistsProcessed      int
	AlbumsSelected        int
	EPsSelected           int
	SinglesSelected       int
	EPsSkipped            int
	SinglesSkipped        int
	Excluded              int
	UserUnmonitored       int
	SearchesSubmitted     int
	Warnings              int
	RelationshipChecks    int
	RelationshipCacheHits int
	RelationshipFailures  int
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
	var classifier *CompilationSingleClassifier
	if m.opts.Policy.CompilationSingles == config.CompilationSinglesExclude {
		classifier = NewCompilationSingleClassifier(m.opts.MBClient)
	}

	for i, catalog := range catalogs {
		log.Printf(
			"Processing artist %d/%d: %s",
			i+1,
			len(catalogs),
			catalog.artist.ArtistName,
		)

		result, err := m.processArtist(catalog.artist, catalog.albums, cache, classifier)
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
			logSkippedAlbum(skipped)
		}
		for _, excluded := range result.Excluded {
			stats.Excluded++
			logExcludedAlbum(excluded, false)
		}
		logSelectionWarnings(result.Warnings)
		allAlbums = append(allAlbums, result.ToMonitor...)
	}
	if classifier != nil {
		classifierStats := classifier.Stats()
		stats.RelationshipChecks = classifierStats.Checks
		stats.RelationshipCacheHits = classifierStats.CacheHits
		stats.RelationshipFailures = classifierStats.Failures
	}

	applyStats, err := applyAlbums(m.opts.Client, m.opts.State, m.opts.DryRun, allAlbums)
	if err != nil {
		return stats, err
	}
	stats.UserUnmonitored = applyStats.UserUnmonitored
	stats.SearchesSubmitted = applyStats.SearchesSubmitted
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
	classifier *CompilationSingleClassifier,
) (*SelectionResult, error) {
	albums, warnings := prepareLidarrCatalogue(
		artist,
		lidarrAlbums,
		cache,
		m.opts.Policy,
		classifier,
	)
	result := SelectAlbumsToMonitor(albums, SelectionOptions{
		Policy: m.opts.Policy,
	})
	result.Warnings = append(warnings, result.Warnings...)
	return &result, nil
}

func prepareLidarrCatalogue(
	artist lidarr.Artist,
	lidarrAlbums []lidarr.Album,
	cache *CatalogCache,
	policy config.ReleaseSelectionPolicy,
	classifier *CompilationSingleClassifier,
) ([]common.Album, []string) {
	albums := make([]common.Album, len(lidarrAlbums))
	var warnings []string
	for i, album := range lidarrAlbums {
		prepared, albumWarnings := prepareAlbumForSelection(
			albumFromLidarr(album, artist),
			policy,
			classifier,
			func() ([]common.Track, error) {
				tracks, err := cache.Tracks(album.ID)
				return lidarr.ConvertTracks(tracks), err
			},
		)
		albums[i] = prepared
		warnings = append(warnings, albumWarnings...)
	}
	return albums, warnings
}

type albumTrackLoader func() ([]common.Track, error)

func prepareAlbumForSelection(
	album common.Album,
	policy config.ReleaseSelectionPolicy,
	classifier *CompilationSingleClassifier,
	loadTracks albumTrackLoader,
) (common.Album, []string) {
	if reason := releasePolicyExclusionReason(album, policy); reason != "" {
		return album, nil
	}
	var warnings []string
	if classifier != nil && (common.IsEP(album) || common.IsSingle(album)) {
		result := classifier.Classify(album)
		if result.Err != nil {
			if !result.CacheHit {
				warnings = append(warnings, fmt.Sprintf(
					"MusicBrainz compilation-single lookup failed for %s: %v",
					album.Title,
					result.Err,
				))
			}
		} else {
			album.CompilationSingleSource = result.Source
		}
	}
	if reason := releasePolicyExclusionReason(album, policy); reason != "" {
		return album, warnings
	}
	tracks, err := loadTracks()
	if err != nil {
		warnings = append(warnings, fmt.Sprintf(
			"failed to get tracks for album %s: %v",
			album.Title,
			err,
		))
		return album, warnings
	}
	album.Tracks = tracks
	return album, warnings
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
		IsVariousArtists: artist.ForeignID == musicbrainz.VariousArtistsID,
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
	if stats.RelationshipChecks > 0 {
		fmt.Printf("Relationship checks: %d\n", stats.RelationshipChecks)
	}
	if stats.RelationshipCacheHits > 0 {
		fmt.Printf("Relationship cache hits: %d\n", stats.RelationshipCacheHits)
	}
	if stats.RelationshipFailures > 0 {
		fmt.Printf("Relationship failures: %d\n", stats.RelationshipFailures)
	}
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
