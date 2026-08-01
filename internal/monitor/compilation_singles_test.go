package monitor

import (
	"errors"
	"testing"

	"github.com/benscobie/lidarr-utils/internal/common"
)

type fakeCompilationSourceClient struct {
	source string
	err    error
	calls  int
}

func (f *fakeCompilationSourceClient) VACompilationSource(string) (string, error) {
	f.calls++
	return f.source, f.err
}

func TestCompilationSingleClassifierCachesPositiveResult(t *testing.T) {
	client := &fakeCompilationSourceClient{source: "Compilation"}
	classifier := NewCompilationSingleClassifier(client)
	album := common.Album{AlbumType: "Single", ForeignAlbumID: "rg-1"}

	first := classifier.Classify(album)
	second := classifier.Classify(album)

	if first.Source != "Compilation" || first.Err != nil || first.CacheHit {
		t.Fatalf("unexpected first result: %#v", first)
	}
	if second.Source != "Compilation" || second.Err != nil || !second.CacheHit {
		t.Fatalf("unexpected cached result: %#v", second)
	}
	assertCompilationClassifierStats(t, client, classifier, CompilationSingleStats{
		Checks: 1, CacheHits: 1,
	})
}

func TestCompilationSingleClassifierCachesNegativeResult(t *testing.T) {
	client := &fakeCompilationSourceClient{}
	classifier := NewCompilationSingleClassifier(client)
	album := common.Album{AlbumType: "Single", ForeignAlbumID: "rg-1"}

	first := classifier.Classify(album)
	second := classifier.Classify(album)

	if first.Source != "" || first.Err != nil || first.CacheHit {
		t.Fatalf("unexpected first result: %#v", first)
	}
	if second.Source != "" || second.Err != nil || !second.CacheHit {
		t.Fatalf("unexpected cached result: %#v", second)
	}
	assertCompilationClassifierStats(t, client, classifier, CompilationSingleStats{
		Checks: 1, CacheHits: 1,
	})
}

func TestCompilationSingleClassifierCachesErrorResult(t *testing.T) {
	client := &fakeCompilationSourceClient{err: errors.New("unavailable")}
	classifier := NewCompilationSingleClassifier(client)
	album := common.Album{AlbumType: "Single", ForeignAlbumID: "rg-1"}

	first := classifier.Classify(album)
	second := classifier.Classify(album)

	if first.Err == nil || first.CacheHit {
		t.Fatalf("unexpected first result: %#v", first)
	}
	if second.Err == nil || !second.CacheHit {
		t.Fatalf("unexpected cached result: %#v", second)
	}
	assertCompilationClassifierStats(t, client, classifier, CompilationSingleStats{
		Checks: 1, CacheHits: 1, Failures: 1,
	})
}

func TestCompilationSingleClassifierSkipsAlbumsAndSinglesWithoutMBID(t *testing.T) {
	client := &fakeCompilationSourceClient{source: "Compilation"}
	classifier := NewCompilationSingleClassifier(client)

	for _, album := range []common.Album{
		{AlbumType: "Album", ForeignAlbumID: "rg-album"},
		{AlbumType: "Single"},
	} {
		result := classifier.Classify(album)
		if result != (CompilationSingleResult{}) {
			t.Fatalf("unexpected result for %#v: %#v", album, result)
		}
	}
	assertCompilationClassifierStats(t, client, classifier, CompilationSingleStats{})
}

func assertCompilationClassifierStats(
	t *testing.T,
	client *fakeCompilationSourceClient,
	classifier *CompilationSingleClassifier,
	want CompilationSingleStats,
) {
	t.Helper()
	if client.calls != want.Checks {
		t.Errorf("adapter calls = %d, want %d", client.calls, want.Checks)
	}
	if got := classifier.Stats(); got != want {
		t.Errorf("stats = %#v, want %#v", got, want)
	}
}
