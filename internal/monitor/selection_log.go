package monitor

import (
	"fmt"
	"log/slog"
)

func logSkippedAlbum(skipped SkippedAlbum) {
	slog.Debug(fmt.Sprintf(
		"  Skip %s: %s (%s)",
		skipped.Album.AlbumType,
		skipped.Album.Title,
		skipped.Reason,
	))
}

func logExcludedAlbum(
	excluded ExcludedAlbum,
	includeArtist bool,
) {
	name := excluded.Album.Title
	if includeArtist && excluded.Album.ArtistName != "" {
		name = excluded.Album.ArtistName + " - " + name
	}
	slog.Debug(fmt.Sprintf(
		"  Exclude: %s (%s) — %s",
		name,
		excluded.Album.AlbumType,
		excluded.Reason,
	))
}

func logSelectionWarnings(warnings []string) int {
	for _, warning := range warnings {
		slog.Warn("Release selection warning", "warning", warning)
	}
	return len(warnings)
}
