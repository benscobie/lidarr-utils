package monitor

import (
	"testing"

	"github.com/benscobie/lidarr-utils/internal/config"
	"github.com/benscobie/lidarr-utils/internal/lidarr"
	"github.com/benscobie/lidarr-utils/internal/musicbrainz"
)

type fakeLabelMBClient struct {
	results map[string]musicbrainz.LabelBrowseResult
	vaCalls int
}

func (f *fakeLabelMBClient) LabelReleaseGroups(labelID string) (musicbrainz.LabelBrowseResult, error) {
	return f.results[labelID], nil
}

func (f *fakeLabelMBClient) VACompilationSource(_ string) (string, error) {
	f.vaCalls++
	return "", nil
}

func TestPlanLabelsUsesNonLabelCatalogueForCoverage(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.artists = []lidarr.Artist{{ID: 1, ArtistName: "Artist", ForeignID: "artist-a"}}
	catalog.albumsByArtist[1] = []lidarr.Album{
		{
			ID:             10,
			ArtistID:       1,
			Title:          "Non-label Album",
			AlbumType:      "Album",
			ForeignAlbumID: "rg-album",
		},
		{
			ID:             11,
			ArtistID:       1,
			Title:          "Label Single",
			AlbumType:      "Single",
			ForeignAlbumID: "rg-single",
		},
		{
			ID:             12,
			ArtistID:       1,
			Title:          "Already Monitored",
			AlbumType:      "Album",
			ForeignAlbumID: "rg-monitored",
			Monitored:      true,
		},
	}
	catalog.tracks[10] = []lidarr.Track{{ForeignRecordingID: "recording-1"}}
	catalog.tracks[11] = []lidarr.Track{{ForeignRecordingID: "recording-1"}}
	catalog.tracks[12] = []lidarr.Track{{ForeignRecordingID: "recording-2"}}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {
			ReleasesSeen: 2,
			Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-single", "Single", "artist-a", "recording-1"),
				labelGroup("rg-monitored", "Album", "artist-a", "recording-2"),
			},
		},
	}}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(
		LabelOptions{IDs: []string{"label"}, SkipFullyCoveredReleases: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Selected) != 0 ||
		plan.Stats.GroupsCoverageSkipped != 1 ||
		plan.Stats.AlreadyMonitored != 1 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
	if catalog.lookupCalls["rg-single"] != 0 ||
		catalog.lookupCalls["rg-monitored"] != 0 {
		t.Fatalf("catalogue matches should bypass lookup: %v", catalog.lookupCalls)
	}
}

func TestPlanLabelsMissingArtistUsesOnlyDiscoveredLabelCatalogue(t *testing.T) {
	plan, _ := planMissingArtistCoverage(t, true)
	if len(plan.Selected) != 1 || plan.Selected[0].Album.ForeignAlbumID != "rg-ep" {
		t.Fatalf("expected only EP selected: %#v", plan.Selected)
	}
	if plan.Stats.GroupsCoverageSkipped != 1 {
		t.Fatalf("expected redundant single to be counted: %#v", plan.Stats)
	}
}

func TestPlanLabelsCoverageCanBeDisabled(t *testing.T) {
	plan, _ := planMissingArtistCoverage(t, false)
	if len(plan.Selected) != 2 || plan.Stats.GroupsCoverageSkipped != 0 {
		t.Fatalf("expected both groups selected: %#v", plan)
	}
}

func TestPlanLabelsDeduplicatesRemoteWork(t *testing.T) {
	catalog := newCountingCatalogClient()
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	firstEdition := labelGroup("rg-album", "Album", "missing-artist", "recording-1")
	secondEdition := labelGroup("rg-album", "Album", "missing-artist", "recording-2")
	catalog.lookups["rg-album"] = []lidarr.Album{lookupAlbum(
		"rg-album",
		"Album",
		"missing-artist",
	)}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"one": {ReleasesSeen: 1, Groups: []musicbrainz.LabelReleaseGroup{firstEdition}},
		"two": {ReleasesSeen: 1, Groups: []musicbrainz.LabelReleaseGroup{secondEdition}},
	}}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(
		LabelOptions{IDs: []string{"one", "two", "one"}, AddMissingArtists: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.lookupCalls["rg-album"] != 1 ||
		plan.Stats.LabelsProcessed != 2 ||
		plan.Stats.GroupsDiscovered != 1 ||
		len(plan.Selected) != 1 ||
		len(plan.Selected[0].Album.Tracks) != 2 {
		t.Fatalf("remote work was not deduplicated: calls=%v plan=%#v", catalog.lookupCalls, plan)
	}
}

func TestPlanLabelsInjectsMissingAlbumIntoExistingArtistCoverage(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.artists = []lidarr.Artist{{ID: 1, ArtistName: "Artist", ForeignID: "artist-a"}}
	catalog.lookups["rg-album"] = []lidarr.Album{lookupAlbum("rg-album", "Album", "artist-a")}
	catalog.lookups["rg-single"] = []lidarr.Album{lookupAlbum("rg-single", "Single", "artist-a")}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {
			ReleasesSeen: 2,
			Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-album", "Album", "artist-a", "recording-1"),
				labelGroup("rg-single", "Single", "artist-a", "recording-1"),
			},
		},
	}}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(
		LabelOptions{IDs: []string{"label"}, SkipFullyCoveredReleases: true},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Selected) != 1 || plan.Selected[0].Album.ForeignAlbumID != "rg-album" {
		t.Fatalf("synthetic album should cover the single: %#v", plan.Selected)
	}
}

func TestPlanLabelsSkipsLookupForAlbumAlreadyInCatalogue(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.artists = []lidarr.Artist{{ID: 1, ArtistName: "Artist", ForeignID: "artist-a"}}
	catalog.albumsByArtist[1] = []lidarr.Album{{
		ID:             10,
		ArtistID:       1,
		Title:          "Known Album",
		AlbumType:      "Album",
		ForeignAlbumID: "rg-known",
	}}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {
			Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-known", "Album", "artist-a", "recording-1"),
			},
		},
	}}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(
		LabelOptions{IDs: []string{"label"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.lookupCalls["rg-known"] != 0 || len(plan.Selected) != 1 {
		t.Fatalf("unexpected lookup or selection: calls=%v plan=%#v", catalog.lookupCalls, plan)
	}
}

func TestPlanLabelsSkipsAbsentArtistsBeforeLookupWhenAddingDisabled(t *testing.T) {
	catalog := newCountingCatalogClient()
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {
			Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-missing", "Album", "missing-artist", "recording-1"),
			},
		},
	}}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(
		LabelOptions{IDs: []string{"label"}, AddMissingArtists: false},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Stats.MissingArtistSkipped != 1 ||
		catalog.lookupCalls["rg-missing"] != 0 ||
		len(plan.Selected) != 0 {
		t.Fatalf("unexpected missing-artist plan: calls=%v plan=%#v", catalog.lookupCalls, plan)
	}
}

func TestPlanLabelsDirectVACreditNeedsNoRelationshipLookup(t *testing.T) {
	catalog := newCountingCatalogClient()
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {
			Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup(
					"rg-va",
					"Single",
					musicbrainz.VariousArtistsID,
					"recording-1",
				),
			},
		},
	}}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(
		LabelOptions{
			IDs:               []string{"label"},
			AddMissingArtists: true,
			Filters: config.MonitorFilters{
				ExcludeVAReleases: true,
			},
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Stats.GroupsFiltered != 1 || mb.vaCalls != 0 {
		t.Fatalf("unexpected VA work: calls=%d plan=%#v", mb.vaCalls, plan)
	}
}

func planMissingArtistCoverage(
	t *testing.T,
	skipCovered bool,
) (LabelPlan, *countingCatalogClient) {
	t.Helper()
	catalog := newCountingCatalogClient()
	catalog.lookups["rg-ep"] = []lidarr.Album{lookupAlbum("rg-ep", "EP", "missing-artist")}
	catalog.lookups["rg-single"] = []lidarr.Album{
		lookupAlbum("rg-single", "Single", "missing-artist"),
	}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {
			ReleasesSeen: 2,
			Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-ep", "EP", "missing-artist", "recording-1"),
				labelGroup("rg-single", "Single", "missing-artist", "recording-1"),
			},
		},
	}}
	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(
		LabelOptions{
			IDs:                      []string{"label"},
			AddMissingArtists:        true,
			SkipFullyCoveredReleases: skipCovered,
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	return plan, catalog
}

func labelGroup(
	id string,
	primaryType string,
	artistID string,
	recordingID string,
) musicbrainz.LabelReleaseGroup {
	return musicbrainz.LabelReleaseGroup{
		ID:          id,
		Title:       id,
		PrimaryType: primaryType,
		ArtistCredits: []musicbrainz.ArtistCredit{{
			ArtistID: artistID,
			Name:     artistID,
		}},
		Tracks: []musicbrainz.Track{{
			ID:          "track-" + recordingID,
			RecordingID: recordingID,
			Title:       recordingID,
		}},
	}
}

func lookupAlbum(id string, albumType string, artistID string) lidarr.Album {
	return lidarr.Album{
		ForeignAlbumID: id,
		Title:          id,
		AlbumType:      albumType,
		Artist: &lidarr.Artist{
			ForeignID:  artistID,
			ArtistName: artistID,
		},
		Releases: []lidarr.Release{{Format: "Digital Media"}},
	}
}
