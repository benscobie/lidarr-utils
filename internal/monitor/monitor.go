package monitor

import (
	"fmt"
	"strings"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/config"
)

type SelectionResult struct {
	ToMonitor []common.Album
	Skipped   []SkippedAlbum
	Excluded  []common.Album
	Warnings  []string
}

type SkippedAlbum struct {
	Album  common.Album
	Reason string
}

type SelectionOptions struct {
	Filters                  config.MonitorFilters
	SkipFullyCoveredReleases bool
	CandidateReleaseGroups   map[string]struct{}
}

func SelectAlbumsToMonitor(albums []common.Album, opts SelectionOptions) SelectionResult {
	var result SelectionResult

	filtered, excluded := partitionAlbumsByFilters(albums, opts.Filters)
	for _, album := range excluded {
		if isCandidate(album, opts.CandidateReleaseGroups) {
			result.Excluded = append(result.Excluded, album)
		}
	}

	if !opts.SkipFullyCoveredReleases {
		for _, album := range filtered {
			if isCandidate(album, opts.CandidateReleaseGroups) &&
				!album.Monitored && !album.HasFiles {
				result.ToMonitor = append(result.ToMonitor, album)
			}
		}
		return result
	}

	// Sort into tiers
	var albumTier, epTier, singleTier []common.Album
	for _, album := range filtered {
		switch {
		case common.IsAlbum(album):
			albumTier = append(albumTier, album)
		case common.IsEP(album):
			epTier = append(epTier, album)
		case common.IsSingle(album):
			singleTier = append(singleTier, album)
		default:
			albumTier = append(albumTier, album)
		}
	}

	// Build selected set — already-monitored albums go in but not into ToMonitor
	selectedTracks := make(map[string]bool)

	// Process albums (always selected)
	for _, album := range albumTier {
		candidate := isCandidate(album, opts.CandidateReleaseGroups)
		if len(album.Tracks) == 0 && candidate {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("Album '%s' has no track data — monitoring anyway", album.Title))
		}
		addTracksToSelected(album.Tracks, selectedTracks)
		if candidate && !album.Monitored && !album.HasFiles {
			result.ToMonitor = append(result.ToMonitor, album)
		}
	}

	// Process EPs
	for _, ep := range epTier {
		candidate := isCandidate(ep, opts.CandidateReleaseGroups)
		if len(ep.Tracks) == 0 {
			if candidate {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("EP '%s' has no track data — monitoring anyway", ep.Title))
			}
			if candidate && !ep.Monitored && !ep.HasFiles {
				result.ToMonitor = append(result.ToMonitor, ep)
			}
			continue
		}
		if allTracksCovered(ep.Tracks, selectedTracks, filtered) {
			if candidate {
				reason := buildSkipReason(ep, selectedTracks, filtered)
				result.Skipped = append(result.Skipped, SkippedAlbum{Album: ep, Reason: reason})
			}
		} else {
			addTracksToSelected(ep.Tracks, selectedTracks)
			if candidate && !ep.Monitored && !ep.HasFiles {
				result.ToMonitor = append(result.ToMonitor, ep)
			}
		}
	}

	// Process singles
	for _, single := range singleTier {
		candidate := isCandidate(single, opts.CandidateReleaseGroups)
		if len(single.Tracks) == 0 {
			if candidate {
				result.Warnings = append(result.Warnings,
					fmt.Sprintf("Single '%s' has no track data — monitoring anyway", single.Title))
			}
			if candidate && !single.Monitored && !single.HasFiles {
				result.ToMonitor = append(result.ToMonitor, single)
			}
			continue
		}
		if allTracksCovered(single.Tracks, selectedTracks, filtered) {
			if candidate {
				reason := buildSkipReason(single, selectedTracks, filtered)
				result.Skipped = append(result.Skipped, SkippedAlbum{Album: single, Reason: reason})
			}
		} else {
			addTracksToSelected(single.Tracks, selectedTracks)
			if candidate && !single.Monitored && !single.HasFiles {
				result.ToMonitor = append(result.ToMonitor, single)
			}
		}
	}

	return result
}

func partitionAlbumsByFilters(
	albums []common.Album,
	filters config.MonitorFilters,
) (kept, excluded []common.Album) {
	for _, album := range albums {
		if common.ShouldExcludeByFormat(album, filters.ExcludeFormats) ||
			common.ShouldExcludeBySecondaryType(
				album,
				filters.OfficialOnly,
				filters.ExcludeSecondaryTypes,
			) ||
			(filters.ExcludeVAReleases && album.IsVACompilation) {
			excluded = append(excluded, album)
			continue
		}
		kept = append(kept, album)
	}
	return kept, excluded
}

func isCandidate(album common.Album, candidates map[string]struct{}) bool {
	if candidates == nil {
		return true
	}
	_, ok := candidates[album.ForeignAlbumID]
	return ok
}

func trackKey(track common.Track) string {
	if track.ForeignRecordingID != "" {
		return "rec:" + track.ForeignRecordingID
	}
	if track.ForeignTrackID != "" {
		return "trk:" + track.ForeignTrackID
	}
	return "title:" + common.NormalizeTrackTitle(track.Title)
}

func addTracksToSelected(tracks []common.Track, selected map[string]bool) {
	for _, track := range tracks {
		selected[trackKey(track)] = true
	}
}

func allTracksCovered(tracks []common.Track, selected map[string]bool, allAlbums []common.Album) bool {
	for _, track := range tracks {
		found := false

		// Fast path: direct key lookup
		if selected[trackKey(track)] {
			found = true
		}

		if !found {
			// Slow path: check using full matching logic
			for _, album := range allAlbums {
				for _, otherTrack := range album.Tracks {
					if selected[trackKey(otherTrack)] && common.AreTracksTheSame(track, otherTrack) {
						found = true
						break
					}
				}
				if found {
					break
				}
			}
		}

		if !found {
			return false
		}
	}
	return true
}

func buildSkipReason(album common.Album, selected map[string]bool, allAlbums []common.Album) string {
	var reasons []string
	for _, track := range album.Tracks {
		for _, other := range allAlbums {
			if other.ID == album.ID {
				continue
			}
			for _, otherTrack := range other.Tracks {
				if selected[trackKey(otherTrack)] && common.AreTracksTheSame(track, otherTrack) {
					reasons = append(reasons,
						fmt.Sprintf("'%s' found in %s '%s'",
							track.Title, strings.ToLower(other.AlbumType), other.Title))
					break
				}
			}
		}
	}
	if len(reasons) == 0 {
		return "all tracks found in selected albums"
	}
	return strings.Join(reasons, "; ")
}
