package monitor

import (
	"fmt"

	"github.com/benscobie/lidarr-utils/internal/common"
)

type vaSourceClient interface {
	VACompilationSource(releaseGroupID string) (string, error)
}

type CompilationSingleResult struct {
	Source   string
	Err      error
	CacheHit bool
}

type CompilationSingleStats struct {
	Checks    int
	CacheHits int
	Failures  int
}

type compilationSingleCachedResult struct {
	source string
	err    error
}

type CompilationSingleClassifier struct {
	client  vaSourceClient
	results map[string]compilationSingleCachedResult
	stats   CompilationSingleStats
}

func NewCompilationSingleClassifier(client vaSourceClient) *CompilationSingleClassifier {
	return &CompilationSingleClassifier{
		client:  client,
		results: make(map[string]compilationSingleCachedResult),
	}
}

func (c *CompilationSingleClassifier) Classify(album common.Album) CompilationSingleResult {
	if (!common.IsEP(album) && !common.IsSingle(album)) || album.ForeignAlbumID == "" {
		return CompilationSingleResult{}
	}

	if result, ok := c.results[album.ForeignAlbumID]; ok {
		c.stats.CacheHits++
		return CompilationSingleResult{
			Source:   result.source,
			Err:      result.err,
			CacheHit: true,
		}
	}

	c.stats.Checks++
	result := compilationSingleCachedResult{}
	if c.client == nil {
		result.err = fmt.Errorf("MusicBrainz client is unavailable")
	} else {
		result.source, result.err = c.client.VACompilationSource(album.ForeignAlbumID)
	}
	if result.err != nil {
		c.stats.Failures++
	}
	c.results[album.ForeignAlbumID] = result
	return CompilationSingleResult{Source: result.source, Err: result.err}
}

func (c *CompilationSingleClassifier) Stats() CompilationSingleStats {
	return c.stats
}
