package musicbrainz

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

const testLabelID = "a5bfec28-ea8f-426d-ab23-e14aa692c9b5"

func TestLabelReleaseGroupsPagesAndMergesEditions(t *testing.T) {
	pages := [][]byte{
		readFixture(t, "1985-music-page-1.json"),
		readFixture(t, "1985-music-page-2.json"),
	}
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if requests >= len(pages) {
			t.Fatalf("unexpected request %d", requests+1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(pages[requests])
		requests++
	}))
	defer srv.Close()

	client := NewClient("test")
	client.baseURL = srv.URL
	client.minRequestInterval = 0

	result, err := client.LabelReleaseGroups(testLabelID)
	if err != nil {
		t.Fatal(err)
	}
	if result.ReleasesSeen != 3 || len(result.Groups) != 2 {
		t.Fatalf("unexpected browse result: %#v", result)
	}
	if requests != 2 {
		t.Fatalf("expected two pages, got %d requests", requests)
	}

	first := groupByID(t, result.Groups, "bba5fb34-41ca-4200-bc7c-5c28faff64d1")
	if len(first.Tracks) != 2 {
		t.Fatalf("expected recordings merged without duplicates: %#v", first.Tracks)
	}
	if len(first.Formats) != 2 || first.Formats[0] != "CD" || first.Formats[1] != "Vinyl" {
		t.Fatalf("expected formats in encounter order: %#v", first.Formats)
	}
	if len(first.SourceLabels) != 1 || first.SourceLabels[0] != testLabelID {
		t.Fatalf("unexpected source labels: %#v", first.SourceLabels)
	}
}

func TestLabelReleaseGroupsDistinguishesMissingFromEmpty(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.NotFound(w, nil)
		}))
		defer srv.Close()

		client := NewClient("test")
		client.baseURL = srv.URL
		client.minRequestInterval = 0

		_, err := client.LabelReleaseGroups(testLabelID)
		if !errors.Is(err, ErrLabelNotFound) {
			t.Fatalf("expected ErrLabelNotFound, got %v", err)
		}
	})

	t.Run("empty", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"release-count":0,"release-offset":0,"releases":[]}`))
		}))
		defer srv.Close()

		client := NewClient("test")
		client.baseURL = srv.URL
		client.minRequestInterval = 0

		result, err := client.LabelReleaseGroups(testLabelID)
		if err != nil {
			t.Fatal(err)
		}
		if result.ReleasesSeen != 0 || len(result.Groups) != 0 {
			t.Fatalf("expected empty successful result, got %#v", result)
		}
	})
}

func TestLabelReleaseGroupsRetriesTransientFailure(t *testing.T) {
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		if requests == 1 {
			http.Error(w, "try again", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"release-count":0,"release-offset":0,"releases":[]}`))
	}))
	defer srv.Close()

	client := NewClient("test")
	client.baseURL = srv.URL
	client.minRequestInterval = 0
	client.sleep = func(_ time.Duration) {}

	if _, err := client.LabelReleaseGroups(testLabelID); err != nil {
		t.Fatal(err)
	}
	if requests != 2 {
		t.Fatalf("expected transient response to be retried once, got %d requests", requests)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func groupByID(t *testing.T, groups []LabelReleaseGroup, id string) LabelReleaseGroup {
	t.Helper()
	for _, group := range groups {
		if group.ID == id {
			return group
		}
	}
	t.Fatalf("release group %s not found", id)
	return LabelReleaseGroup{}
}
