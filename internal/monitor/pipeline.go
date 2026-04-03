package monitor

import (
	"fmt"
	"log"
	"time"

	"github.com/lidarr-utils/internal/common"
	"github.com/lidarr-utils/internal/lidarr"
)

type Monitor struct {
	client                *lidarr.Client
	dryRun                bool
	officialOnly          bool
	excludeSecondaryTypes []string
	queue                 *SearchQueue
}

type Stats struct {
	ArtistsProcessed int
	AlbumsMonitored  int
	EPsMonitored     int
	SinglesMonitored int
	EPsSkipped       int
	SinglesSkipped   int
	Excluded         int
	SearchesComplete int
	Warnings         int
}

func NewMonitor(client *lidarr.Client, dryRun bool, officialOnly bool, excludeSecondaryTypes []string, queueCfg QueueConfig) *Monitor {
	return &Monitor{
		client:                client,
		dryRun:                dryRun,
		officialOnly:          officialOnly,
		excludeSecondaryTypes: excludeSecondaryTypes,
		queue:                 NewSearchQueue(client, queueCfg, dryRun),
	}
}

func (m *Monitor) Run(artistIDs []int) (*Stats, error) {
	stats := &Stats{}

	albumChan := make(chan common.Album, 10)

	// Stage 2: Search consumer
	searchDone := make(chan int, 1)
	go func() {
		searched := m.queue.ProcessSearches(albumChan)
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
				stats.AlbumsMonitored++
			case common.IsEP(album):
				stats.EPsMonitored++
			default:
				stats.SinglesMonitored++
			}
			log.Printf("  Monitor: %s (%s)", album.Title, album.AlbumType)
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
			log.Printf("  Exclude: %s (%s) — secondary types: %v",
				excluded.Title, excluded.AlbumType, excluded.SecondaryTypes)
		}

		for _, warning := range result.Warnings {
			log.Printf("  WARNING: %s", warning)
		}

		for _, album := range result.ToMonitor {
			albumChan <- album
		}

		time.Sleep(100 * time.Millisecond)
	}

	close(albumChan)
	stats.SearchesComplete = <-searchDone

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

		albums = append(albums, common.Album{
			ID:             la.ID,
			Title:          la.Title,
			AlbumType:      la.AlbumType,
			SecondaryTypes: la.SecondaryTypes,
			ArtistID:       la.ArtistID,
			ArtistName:     artistName,
			Tracks:         commonTracks,
			HasFiles:       la.Statistics != nil && la.Statistics.TrackFileCount > 0,
			Monitored:      la.Monitored,
		})
	}

	result := SelectAlbumsToMonitor(albums, m.officialOnly, m.excludeSecondaryTypes)

	// Set monitored in Lidarr
	for _, album := range result.ToMonitor {
		if m.dryRun {
			continue
		}
		if err := m.client.MonitorAlbum(album.ID); err != nil {
			log.Printf("ERROR: Failed to monitor album %s: %v", album.Title, err)
		}
	}

	return &result, nil
}

func (m *Monitor) PrintSummary(stats *Stats, duration time.Duration) {
	fmt.Printf("\n=== MONITOR SUMMARY ===\n")
	fmt.Printf("Completed in %v\n", duration)
	fmt.Printf("Artists processed: %d\n", stats.ArtistsProcessed)
	fmt.Printf("Albums monitored: %d\n", stats.AlbumsMonitored)
	fmt.Printf("EPs monitored: %d\n", stats.EPsMonitored)
	fmt.Printf("Singles monitored: %d\n", stats.SinglesMonitored)
	fmt.Printf("EPs skipped: %d\n", stats.EPsSkipped)
	fmt.Printf("Singles skipped: %d\n", stats.SinglesSkipped)
	fmt.Printf("Excluded (secondary type): %d\n", stats.Excluded)
	fmt.Printf("Searches completed: %d\n", stats.SearchesComplete)
	if stats.Warnings > 0 {
		fmt.Printf("Warnings: %d\n", stats.Warnings)
	}
	fmt.Println()

	if m.dryRun {
		fmt.Println("This was a dry run. To run for real, remove the --dry-run flag.")
	}
}
