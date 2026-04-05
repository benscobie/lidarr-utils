package musicbrainz

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestVACompilationSource_Found(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"relations": [
				{
					"type": "single from",
					"release_group": {
						"id": "some-rg-id",
						"title": "MONDAZE FINEST VOL. 4",
						"artist-credit": [
							{
								"artist": {
									"id": "89ad4ac3-39f7-470e-963a-56509c546377",
									"name": "Various Artists"
								}
							}
						]
					}
				}
			]
		}`))
	}))
	defer srv.Close()

	c := NewClient()
	c.baseURL = srv.URL

	title, err := c.VACompilationSource("test-rg-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if title != "MONDAZE FINEST VOL. 4" {
		t.Errorf("expected title %q, got %q", "MONDAZE FINEST VOL. 4", title)
	}
}

func TestVACompilationSource_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"relations": [
				{
					"type": "single from",
					"release_group": {
						"id": "some-other-rg-id",
						"title": "Some Album",
						"artist-credit": [
							{
								"artist": {
									"id": "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
									"name": "Chee"
								}
							}
						]
					}
				}
			]
		}`))
	}))
	defer srv.Close()

	c := NewClient()
	c.baseURL = srv.URL

	title, err := c.VACompilationSource("test-rg-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if title != "" {
		t.Errorf("expected empty string, got %q", title)
	}
}

func TestVACompilationSource_NoRelations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"relations": []}`))
	}))
	defer srv.Close()

	c := NewClient()
	c.baseURL = srv.URL

	title, err := c.VACompilationSource("test-rg-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if title != "" {
		t.Errorf("expected empty string, got %q", title)
	}
}

func TestVACompilationSource_APIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	c := NewClient()
	c.baseURL = srv.URL

	_, err := c.VACompilationSource("test-rg-id")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
