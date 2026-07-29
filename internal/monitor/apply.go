package monitor

import (
	"fmt"
	"log"
	"strings"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/lidarr"
	"github.com/benscobie/lidarr-utils/internal/state"
)

type albumWriter interface {
	MonitorAlbums(albumIDs []int) error
	SearchAlbum(albumIDs []int) error
}

type albumImporter interface {
	AddAlbum(album lidarr.Album) (*lidarr.Album, error)
	GetRootFolders() ([]lidarr.RootFolder, error)
}

type ApplyStats struct {
	UserUnmonitored   int
	AlbumsMonitored   int
	SearchesSubmitted int
}

func applyAlbums(
	client albumWriter,
	st *state.State,
	dryRun bool,
	albums []common.Album,
) (ApplyStats, error) {
	var stats ApplyStats
	albums = uniqueAlbums(albums)

	var userSkipped []common.Album
	albums, userSkipped = filterUserUnmonitored(albums, st)
	stats.UserUnmonitored = len(userSkipped)
	for _, album := range userSkipped {
		log.Printf(
			"  Skipping %s - %s — previously unmonitored by user",
			album.ArtistName,
			album.Title,
		)
	}
	if len(albums) == 0 {
		log.Println("No albums to monitor or search")
		return stats, nil
	}

	if dryRun {
		log.Printf("[DRY RUN] Would monitor and search %d existing albums", len(albums))
		stats.AlbumsMonitored = len(albums)
		stats.SearchesSubmitted = len(albums)
		return stats, nil
	}

	albumIDs := make([]int, len(albums))
	for i, album := range albums {
		albumIDs[i] = album.ID
	}
	if err := client.MonitorAlbums(albumIDs); err != nil {
		return stats, fmt.Errorf("failed to monitor albums: %w", err)
	}
	stats.AlbumsMonitored = len(albumIDs)
	log.Printf("Monitored %d albums", len(albumIDs))

	if st != nil {
		for _, album := range albums {
			st.RecordAlbum(album.ID, album.ArtistName, album.Title)
		}
		if err := st.Save(); err != nil {
			log.Printf("WARNING: failed to save state file: %v", err)
		}
	}

	if err := client.SearchAlbum(albumIDs); err != nil {
		return stats, fmt.Errorf("failed to search albums: %w", err)
	}
	stats.SearchesSubmitted = len(albumIDs)
	log.Printf("Submitted search for %d albums", len(albumIDs))
	return stats, nil
}

func uniqueAlbums(albums []common.Album) []common.Album {
	seen := make(map[int]struct{}, len(albums))
	result := make([]common.Album, 0, len(albums))
	for _, album := range albums {
		if album.ID <= 0 {
			continue
		}
		if _, duplicate := seen[album.ID]; duplicate {
			continue
		}
		seen[album.ID] = struct{}{}
		result = append(result, album)
	}
	return result
}

func selectRootFolder(
	roots []lidarr.RootFolder,
	configuredPath string,
) (lidarr.RootFolder, error) {
	accessible := make([]lidarr.RootFolder, 0, len(roots))
	for _, root := range roots {
		if root.Accessible {
			accessible = append(accessible, root)
		}
	}
	if len(accessible) == 0 {
		return lidarr.RootFolder{}, fmt.Errorf("Lidarr has no accessible root folders")
	}

	if configuredPath != "" {
		configured := normalizeRootPath(configuredPath)
		for _, root := range accessible {
			if normalizeRootPath(root.Path) == configured {
				return root, nil
			}
		}
		return lidarr.RootFolder{}, fmt.Errorf(
			"configured root folder %q is not accessible in Lidarr",
			configuredPath,
		)
	}

	if len(accessible) != 1 {
		return lidarr.RootFolder{}, fmt.Errorf(
			"root_folder is required when Lidarr has %d accessible root folders",
			len(accessible),
		)
	}
	return accessible[0], nil
}

func normalizeRootPath(path string) string {
	return strings.TrimRight(path, `/\`)
}
