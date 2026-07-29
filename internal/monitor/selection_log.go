package monitor

import (
	"log"
	"strings"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/config"
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
	album common.Album,
	filters config.MonitorFilters,
	includeArtist bool,
) {
	if album.IsVACompilation {
		return
	}

	name := album.Title
	if includeArtist && album.ArtistName != "" {
		name = album.ArtistName + " - " + name
	}
	if len(album.Releases) > 0 &&
		common.ShouldExcludeByFormat(album, filters.ExcludeFormats) {
		formats := make([]string, 0, len(album.Releases))
		for _, release := range album.Releases {
			formats = append(formats, release.Format)
		}
		log.Printf(
			"  Skip: %s (%s) — no acceptable format (available: %s)",
			name,
			album.AlbumType,
			strings.Join(formats, ", "),
		)
		return
	}

	log.Printf(
		"  Exclude: %s (%s) — secondary types: %v",
		name,
		album.AlbumType,
		album.SecondaryTypes,
	)
}

func logSelectionWarnings(warnings []string) {
	for _, warning := range warnings {
		log.Printf("  WARNING: %s", warning)
	}
}
