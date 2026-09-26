package cmd

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"github.com/benscobie/lidarr-utils/internal/config"
)

func TestNewMusicBrainzClientUsesConfiguredURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/release" {
			t.Fatalf("request path = %q, want /release", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"release-count":0,"release-offset":0,"releases":[]}`))
	}))
	defer srv.Close()

	cfg := &config.Config{
		MusicBrainz: config.MusicBrainzConfig{URL: srv.URL + "/"},
	}
	client := newMusicBrainzClient(cfg)
	if _, err := client.LabelReleaseGroups("a5bfec28-ea8f-426d-ab23-e14aa692c9b5"); err != nil {
		t.Fatal(err)
	}
}

func TestLabelIDsForRunPositionalOverridesConfig(t *testing.T) {
	got, err := labelIDsForRun(
		[]string{"11111111-1111-1111-1111-111111111111"},
		[]string{"22222222-2222-2222-2222-222222222222"},
	)
	if err != nil ||
		!slices.Equal(got, []string{"11111111-1111-1111-1111-111111111111"}) {
		t.Fatalf("unexpected IDs: %v, %v", got, err)
	}
}

func TestLabelIDsForRunRequiresAtLeastOneID(t *testing.T) {
	if _, err := labelIDsForRun(nil, nil); err == nil {
		t.Fatal("expected usage error")
	}
}

func TestLabelIDsForRunRejectsMalformedPositionalID(t *testing.T) {
	if _, err := labelIDsForRun([]string{"not-a-uuid"}, nil); err == nil {
		t.Fatal("expected UUID validation error")
	}
}
