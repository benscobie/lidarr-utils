package common

import (
	"regexp"
	"strings"
)

func IsSingle(album Album) bool {
	albumType := strings.ToLower(album.AlbumType)
	trackCount := len(album.Tracks)

	if albumType == "single" {
		return true
	}

	if trackCount <= 2 && albumType != "ep" && albumType != "album" {
		return true
	}

	return false
}

func IsEP(album Album) bool {
	return strings.ToLower(album.AlbumType) == "ep"
}

func IsAlbum(album Album) bool {
	return strings.ToLower(album.AlbumType) == "album"
}

func HasSecondaryTypes(album Album) bool {
	return len(album.SecondaryTypes) > 0
}

func ShouldExcludeBySecondaryType(album Album, officialOnly bool, excludeTypes []string) bool {
	if officialOnly {
		return HasSecondaryTypes(album)
	}

	for _, excludeType := range excludeTypes {
		for _, albumType := range album.SecondaryTypes {
			if strings.EqualFold(albumType, excludeType) {
				return true
			}
		}
	}

	return false
}

var quantityPrefixRe = regexp.MustCompile(`^\d+(x|")\s*`)

// ParseFormatComponents splits a release format string like "2xVinyl, CD"
// into normalized components: ["Vinyl", "CD"].
func ParseFormatComponents(format string) []string {
	if format == "" {
		return nil
	}
	parts := strings.Split(format, ", ")
	components := make([]string, 0, len(parts))
	for _, part := range parts {
		stripped := quantityPrefixRe.ReplaceAllString(strings.TrimSpace(part), "")
		if stripped != "" {
			components = append(components, stripped)
		}
	}
	return components
}

// ShouldExcludeByFormat returns true if every release on the album has only
// formats present in the exclude list. If any release has at least one
// non-excluded format component, the album passes.
func ShouldExcludeByFormat(album Album, excludeFormats []string) bool {
	if len(excludeFormats) == 0 || len(album.Releases) == 0 {
		return false
	}

	excludeSet := make(map[string]bool, len(excludeFormats))
	for _, f := range excludeFormats {
		excludeSet[strings.ToLower(f)] = true
	}

	for _, release := range album.Releases {
		components := ParseFormatComponents(release.Format)
		if len(components) == 0 {
			// Empty/unknown format — don't exclude
			return false
		}
		hasAcceptable := false
		for _, c := range components {
			if !excludeSet[strings.ToLower(c)] {
				hasAcceptable = true
				break
			}
		}
		if hasAcceptable {
			return false
		}
	}

	return true
}
