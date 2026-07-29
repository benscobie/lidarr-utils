package lidarr

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAddAlbumPostsUnmonitoredWithoutSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var got Album
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Fatal(err)
		}
		if got.Monitored || got.AddOptions.SearchForNewAlbum {
			t.Fatalf("unsafe add payload: %#v", got)
		}
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(Album{ID: 42, ForeignAlbumID: got.ForeignAlbumID})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key")
	album, err := client.AddAlbum(Album{
		ForeignAlbumID: "release-group",
		AddOptions:     AddAlbumOptions{SearchForNewAlbum: false},
	})
	if err != nil || album.ID != 42 {
		t.Fatalf("unexpected result: %#v, %v", album, err)
	}
}

func TestGetRootFoldersDecodesImportDefaults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode([]RootFolder{{
			ID:                       7,
			Path:                     "/music",
			Accessible:               true,
			DefaultQualityProfileID:  2,
			DefaultMetadataProfileID: 3,
			DefaultTags:              []int{5, 8},
		}})
	}))
	defer srv.Close()

	client := NewClient(srv.URL, "key")
	roots, err := client.GetRootFolders()
	if err != nil {
		t.Fatal(err)
	}
	if len(roots) != 1 {
		t.Fatalf("unexpected roots: %#v", roots)
	}
	got := roots[0]
	if got.DefaultQualityProfileID != 2 ||
		got.DefaultMetadataProfileID != 3 ||
		len(got.DefaultTags) != 2 {
		t.Fatalf("root import defaults were not decoded: %#v", got)
	}
}
