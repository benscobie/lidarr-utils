package monitor

import (
	"fmt"
	"strings"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/config"
)

type ExcludedAlbum struct {
	Album  common.Album
	Reason string
}

func releasePolicyExclusionReason(
	album common.Album,
	policy config.ReleaseSelectionPolicy,
) string {
	if reason := secondaryTypeExclusionReason(album, policy); reason != "" {
		return reason
	}
	if reason := variousArtistsExclusionReason(album.IsVariousArtists, policy); reason != "" {
		return reason
	}
	if common.ShouldExcludeByFormat(album, policy.ExcludeFormats) {
		formats := make([]string, 0, len(album.Releases))
		for _, release := range album.Releases {
			formats = append(formats, release.Format)
		}
		return fmt.Sprintf("no acceptable format (available: %s)", strings.Join(formats, ", "))
	}
	if policy.CompilationSingles == config.CompilationSinglesExclude &&
		album.CompilationSingleSource != "" {
		return fmt.Sprintf("compilation single from %q", album.CompilationSingleSource)
	}
	return ""
}

func secondaryTypeExclusionReason(
	album common.Album,
	policy config.ReleaseSelectionPolicy,
) string {
	if !policy.IncludeSecondaryTypes && len(album.SecondaryTypes) > 0 {
		return fmt.Sprintf(
			"has secondary type %q while secondary types are disabled",
			album.SecondaryTypes[0],
		)
	}
	for _, excludedType := range policy.ExcludeSecondaryTypes {
		for _, secondaryType := range album.SecondaryTypes {
			if strings.EqualFold(secondaryType, excludedType) {
				return fmt.Sprintf("excluded secondary type %q", secondaryType)
			}
		}
	}
	return ""
}

func variousArtistsExclusionReason(
	isVariousArtists bool,
	policy config.ReleaseSelectionPolicy,
) string {
	switch policy.VariousArtists {
	case config.VariousArtistsExclude:
		if isVariousArtists {
			return "owned by Various Artists in Lidarr"
		}
	case config.VariousArtistsOnly:
		if !isVariousArtists {
			return "not owned by Various Artists in Lidarr"
		}
	}
	return ""
}
