package common

import "strings"

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
