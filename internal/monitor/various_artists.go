package monitor

import (
	"fmt"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/musicbrainz"
)

type vaSourceClient interface {
	VACompilationSource(releaseGroupID string) (string, error)
}

type vaResult struct {
	reason string
	err    error
}

type VAFilter struct {
	client  vaSourceClient
	results map[string]vaResult
}

func NewVAFilter(client vaSourceClient) *VAFilter {
	return &VAFilter{
		client:  client,
		results: make(map[string]vaResult),
	}
}

func (f *VAFilter) ExclusionReason(album common.Album) (string, error) {
	for _, artistID := range album.ForeignArtistIDs {
		if artistID == musicbrainz.VariousArtistsID {
			return "credited directly to Various Artists", nil
		}
	}

	if (!common.IsEP(album) && !common.IsSingle(album)) || album.ForeignAlbumID == "" {
		return "", nil
	}
	if result, ok := f.results[album.ForeignAlbumID]; ok {
		return result.reason, result.err
	}
	if f.client == nil {
		return "", fmt.Errorf("MusicBrainz client is unavailable")
	}

	source, err := f.client.VACompilationSource(album.ForeignAlbumID)
	result := vaResult{err: err}
	if source != "" {
		result.reason = fmt.Sprintf("VA compilation single (from: %s)", source)
	}
	f.results[album.ForeignAlbumID] = result
	return result.reason, result.err
}
