package monitor

import (
	"testing"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/musicbrainz"
)

type fakeVASourceClient struct {
	source string
	calls  int
}

func (f *fakeVASourceClient) VACompilationSource(_ string) (string, error) {
	f.calls++
	return f.source, nil
}

func TestVAFilterDirectCreditNeedsNoRelationshipLookup(t *testing.T) {
	fake := &fakeVASourceClient{}
	filter := NewVAFilter(fake)
	reason, err := filter.ExclusionReason(common.Album{
		ForeignArtistIDs: []string{musicbrainz.VariousArtistsID},
	})
	if err != nil || reason == "" || fake.calls != 0 {
		t.Fatalf("unexpected result: %q %v calls=%d", reason, err, fake.calls)
	}
}

func TestVAFilterCachesReleaseGroupRelationship(t *testing.T) {
	fake := &fakeVASourceClient{source: "Compilation"}
	filter := NewVAFilter(fake)
	album := common.Album{AlbumType: "Single", ForeignAlbumID: "rg-1"}

	first, err := filter.ExclusionReason(album)
	if err != nil {
		t.Fatal(err)
	}
	second, err := filter.ExclusionReason(album)
	if err != nil {
		t.Fatal(err)
	}
	if first == "" || second != first || fake.calls != 1 {
		t.Fatalf("unexpected cached result: %q %q calls=%d", first, second, fake.calls)
	}
}
