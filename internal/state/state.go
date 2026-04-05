package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"
)

// MonitoredAlbum records that a specific album was set to monitored.
type MonitoredAlbum struct {
	ArtistName  string    `json:"artist_name"`
	AlbumTitle  string    `json:"album_title"`
	MonitoredAt time.Time `json:"monitored_at"`
}

// stateFile is the on-disk JSON format. Map keys are string because JSON
// requires string keys.
type stateFile struct {
	Version         int                       `json:"version"`
	MonitoredAlbums map[string]MonitoredAlbum `json:"monitored_albums"`
}

// State is the in-memory representation. Map keys are Lidarr album IDs (ints).
type State struct {
	path            string
	MonitoredAlbums map[int]MonitoredAlbum
}

// Load reads state from a JSON file at path. If the file does not exist,
// an empty state is returned. If the file contains corrupt JSON, a warning
// is logged and an empty state is returned.
func Load(path string) (*State, error) {
	s := &State{
		path:            path,
		MonitoredAlbums: make(map[int]MonitoredAlbum),
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, fmt.Errorf("reading state file: %w", err)
	}

	var sf stateFile
	if err := json.Unmarshal(data, &sf); err != nil {
		log.Printf("WARNING: state file %s contains invalid JSON, starting with empty state: %v", path, err)
		return s, nil
	}

	for k, v := range sf.MonitoredAlbums {
		id, err := strconv.Atoi(k)
		if err != nil {
			log.Printf("WARNING: skipping non-integer album ID %q in state file", k)
			continue
		}
		s.MonitoredAlbums[id] = v
	}

	return s, nil
}

// Save writes the state to disk atomically by writing to a temporary file
// and then renaming it.
func (s *State) Save() error {
	sf := stateFile{
		Version:         1,
		MonitoredAlbums: make(map[string]MonitoredAlbum, len(s.MonitoredAlbums)),
	}

	for k, v := range s.MonitoredAlbums {
		sf.MonitoredAlbums[strconv.Itoa(k)] = v
	}

	data, err := json.MarshalIndent(sf, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling state: %w", err)
	}

	tmpPath := s.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing temporary state file: %w", err)
	}

	if err := os.Rename(tmpPath, s.path); err != nil {
		os.Remove(tmpPath) // best-effort cleanup
		return fmt.Errorf("renaming temporary state file: %w", err)
	}

	return nil
}

// RecordAlbum adds or updates a monitored album entry with the current time.
func (s *State) RecordAlbum(albumID int, artistName, albumTitle string) {
	s.MonitoredAlbums[albumID] = MonitoredAlbum{
		ArtistName:  artistName,
		AlbumTitle:  albumTitle,
		MonitoredAt: time.Now(),
	}
}

// WasPreviouslyMonitored returns true if the given album ID exists in state.
func (s *State) WasPreviouslyMonitored(albumID int) bool {
	_, ok := s.MonitoredAlbums[albumID]
	return ok
}
