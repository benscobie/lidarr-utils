package monitor

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/benscobie/lidarr-utils/internal/config"
	"github.com/benscobie/lidarr-utils/internal/lidarr"
	"github.com/benscobie/lidarr-utils/internal/musicbrainz"
)

func TestPlanLabelsLogsDetailedCandidateDecisions(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.artists = []lidarr.Artist{{
		ID:         1,
		ArtistName: "Existing Artist",
		ForeignID:  "existing-artist",
	}}
	catalog.albumsByArtist[1] = []lidarr.Album{
		{
			ID:             10,
			ArtistID:       1,
			Title:          "Existing Album",
			AlbumType:      "Album",
			ForeignAlbumID: "rg-existing",
		},
		{
			ID:             11,
			ArtistID:       1,
			Title:          "Covered Single",
			AlbumType:      "Single",
			ForeignAlbumID: "rg-covered",
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
	catalog.tracks[10] = []lidarr.Track{{
		Title:              "Shared Track",
		ForeignRecordingID: "recording-shared",
	}}
	catalog.tracks[11] = []lidarr.Track{{
		Title:              "Shared Track",
		ForeignRecordingID: "recording-shared",
	}}
	catalog.tracks[12] = []lidarr.Track{{
		Title:              "Monitored Track",
		ForeignRecordingID: "recording-monitored",
	}}
	catalog.lookups["rg-new"] = []lidarr.Album{
		lookupAlbum("rg-new", "Album", "existing-artist"),
	}
	catalog.lookups["rg-missing"] = []lidarr.Album{
		lookupAlbum("rg-missing", "Album", "missing-artist"),
	}
	catalog.lookups["rg-new"][0].Title = "New Album"
	catalog.lookups["rg-new"][0].Artist.ArtistName = "Existing Artist"
	catalog.lookups["rg-missing"][0].Title = "Missing Album"
	catalog.lookups["rg-missing"][0].Artist.ArtistName = "Missing Artist"

	existing := labelGroup("rg-existing", "Album", "existing-artist", "recording-existing")
	covered := labelGroup("rg-covered", "Single", "existing-artist", "recording-shared")
	covered.Title = "Covered Single"
	covered.Tracks[0].Title = "Shared Track"
	monitored := labelGroup("rg-monitored", "Album", "existing-artist", "recording-monitored")
	newAlbum := labelGroup("rg-new", "Album", "existing-artist", "recording-new")
	missingAlbum := labelGroup("rg-missing", "Album", "missing-artist", "recording-missing")
	live := labelGroup("rg-live", "Album", "existing-artist", "recording-live")
	live.Title = "Live Release"
	live.SecondaryTypes = []string{"Live"}
	live.ArtistCredits[0].Name = "Existing Artist"

	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {
			Groups: []musicbrainz.LabelReleaseGroup{
				existing,
				covered,
				monitored,
				newAlbum,
				missingAlbum,
				live,
			},
		},
	}}
	mon := NewMonitor(MonitorOptions{Client: client, MBClient: mb})

	output := captureMonitorLogs(t, func() {
		_, err := mon.PlanLabels(LabelOptions{
			IDs:            []string{"label"},
			MissingArtists: config.MissingArtistsConfig{Enabled: true},
			Policy: config.ReleaseSelectionPolicy{
				IncludeSecondaryTypes:    true,
				ExcludeSecondaryTypes:    []string{"Live"},
				VariousArtists:           config.VariousArtistsInclude,
				CompilationSingles:       config.CompilationSinglesInclude,
				SkipFullyCoveredReleases: true,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	for _, expected := range []string{
		`Exclude: Existing Artist - Live Release (Album) — excluded secondary type "Live"`,
		"Processing label artist 1/2: Existing Artist",
		"Selected existing album: Existing Album (Album)",
		"Selected album to add: New Album (Album)",
		"Already monitored: Already Monitored (Album)",
		"Skip Single: Covered Single ('Shared Track' found in album 'Existing Album')",
		"Processing label artist 2/2: Missing Artist",
		"Selected album to add: Missing Album (Album) — missing artist will be added",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("missing log entry %q in:\n%s", expected, output)
		}
	}
}

func captureMonitorLogs(t *testing.T, run func()) string {
	t.Helper()
	var output bytes.Buffer
	previousWriter := log.Writer()
	previousFlags := log.Flags()
	previousPrefix := log.Prefix()
	log.SetOutput(&output)
	log.SetFlags(0)
	log.SetPrefix("")
	defer func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
		log.SetPrefix(previousPrefix)
	}()

	run()
	return output.String()
}

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
		LabelOptions{IDs: []string{"label"}, Policy: testSelectionPolicy(true)},
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
		LabelOptions{IDs: []string{"one", "two", "one"}, MissingArtists: config.MissingArtistsConfig{Enabled: true}},
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
		LabelOptions{IDs: []string{"label"}, Policy: testSelectionPolicy(true)},
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
		LabelOptions{IDs: []string{"label"}, MissingArtists: config.MissingArtistsConfig{Enabled: false}},
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
			IDs:            []string{"label"},
			MissingArtists: config.MissingArtistsConfig{Enabled: true},
			Policy: config.ReleaseSelectionPolicy{
				IncludeSecondaryTypes: true,
				VariousArtists:        config.VariousArtistsExclude,
				CompilationSingles:    config.CompilationSinglesInclude,
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

func TestRunLabelsDryRunDoesNotAddOrMutate(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.artists = []lidarr.Artist{{ID: 1, ArtistName: "Existing", ForeignID: "existing"}}
	catalog.albumsByArtist[1] = []lidarr.Album{{
		ID:             10,
		ArtistID:       1,
		Title:          "Existing Album",
		AlbumType:      "Album",
		ForeignAlbumID: "rg-existing",
	}}
	catalog.lookups["rg-missing"] = []lidarr.Album{
		lookupAlbum("rg-missing", "Album", "missing"),
	}
	client := &fakeMonitorClient{
		countingCatalogClient: catalog,
		roots: []lidarr.RootFolder{{
			Path:                     "/music",
			Accessible:               true,
			DefaultQualityProfileID:  2,
			DefaultMetadataProfileID: 3,
		}},
	}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {
			Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-existing", "Album", "existing", "recording-1"),
				labelGroup("rg-missing", "Album", "missing", "recording-2"),
			},
		},
	}}
	mon := NewMonitor(MonitorOptions{Client: client, MBClient: mb, DryRun: true})

	stats, err := mon.RunLabels(LabelOptions{
		IDs:            []string{"label"},
		MissingArtists: config.MissingArtistsConfig{Enabled: true},
		DryRun:         true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.addRequests) != 0 ||
		len(client.monitorCalls) != 0 ||
		len(client.searchCalls) != 0 {
		t.Fatalf("dry run mutated Lidarr: %#v", client)
	}
	if stats.ArtistsAdded != 1 ||
		stats.AlbumsAdded != 1 ||
		stats.AlbumsMonitored != 2 ||
		stats.SearchesSubmitted != 2 {
		t.Fatalf("unexpected dry-run stats: %#v", stats)
	}
}

func TestRunLabelsDryRunValidatesAmbiguousRootBeforeProcessing(t *testing.T) {
	catalog := newCountingCatalogClient()
	client := &fakeMonitorClient{
		countingCatalogClient: catalog,
		roots: []lidarr.RootFolder{
			{Path: "/one", Accessible: true},
			{Path: "/two", Accessible: true},
		},
	}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{}}
	mon := NewMonitor(MonitorOptions{Client: client, MBClient: mb, DryRun: true})

	_, err := mon.RunLabels(LabelOptions{
		IDs:            []string{"label"},
		MissingArtists: config.MissingArtistsConfig{Enabled: true},
		DryRun:         true,
	})
	if err == nil {
		t.Fatal("expected ambiguous root validation error")
	}
	if len(client.addRequests) != 0 ||
		len(client.monitorCalls) != 0 ||
		len(client.searchCalls) != 0 {
		t.Fatalf("validation failure mutated Lidarr: %#v", client)
	}
}

func TestRunLabelsCreatesMissingArtistOnceAndBatchAppliesAlbums(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.lookups["rg-one"] = []lidarr.Album{lookupAlbum("rg-one", "Album", "missing")}
	catalog.lookups["rg-two"] = []lidarr.Album{lookupAlbum("rg-two", "Album", "missing")}
	client := &fakeMonitorClient{
		countingCatalogClient: catalog,
		roots: []lidarr.RootFolder{{
			Path:                     "/music",
			Accessible:               true,
			DefaultQualityProfileID:  2,
			DefaultMetadataProfileID: 3,
			DefaultTags:              []int{4},
		}},
		nextAlbumID: 40,
	}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {
			Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-one", "Album", "missing", "recording-1"),
				labelGroup("rg-two", "Album", "missing", "recording-2"),
			},
		},
	}}
	mon := NewMonitor(MonitorOptions{Client: client, MBClient: mb})

	stats, err := mon.RunLabels(LabelOptions{
		IDs:            []string{"label"},
		MissingArtists: config.MissingArtistsConfig{Enabled: true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.addRequests) != 2 ||
		client.addRequests[0].Artist.AddOptions == nil ||
		client.addRequests[1].Artist.AddOptions != nil {
		t.Fatalf("unexpected artist creation payloads: %#v", client.addRequests)
	}
	if len(client.monitorCalls) != 1 ||
		len(client.monitorCalls[0]) != 2 ||
		len(client.searchCalls) != 1 ||
		len(client.searchCalls[0]) != 2 {
		t.Fatalf("expected one batch mutation: monitor=%v search=%v", client.monitorCalls, client.searchCalls)
	}
	if stats.ArtistsAdded != 1 || stats.AlbumsAdded != 2 || stats.AlbumsMonitored != 2 {
		t.Fatalf("unexpected stats: %#v", stats)
	}
}

func TestRunLabelsExistingArtistPayloadPreservesSettings(t *testing.T) {
	existing := lidarr.Artist{
		ID:                7,
		ArtistName:        "Existing",
		ForeignID:         "existing",
		Path:              "/music/Existing",
		QualityProfileID:  2,
		MetadataProfileID: 3,
		Monitored:         true,
		MonitorNewItems:   "new",
		Tags:              []int{4},
	}
	catalog := newCountingCatalogClient()
	catalog.artists = []lidarr.Artist{existing}
	catalog.lookups["rg-new"] = []lidarr.Album{lookupAlbum("rg-new", "Album", "existing")}
	client := &fakeMonitorClient{countingCatalogClient: catalog, nextAlbumID: 40}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {
			Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-new", "Album", "existing", "recording-1"),
			},
		},
	}}
	mon := NewMonitor(MonitorOptions{Client: client, MBClient: mb})

	if _, err := mon.RunLabels(LabelOptions{IDs: []string{"label"}}); err != nil {
		t.Fatal(err)
	}
	if len(client.addRequests) != 1 || client.addRequests[0].Artist == nil {
		t.Fatalf("expected one add request: %#v", client.addRequests)
	}
	got := client.addRequests[0].Artist
	if got.ForeignID != existing.ForeignID ||
		got.Path != existing.Path ||
		got.QualityProfileID != existing.QualityProfileID ||
		got.MetadataProfileID != existing.MetadataProfileID ||
		got.AddOptions != nil {
		t.Fatalf("existing artist settings were not preserved: %#v", got)
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
			IDs:            []string{"label"},
			MissingArtists: config.MissingArtistsConfig{Enabled: true},
			Policy:         testSelectionPolicy(skipCovered),
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
