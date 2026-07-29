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
	httpClient         *http.Client
	baseURL            string
	userAgent          string
	mu                 sync.Mutex
	lastReq            time.Time
	minRequestInterval time.Duration
	sleep              func(time.Duration)
}

func NewClient(version string) *Client {
	return &Client{
		httpClient:         &http.Client{Timeout: 30 * time.Second},
		baseURL:            defaultBaseURL,
		userAgent:          fmt.Sprintf("LidarrUtils/%s ( https://github.com/benscobie/lidarr-utils )", version),
		minRequestInterval: time.Second,
		sleep:              time.Sleep,
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

	if elapsed := time.Since(c.lastReq); elapsed < c.minRequestInterval {
		c.sleep(c.minRequestInterval - elapsed)
	}
	c.lastReq = time.Now()
}

func (c *Client) get(requestURL string) (*http.Response, error) {
	const maxAttempts = 3
	backoff := []time.Duration{time.Second, 2 * time.Second}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		c.rateLimit()

		req, err := http.NewRequest(http.MethodGet, requestURL, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("User-Agent", c.userAgent)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}

		retryable := resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode >= http.StatusInternalServerError
		if !retryable || attempt == maxAttempts-1 {
			return resp, nil
		}

		resp.Body.Close()
		c.sleep(backoff[attempt])
	}

	panic("unreachable")
}

// VACompilationSource checks if a release group has a "single from" relationship
// to a Various Artists release group. Returns the compilation title if found,
// or empty string if not.
func (c *Client) VACompilationSource(releaseGroupID string) (string, error) {
	url := fmt.Sprintf("%s/release-group/%s?inc=release-group-rels+artist-credits&fmt=json",
		c.baseURL, releaseGroupID)

	resp, err := c.get(url)
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
