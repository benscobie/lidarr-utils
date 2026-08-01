package monitor

import (
	"log"
)

func logSkippedAlbum(skipped SkippedAlbum) {
	log.Printf(
		"  Skip %s: %s (%s)",
		skipped.Album.AlbumType,
		skipped.Album.Title,
		skipped.Reason,
	)
}

func logExcludedAlbum(
	excluded ExcludedAlbum,
	includeArtist bool,
) {
	name := excluded.Album.Title
	if includeArtist && excluded.Album.ArtistName != "" {
		name = excluded.Album.ArtistName + " - " + name
	}
	log.Printf(
		"  Exclude: %s (%s) — %s",
		name,
		excluded.Album.AlbumType,
		excluded.Reason,
	)
}

func logSelectionWarnings(warnings []string) int {
	for _, warning := range warnings {
		log.Printf("  WARNING: %s", warning)
	}
	return len(warnings)
}
