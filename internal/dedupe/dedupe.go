package dedupe

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/lidarr-utils/internal/lidarr"
)

type Deduper struct {
	client             *lidarr.Client
	dryRun             bool
	addImportExclusion bool
}

type DuplicateResult struct {
	SingleAlbum   Album
	FoundInAlbums []Album
	Reason        string
}

type Album struct {
	ID         int
	Title      string
	AlbumType  string
	ArtistID   int
	ArtistName string
	Tracks     []Track
	HasFiles   bool
}

type Track struct {
	ID                 int
	Title              string
	ForeignTrackID     string
	ForeignRecordingID string
	TrackNumber        string // Changed to string to match API
	HasFile            bool
}

func NewDeduper(client *lidarr.Client, dryRun bool, addImportExclusion bool) *Deduper {
	return &Deduper{
		client:             client,
		dryRun:             dryRun,
		addImportExclusion: addImportExclusion,
	}
}

func (d *Deduper) FindDuplicates() ([]DuplicateResult, error) {
	log.Println("Starting duplicate detection...")

	artists, err := d.client.GetArtists()
	if err != nil {
		return nil, fmt.Errorf("failed to get artists: %w", err)
	}

	log.Printf("Found %d artists", len(artists))

	var allDuplicates []DuplicateResult

	for i, artist := range artists {
		log.Printf("Processing artist %d/%d: %s", i+1, len(artists), artist.ArtistName)

		duplicates, err := d.findDuplicatesForArtist(artist.ID, artist.ArtistName)
		if err != nil {
			log.Printf("Error processing artist %s: %v", artist.ArtistName, err)
			continue
		}

		allDuplicates = append(allDuplicates, duplicates...)

		// Add a small delay to avoid overwhelming the API
		time.Sleep(100 * time.Millisecond)
	}

	log.Printf("Found %d duplicate singles", len(allDuplicates))
	return allDuplicates, nil
}

func (d *Deduper) findDuplicatesForArtist(artistID int, artistName string) ([]DuplicateResult, error) {
	albums, err := d.client.GetAlbumsByArtist(artistID)
	if err != nil {
		return nil, fmt.Errorf("failed to get albums for artist %d: %w", artistID, err)
	}

	// Convert to our internal format and filter for downloaded albums
	var processedAlbums []Album
	for _, album := range albums {
		tracks, err := d.client.GetTracksByAlbum(album.ID)
		if err != nil {
			log.Printf("Warning: failed to get tracks for album %s: %v", album.Title, err)
			continue
		}

		// Convert tracks
		var processedTracks []Track
		hasFiles := false
		for _, track := range tracks {
			processedTracks = append(processedTracks, Track{
				ID:                 track.ID,
				Title:              track.Title,
				ForeignTrackID:     track.ForeignTrackID,
				ForeignRecordingID: track.ForeignRecordingID,
				TrackNumber:        track.TrackNumber,
				HasFile:            track.HasFile,
			})
			if track.HasFile {
				hasFiles = true
			}
		}

		// Only include albums with downloaded files
		if hasFiles {
			processedAlbums = append(processedAlbums, Album{
				ID:         album.ID,
				Title:      album.Title,
				AlbumType:  album.AlbumType,
				ArtistID:   album.ArtistID,
				ArtistName: artistName,
				Tracks:     processedTracks,
				HasFiles:   hasFiles,
			})
		}
	}

	log.Printf("Artist %s has %d albums with downloaded files", artistName, len(processedAlbums))

	// Find singles and check for duplicates
	var duplicates []DuplicateResult

	for _, album := range processedAlbums {
		if d.isSingle(album) {
			foundIn, reason := d.findSingleInOtherAlbums(album, processedAlbums)
			if len(foundIn) > 0 {
				duplicates = append(duplicates, DuplicateResult{
					SingleAlbum:   album,
					FoundInAlbums: foundIn,
					Reason:        reason,
				})
			}
		}
	}

	return duplicates, nil
}

func (d *Deduper) isSingle(album Album) bool {
	// Consider it a single if:
	// 1. AlbumType is "Single"
	// 2. Has only 1-2 tracks (allowing for B-side or alternate version)
	// 3. Not explicitly an EP or Album

	albumType := strings.ToLower(album.AlbumType)
	trackCount := len(album.Tracks)

	// Explicit single type
	if albumType == "single" {
		return true
	}

	// Small track count that's not explicitly an EP or Album
	if trackCount <= 2 && albumType != "ep" && albumType != "album" {
		return true
	}

	return false
}

func (d *Deduper) findSingleInOtherAlbums(single Album, allAlbums []Album) ([]Album, string) {
	// Get all downloadable tracks from the single
	var singleTracks []Track
	for _, track := range single.Tracks {
		if track.HasFile {
			singleTracks = append(singleTracks, track)
		}
	}

	if len(singleTracks) == 0 {
		return nil, ""
	}

	// Track which single tracks have been found and where
	trackMatches := make(map[int][]Album)  // track index -> albums where found
	trackReasons := make(map[int][]string) // track index -> reasons

	// Check each album to see which single tracks it contains
	for _, album := range allAlbums {
		// Skip the single itself
		if album.ID == single.ID {
			continue
		}

		// Skip other singles
		if d.isSingle(album) {
			continue
		}

		// Check each track from the single against this album
		for singleTrackIdx, singleTrack := range singleTracks {
			for _, albumTrack := range album.Tracks {
				if !albumTrack.HasFile {
					continue
				}

				if d.areTracksTheSame(singleTrack, albumTrack) {
					// Add this album to the matches for this track
					trackMatches[singleTrackIdx] = append(trackMatches[singleTrackIdx], album)
					trackReasons[singleTrackIdx] = append(trackReasons[singleTrackIdx],
						fmt.Sprintf("Track '%s' found in %s '%s'",
							singleTrack.Title, strings.ToLower(album.AlbumType), album.Title))
					break // Found this track in this album, no need to check other tracks in this album
				}
			}
		}
	}

	// Check if ALL tracks from the single have been found
	if len(trackMatches) != len(singleTracks) {
		// Not all tracks were found, so this single is not a duplicate
		return nil, ""
	}

	// All tracks found - collect unique albums and build reason string
	albumSet := make(map[int]Album)
	var allReasons []string

	for trackIdx := range singleTracks {
		if matches, exists := trackMatches[trackIdx]; exists {
			for _, album := range matches {
				albumSet[album.ID] = album
			}
			if reasons, exists := trackReasons[trackIdx]; exists {
				allReasons = append(allReasons, reasons...)
			}
		}
	}

	// Convert set to slice
	var foundIn []Album
	for _, album := range albumSet {
		foundIn = append(foundIn, album)
	}

	var reason string
	if len(allReasons) > 0 {
		reason = strings.Join(allReasons, "; ")
	}

	return foundIn, reason
}

func (d *Deduper) areTracksTheSame(track1, track2 Track) bool {
	// Method 1: Compare by MusicBrainz recording IDs (most reliable)
	if track1.ForeignRecordingID != "" && track2.ForeignRecordingID != "" {
		return track1.ForeignRecordingID == track2.ForeignRecordingID
	}

	// Method 2: Compare by MusicBrainz track IDs
	if track1.ForeignTrackID != "" && track2.ForeignTrackID != "" {
		return track1.ForeignTrackID == track2.ForeignTrackID
	}

	// Method 3: Compare by normalized titles (fallback)
	return d.normalizeTrackTitle(track1.Title) == d.normalizeTrackTitle(track2.Title)
}

func (d *Deduper) normalizeTrackTitle(title string) string {
	// Remove common variations and normalize for comparison
	title = strings.ToLower(title)
	title = strings.TrimSpace(title)

	// Remove common suffixes that might indicate different versions
	suffixes := []string{
		" (remaster)", " (remastered)", " - remaster", " - remastered",
		" (radio edit)", " - radio edit",
		" (single version)", " - single version",
		" (album version)", " - album version",
		" (explicit)", " - explicit",
		" (clean)", " - clean",
	}

	for _, suffix := range suffixes {
		title = strings.TrimSuffix(title, suffix)
	}

	return title
}

func (d *Deduper) ProcessDuplicates(duplicates []DuplicateResult) error {
	if len(duplicates) == 0 {
		log.Println("No duplicates to process")
		return nil
	}

	log.Printf("Processing %d duplicate singles...", len(duplicates))

	for i, duplicate := range duplicates {
		log.Printf("Processing duplicate %d/%d: '%s' by %s",
			i+1, len(duplicates), duplicate.SingleAlbum.Title, duplicate.SingleAlbum.ArtistName)
		log.Printf("  Reason: %s", duplicate.Reason)

		if d.dryRun {
			log.Printf("  [DRY RUN] Would unmonitor and delete files for single: %s", duplicate.SingleAlbum.Title)
			if d.addImportExclusion {
				log.Printf("  [DRY RUN] Would add to import exclusion list")
			}
		} else {
			log.Printf("  Unmonitoring and deleting files for single: %s", duplicate.SingleAlbum.Title)

			// Unmonitor and delete files
			err := d.client.UnmonitorAndDeleteFiles(duplicate.SingleAlbum.ID)
			if err != nil {
				log.Printf("  ERROR: Failed to unmonitor and delete files: %v", err)
				continue
			}

			log.Printf("  Successfully unmonitored and deleted files")

			// Add to import exclusion if requested
			if d.addImportExclusion {
				err := d.client.AddToImportExclusion(duplicate.SingleAlbum.ID)
				if err != nil {
					log.Printf("  WARNING: Failed to add to import exclusion list: %v", err)
				} else {
					log.Printf("  Added to import exclusion list")
				}
			}
		}

		// Add a small delay between deletions
		time.Sleep(500 * time.Millisecond)
	}

	return nil
}

func (d *Deduper) PrintSummary(duplicates []DuplicateResult) {
	fmt.Printf("\n=== DUPLICATE DETECTION SUMMARY ===\n")
	fmt.Printf("Found %d duplicate singles\n\n", len(duplicates))

	if len(duplicates) == 0 {
		fmt.Println("No duplicates found! Your library is clean.")
		return
	}

	for i, duplicate := range duplicates {
		fmt.Printf("%d. Single: '%s' by %s\n", i+1, duplicate.SingleAlbum.Title, duplicate.SingleAlbum.ArtistName)
		fmt.Printf("   Reason: %s\n", duplicate.Reason)
		fmt.Printf("   Found in %d other album(s):\n", len(duplicate.FoundInAlbums))
		for _, album := range duplicate.FoundInAlbums {
			fmt.Printf("     - %s: '%s'\n", strings.Title(album.AlbumType), album.Title)
		}

		if d.dryRun {
			fmt.Printf("   Action: [DRY RUN] Would unmonitor and delete files")
			if d.addImportExclusion {
				fmt.Printf(" and add to exclusion list")
			}
			fmt.Printf("\n\n")
		} else {
			fmt.Printf("   Action: Unmonitored and deleted files")
			if d.addImportExclusion {
				fmt.Printf(" and added to exclusion list")
			}
			fmt.Printf("\n\n")
		}
	}

	if d.dryRun {
		fmt.Printf("This was a dry run. To actually process duplicates, run with --dry-run=false\n")
	}
}
