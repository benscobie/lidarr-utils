package musicbrainz

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	defaultBaseURL   = "https://musicbrainz.org/ws/2"
	VariousArtistsID = "89ad4ac3-39f7-470e-963a-56509c546377"
)

// Client queries the MusicBrainz API with built-in rate limiting (1 req/sec).
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	mu         sync.Mutex
	lastReq    time.Time
}

func NewClient(version string) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		baseURL:    defaultBaseURL,
		userAgent:  fmt.Sprintf("LidarrUtils/%s ( https://github.com/benscobie/lidarr-utils )", version),
	}
}

type releaseGroupResponse struct {
	Relations []relation `json:"relations"`
}

type relation struct {
	Type         string       `json:"type"`
	ReleaseGroup releaseGroup `json:"release_group"`
}

type releaseGroup struct {
	ID           string         `json:"id"`
	Title        string         `json:"title"`
	ArtistCredit []artistCredit `json:"artist-credit"`
}

type artistCredit struct {
	Artist artist `json:"artist"`
}

type artist struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func (c *Client) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if elapsed := time.Since(c.lastReq); elapsed < time.Second {
		time.Sleep(time.Second - elapsed)
	}
	c.lastReq = time.Now()
}

// VACompilationSource checks if a release group has a "single from" relationship
// to a Various Artists release group. Returns the compilation title if found,
// or empty string if not.
func (c *Client) VACompilationSource(releaseGroupID string) (string, error) {
	c.rateLimit()

	url := fmt.Sprintf("%s/release-group/%s?inc=release-group-rels+artist-credits&fmt=json",
		c.baseURL, releaseGroupID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", c.userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("MusicBrainz API returned status %d for release-group %s",
			resp.StatusCode, releaseGroupID)
	}

	var rg releaseGroupResponse
	if err := json.NewDecoder(resp.Body).Decode(&rg); err != nil {
		return "", err
	}

	for _, rel := range rg.Relations {
		if rel.Type != "single from" {
			continue
		}
		for _, ac := range rel.ReleaseGroup.ArtistCredit {
			if ac.Artist.ID == VariousArtistsID {
				return rel.ReleaseGroup.Title, nil
			}
		}
	}

	return "", nil
}
