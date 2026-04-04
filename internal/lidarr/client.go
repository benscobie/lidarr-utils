package lidarr

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

type Artist struct {
	ID                int    `json:"id"`
	ArtistName        string `json:"artistName"`
	ForeignID         string `json:"foreignArtistId"`
	Status            string `json:"status"`
	Monitored         bool   `json:"monitored"`
	Path              string `json:"path"`
	QualityProfileID  int    `json:"qualityProfileId"`
	MetadataProfileID int    `json:"metadataProfileId"`
	MonitorNewItems   string `json:"monitorNewItems"`
}

type Album struct {
	ID             int              `json:"id"`
	Title          string           `json:"title"`
	ArtistID       int              `json:"artistId"`
	ForeignAlbumID string           `json:"foreignAlbumId"`
	Monitored      bool             `json:"monitored"`
	ProfileID      int              `json:"profileId"`
	Duration       int              `json:"duration"`
	AlbumType      string           `json:"albumType"`
	SecondaryTypes []string         `json:"secondaryTypes"`
	Releases       []Release        `json:"releases,omitempty"`
	ReleaseDate    string           `json:"releaseDate"`
	Statistics     *AlbumStatistics `json:"statistics,omitempty"`
	Artist         *Artist          `json:"artist,omitempty"`
	Tracks         []Track          `json:"tracks,omitempty"`
}

type Release struct {
	ID        int    `json:"id"`
	Format    string `json:"format"`
	Monitored bool   `json:"monitored"`
}

type AlbumStatistics struct {
	TrackFileCount  int     `json:"trackFileCount"`
	TrackCount      int     `json:"trackCount"`
	TotalTrackCount int     `json:"totalTrackCount"`
	SizeOnDisk      int64   `json:"sizeOnDisk"`
	PercentOfTracks float64 `json:"percentOfTracks"`
}

type Track struct {
	ID                  int    `json:"id"`
	ForeignTrackID      string `json:"foreignTrackId"`
	ForeignRecordingID  string `json:"foreignRecordingId"`
	TrackFileID         int    `json:"trackFileId"`
	ArtistID            int    `json:"artistId"`
	AlbumID             int    `json:"albumId"`
	TrackNumber         string `json:"trackNumber"` // Changed to string as API returns string
	AbsoluteTrackNumber int    `json:"absoluteTrackNumber"`
	Title               string `json:"title"`
	Duration            int    `json:"duration"`
	HasFile             bool   `json:"hasFile"`
	Monitored           bool   `json:"monitored"`
	MediumNumber        int    `json:"mediumNumber"`
}

type TrackFile struct {
	ID       int    `json:"id"`
	ArtistID int    `json:"artistId"`
	AlbumID  int    `json:"albumId"`
	Path     string `json:"path"`
	Size     int64  `json:"size"`
	Quality  struct {
		Quality struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"quality"`
	} `json:"quality"`
	MediaInfo struct {
		AudioFormat  string `json:"audioFormat"`
		AudioBitrate int    `json:"audioBitrate"`
	} `json:"mediaInfo"`
}

type Command struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	StartedOn string `json:"startedOn,omitempty"`
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) makeRequest(method, endpoint string, body io.Reader) (*http.Response, error) {
	url := fmt.Sprintf("%s%s", c.baseURL, endpoint)

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

func (c *Client) GetArtists() ([]Artist, error) {
	resp, err := c.makeRequest("GET", "/api/v1/artist", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var artists []Artist
	if err := json.NewDecoder(resp.Body).Decode(&artists); err != nil {
		return nil, err
	}

	return artists, nil
}

func (c *Client) GetAlbumsByArtist(artistID int) ([]Album, error) {
	endpoint := fmt.Sprintf("/api/v1/album?artistId=%d", artistID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var albums []Album
	if err := json.NewDecoder(resp.Body).Decode(&albums); err != nil {
		return nil, err
	}

	return albums, nil
}

func (c *Client) GetTracksByAlbum(albumID int) ([]Track, error) {
	endpoint := fmt.Sprintf("/api/v1/track?albumId=%d", albumID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var tracks []Track
	if err := json.NewDecoder(resp.Body).Decode(&tracks); err != nil {
		return nil, err
	}

	return tracks, nil
}

func (c *Client) UnmonitorAndDeleteFiles(albumID int) error {
	if err := c.setAlbumMonitored(albumID, false); err != nil {
		return fmt.Errorf("failed to unmonitor album: %w", err)
	}

	// Get and delete track files
	tracks, err := c.GetTracksByAlbum(albumID)
	if err != nil {
		return fmt.Errorf("failed to get tracks: %w", err)
	}

	for _, track := range tracks {
		if track.HasFile && track.TrackFileID > 0 {
			if err := c.deleteTrackFile(track.TrackFileID); err != nil {
				return fmt.Errorf("failed to delete track file %d: %w", track.TrackFileID, err)
			}
		}
	}

	return nil
}

func (c *Client) AddToImportExclusion(albumID int) error {
	// Get album details first
	album, err := c.getAlbum(albumID)
	if err != nil {
		return fmt.Errorf("failed to get album: %w", err)
	}

	// Add to import list exclusion
	exclusionData := map[string]interface{}{
		"foreignId":  album.ForeignAlbumID,
		"artistName": album.Artist.ArtistName,
		"albumTitle": album.Title,
	}

	jsonData, err := json.Marshal(exclusionData)
	if err != nil {
		return err
	}

	resp, err := c.makeRequest("POST", "/api/v1/exclusions", strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to add import exclusion: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) getAlbum(albumID int) (*Album, error) {
	endpoint := fmt.Sprintf("/api/v1/album/%d", albumID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var album Album
	if err := json.NewDecoder(resp.Body).Decode(&album); err != nil {
		return nil, err
	}

	return &album, nil
}

func (c *Client) deleteTrackFile(trackFileID int) error {
	endpoint := fmt.Sprintf("/api/v1/trackfile/%d", trackFileID)

	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete track file: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) GetCommands() ([]Command, error) {
	resp, err := c.makeRequest("GET", "/api/v1/command", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var commands []Command
	if err := json.NewDecoder(resp.Body).Decode(&commands); err != nil {
		return nil, err
	}

	return commands, nil
}

func (c *Client) SearchAlbum(albumIDs []int) error {
	commandData := map[string]interface{}{
		"name":     "AlbumSearch",
		"albumIds": albumIDs,
	}

	jsonData, err := json.Marshal(commandData)
	if err != nil {
		return err
	}

	resp, err := c.makeRequest("POST", "/api/v1/command", strings.NewReader(string(jsonData)))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to trigger album search: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) setAlbumMonitored(albumID int, monitored bool) error {
	payload := struct {
		AlbumIDs  []int `json:"albumIds"`
		Monitored bool  `json:"monitored"`
	}{
		AlbumIDs:  []int{albumID},
		Monitored: monitored,
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal monitor request: %w", err)
	}

	resp, err := c.makeRequest("PUT", "/api/v1/album/monitor", strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to update album monitored state: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update album monitored state: status %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) MonitorAlbum(albumID int) error {
	return c.setAlbumMonitored(albumID, true)
}

func (c *Client) GetArtist(artistID int) (*Artist, error) {
	endpoint := fmt.Sprintf("/api/v1/artist/%d", artistID)

	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var artist Artist
	if err := json.NewDecoder(resp.Body).Decode(&artist); err != nil {
		return nil, err
	}

	return &artist, nil
}

func (c *Client) TestConnection() error {
	resp, err := c.makeRequest("GET", "/api/v1/system/status", nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API connection test failed with status %d", resp.StatusCode)
	}

	return nil
}
