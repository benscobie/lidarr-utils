package musicbrainz

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

var ErrLabelNotFound = errors.New("musicbrainz label not found")

type LabelReleaseGroup struct {
	ID             string
	Title          string
	PrimaryType    string
	SecondaryTypes []string
	ArtistCredits  []ArtistCredit
	Formats        []string
	Tracks         []Track
	SourceLabels   []string
}

type ArtistCredit struct {
	ArtistID string
	Name     string
}

type Track struct {
	ID          string
	RecordingID string
	Title       string
}

type LabelBrowseResult struct {
	ReleasesSeen int
	Groups       []LabelReleaseGroup
}

type labelBrowseResponse struct {
	ReleaseCount  int            `json:"release-count"`
	ReleaseOffset int            `json:"release-offset"`
	Releases      []labelRelease `json:"releases"`
}

type labelRelease struct {
	ReleaseGroup labelReleaseGroup `json:"release-group"`
	Media        []labelMedium     `json:"media"`
}

type labelReleaseGroup struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	PrimaryType    string         `json:"primary-type"`
	SecondaryTypes []string       `json:"secondary-types"`
	ArtistCredit   []artistCredit `json:"artist-credit"`
}

type labelMedium struct {
	Format string       `json:"format"`
	Tracks []labelTrack `json:"tracks"`
}

type labelTrack struct {
	ID        string         `json:"id"`
	Title     string         `json:"title"`
	Recording labelRecording `json:"recording"`
}

type labelRecording struct {
	ID string `json:"id"`
}

func (c *Client) LabelReleaseGroups(labelID string) (LabelBrowseResult, error) {
	var result LabelBrowseResult
	groups := make(map[string]*groupMerger)
	var groupOrder []string

	for offset := 0; ; {
		params := url.Values{
			"label":  {labelID},
			"inc":    {"release-groups+artist-credits+media+recordings"},
			"limit":  {"100"},
			"offset": {strconv.Itoa(offset)},
			"fmt":    {"json"},
		}
		requestURL := c.baseURL + "/release?" + params.Encode()

		resp, err := c.get(requestURL)
		if err != nil {
			return LabelBrowseResult{}, err
		}
		if resp.StatusCode == http.StatusNotFound {
			resp.Body.Close()
			return LabelBrowseResult{}, fmt.Errorf("%w: %s", ErrLabelNotFound, labelID)
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			return LabelBrowseResult{}, fmt.Errorf(
				"MusicBrainz API returned status %d for label %s",
				resp.StatusCode,
				labelID,
			)
		}

		var page labelBrowseResponse
		err = json.NewDecoder(resp.Body).Decode(&page)
		resp.Body.Close()
		if err != nil {
			return LabelBrowseResult{}, err
		}

		for _, release := range page.Releases {
			groupID := release.ReleaseGroup.ID
			if groupID == "" {
				continue
			}
			merger, ok := groups[groupID]
			if !ok {
				merger = newGroupMerger(release.ReleaseGroup, labelID)
				groups[groupID] = merger
				groupOrder = append(groupOrder, groupID)
			}
			merger.merge(release, labelID)
		}

		returned := len(page.Releases)
		result.ReleasesSeen += returned
		offset += returned
		if returned == 0 || offset >= page.ReleaseCount {
			break
		}
	}

	result.Groups = make([]LabelReleaseGroup, 0, len(groupOrder))
	for _, id := range groupOrder {
		result.Groups = append(result.Groups, groups[id].group)
	}
	return result, nil
}

type groupMerger struct {
	group          LabelReleaseGroup
	secondaryTypes map[string]struct{}
	artists        map[string]struct{}
	formats        map[string]struct{}
	tracks         map[string]struct{}
	labels         map[string]struct{}
}

func newGroupMerger(group labelReleaseGroup, labelID string) *groupMerger {
	merger := &groupMerger{
		group: LabelReleaseGroup{
			ID:          group.ID,
			Title:       group.Title,
			PrimaryType: group.PrimaryType,
		},
		secondaryTypes: make(map[string]struct{}),
		artists:        make(map[string]struct{}),
		formats:        make(map[string]struct{}),
		tracks:         make(map[string]struct{}),
		labels:         make(map[string]struct{}),
	}
	merger.mergeGroup(group)
	merger.addLabel(labelID)
	return merger
}

func (m *groupMerger) merge(release labelRelease, labelID string) {
	m.mergeGroup(release.ReleaseGroup)
	m.addLabel(labelID)
	for _, medium := range release.Media {
		m.addFormat(medium.Format)
		for _, track := range medium.Tracks {
			m.addTrack(track)
		}
	}
}

func (m *groupMerger) mergeGroup(group labelReleaseGroup) {
	for _, secondaryType := range group.SecondaryTypes {
		if _, exists := m.secondaryTypes[secondaryType]; exists {
			continue
		}
		m.secondaryTypes[secondaryType] = struct{}{}
		m.group.SecondaryTypes = append(m.group.SecondaryTypes, secondaryType)
	}
	for _, credit := range group.ArtistCredit {
		key := credit.Artist.ID
		if key == "" {
			key = strings.ToLower(strings.TrimSpace(credit.Artist.Name))
		}
		if key == "" {
			continue
		}
		if _, exists := m.artists[key]; exists {
			continue
		}
		m.artists[key] = struct{}{}
		m.group.ArtistCredits = append(m.group.ArtistCredits, ArtistCredit{
			ArtistID: credit.Artist.ID,
			Name:     credit.Artist.Name,
		})
	}
}

func (m *groupMerger) addFormat(format string) {
	if format == "" {
		return
	}
	if _, exists := m.formats[format]; exists {
		return
	}
	m.formats[format] = struct{}{}
	m.group.Formats = append(m.group.Formats, format)
}

func (m *groupMerger) addTrack(track labelTrack) {
	key := track.Recording.ID
	if key == "" {
		key = track.ID
	}
	if key == "" {
		return
	}
	if _, exists := m.tracks[key]; exists {
		return
	}
	m.tracks[key] = struct{}{}
	m.group.Tracks = append(m.group.Tracks, Track{
		ID:          track.ID,
		RecordingID: track.Recording.ID,
		Title:       track.Title,
	})
}

func (m *groupMerger) addLabel(labelID string) {
	if labelID == "" {
		return
	}
	if _, exists := m.labels[labelID]; exists {
		return
	}
	m.labels[labelID] = struct{}{}
	m.group.SourceLabels = append(m.group.SourceLabels, labelID)
}
