package monitor

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/lidarr"
	"github.com/benscobie/lidarr-utils/internal/musicbrainz"
	"github.com/benscobie/lidarr-utils/internal/state"
)

type MonitorOptions struct {
	Client                *lidarr.Client
	DryRun                bool
	OfficialOnly          bool
	ExcludeSecondaryTypes []string
	ExcludeFormats        []string
	MBClient              *musicbrainz.Client
	State                 *state.State
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

func (m *Monitor) Run(artistIDs []int) (*Stats, error) {
	stats := &Stats{}

	var allAlbums []common.Album

	for i, artistID := range artistIDs {
		artist, err := m.opts.Client.GetArtist(artistID)
		if err != nil {
			log.Printf("ERROR: Failed to get artist %d: %v", artistID, err)
			continue
		}

		log.Printf("Processing artist %d/%d: %s", i+1, len(artistIDs), artist.ArtistName)

		result, err := m.processArtist(artistID, artist.ArtistName)
		if err != nil {
			log.Printf("ERROR: Failed to process artist %s: %v", artist.ArtistName, err)
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
			switch {
			case common.IsEP(skipped.Album):
				stats.EPsSkipped++
			default:
				stats.SinglesSkipped++
			}
			log.Printf("  Skip %s: %s (%s)", skipped.Album.AlbumType, skipped.Album.Title, skipped.Reason)
		}

		for _, excluded := range result.Excluded {
			stats.Excluded++
			if excluded.IsVACompilation {
				// Already logged in processArtist with compilation source name
			} else if len(excluded.Releases) > 0 && common.ShouldExcludeByFormat(excluded, m.opts.ExcludeFormats) {
				formats := make([]string, 0, len(excluded.Releases))
				for _, r := range excluded.Releases {
					formats = append(formats, r.Format)
				}
				log.Printf("  Skip: %s (%s) — no acceptable format (available: %s)",
					excluded.Title, excluded.AlbumType, strings.Join(formats, ", "))
			} else {
				log.Printf("  Exclude: %s (%s) — secondary types: %v",
					excluded.Title, excluded.AlbumType, excluded.SecondaryTypes)
			}
		}

		for _, warning := range result.Warnings {
			log.Printf("  WARNING: %s", warning)
		}

		allAlbums = append(allAlbums, result.ToMonitor...)
	}

	// Filter out albums the user has manually unmonitored
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

func (m *Monitor) processArtist(artistID int, artistName string) (*SelectionResult, error) {
	lidarrAlbums, err := m.opts.Client.GetAlbumsByArtist(artistID)
	if err != nil {
		return nil, fmt.Errorf("failed to get albums: %w", err)
	}

	var albums []common.Album
	for _, la := range lidarrAlbums {
		tracks, err := m.opts.Client.GetTracksByAlbum(la.ID)
		if err != nil {
			log.Printf("Warning: failed to get tracks for album %s: %v", la.Title, err)
		}

		albums = append(albums, common.Album{
			ID:             la.ID,
			Title:          la.Title,
			AlbumType:      la.AlbumType,
			SecondaryTypes: la.SecondaryTypes,
			ArtistID:       la.ArtistID,
			ArtistName:     artistName,
			ForeignAlbumID: la.ForeignAlbumID,
			Tracks:         lidarr.ConvertTracks(tracks),
			Releases:       lidarr.ConvertReleases(la.Releases),
			HasFiles:       la.Statistics != nil && la.Statistics.TrackFileCount > 0,
			Monitored:      la.Monitored,
		})
	}

	result := SelectAlbumsToMonitor(albums, m.opts.OfficialOnly, m.opts.ExcludeSecondaryTypes, m.opts.ExcludeFormats)

	if m.opts.MBClient != nil {
		var kept []common.Album
		for _, album := range result.ToMonitor {
			if album.ForeignAlbumID == "" {
				kept = append(kept, album)
				continue
			}
			source, err := m.opts.MBClient.VACompilationSource(album.ForeignAlbumID)
			if err != nil {
				log.Printf("  Warning: MusicBrainz lookup failed for %s: %v", album.Title, err)
				kept = append(kept, album)
				continue
			}
			if source != "" {
				album.IsVACompilation = true
				result.Excluded = append(result.Excluded, album)
				log.Printf("  Exclude: %s (%s) — VA compilation single (from: %s)",
					album.Title, album.AlbumType, source)
			} else {
				kept = append(kept, album)
			}
		}
		result.ToMonitor = kept
	}

	return &result, nil
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
