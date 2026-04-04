package monitor

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lidarr-utils/internal/common"
	"github.com/lidarr-utils/internal/lidarr"
)

type Monitor struct {
	client                *lidarr.Client
	dryRun                bool
	officialOnly          bool
	excludeSecondaryTypes []string
	excludeFormats        []string
	queue                 *SearchQueue
}

type Stats struct {
	ArtistsProcessed int
	AlbumsSelected  int
	EPsSelected     int
	SinglesSelected int
	EPsSkipped       int
	SinglesSkipped   int
	Excluded         int
	SearchesSubmitted int
	Warnings         int
}

func NewMonitor(client *lidarr.Client, dryRun bool, officialOnly bool, excludeSecondaryTypes []string, excludeFormats []string, queueCfg QueueConfig) *Monitor {
	return &Monitor{
		client:                client,
		dryRun:                dryRun,
		officialOnly:          officialOnly,
		excludeSecondaryTypes: excludeSecondaryTypes,
		excludeFormats:        excludeFormats,
		queue:                 NewSearchQueue(client, queueCfg, dryRun),
	}
}

func (m *Monitor) Run(artistIDs []int) (*Stats, error) {
	stats := &Stats{}

	albumChan := make(chan common.Album, 10)

	// Stage 2: Search consumer
	searchDone := make(chan int, 1)
	go func() {
		searched := m.queue.ProcessAlbums(albumChan)
		searchDone <- searched
	}()

	// Stage 1: Selection producer
	for i, artistID := range artistIDs {
		artist, err := m.client.GetArtist(artistID)
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
			if len(excluded.Releases) > 0 && common.ShouldExcludeByFormat(excluded, m.excludeFormats) {
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

		for _, album := range result.ToMonitor {
			albumChan <- album
		}
	}

	close(albumChan)
	stats.SearchesSubmitted = <-searchDone

	return stats, nil
}

func (m *Monitor) processArtist(artistID int, artistName string) (*SelectionResult, error) {
	lidarrAlbums, err := m.client.GetAlbumsByArtist(artistID)
	if err != nil {
		return nil, fmt.Errorf("failed to get albums: %w", err)
	}

	var albums []common.Album
	for _, la := range lidarrAlbums {
		tracks, err := m.client.GetTracksByAlbum(la.ID)
		if err != nil {
			log.Printf("Warning: failed to get tracks for album %s: %v", la.Title, err)
		}

		var commonTracks []common.Track
		for _, t := range tracks {
			commonTracks = append(commonTracks, common.Track{
				ID:                 t.ID,
				Title:              t.Title,
				ForeignTrackID:     t.ForeignTrackID,
				ForeignRecordingID: t.ForeignRecordingID,
				TrackNumber:        t.TrackNumber,
				HasFile:            t.HasFile,
			})
		}

		var commonReleases []common.Release
		for _, r := range la.Releases {
			commonReleases = append(commonReleases, common.Release{
				ID:        r.ID,
				Format:    r.Format,
				Monitored: r.Monitored,
			})
		}

		albums = append(albums, common.Album{
			ID:             la.ID,
			Title:          la.Title,
			AlbumType:      la.AlbumType,
			SecondaryTypes: la.SecondaryTypes,
			ArtistID:       la.ArtistID,
			ArtistName:     artistName,
			Tracks:         commonTracks,
			Releases:       commonReleases,
			HasFiles:       la.Statistics != nil && la.Statistics.TrackFileCount > 0,
			Monitored:      la.Monitored,
		})
	}

	result := SelectAlbumsToMonitor(albums, m.officialOnly, m.excludeSecondaryTypes, m.excludeFormats)

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
	fmt.Printf("Searches submitted: %d\n", stats.SearchesSubmitted)
	if stats.Warnings > 0 {
		fmt.Printf("Warnings: %d\n", stats.Warnings)
	}
	fmt.Println()

	if m.dryRun {
		fmt.Println("This was a dry run. To run for real, remove the --dry-run flag.")
	}
}
