package monitor

import (
	"bytes"
	"errors"
	"io"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

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
		`Exclude: Existing Artist - Live Release (Album) — excluded secondary type \"Live\"`,
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
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})))
	defer slog.SetDefault(previous)

	run()
	return output.String()
}

func captureMonitorStdout(t *testing.T, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = previous
		_ = reader.Close()
		_ = writer.Close()
	}()

	run()
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(output)
}

func TestPrintLabelSummaryIncludesRelationshipCounters(t *testing.T) {
	mon := NewMonitor(MonitorOptions{})
	output := captureMonitorStdout(t, func() {
		mon.PrintLabelSummary(&LabelStats{
			RelationshipChecks:    3,
			RelationshipCacheHits: 1,
			RelationshipFailures:  1,
			Warnings:              1,
		}, time.Second)
	})

	for _, expected := range []string{
		"Compilation relationship checks: 3",
		"Compilation relationship cache hits: 1",
		"Compilation relationship failures: 1",
		"Warnings: 1",
	} {
		if !strings.Contains(output, expected) {
			t.Errorf("summary missing %q:\n%s", expected, output)
		}
	}
}

type fakeLabelMBClient struct {
	results        map[string]musicbrainz.LabelBrowseResult
	errors         map[string]error
	vaSources      map[string]string
	vaErrors       map[string]error
	vaCalls        int
	vaCallsByGroup map[string]int
}

func (f *fakeLabelMBClient) LabelReleaseGroups(labelID string) (musicbrainz.LabelBrowseResult, error) {
	if err := f.errors[labelID]; err != nil {
		return musicbrainz.LabelBrowseResult{}, err
	}
	return f.results[labelID], nil
}

func TestPlanLabelsContinuesAfterOneLabelFails(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.lookups["rg-success"] = []lidarr.Album{
		lookupAlbum("rg-success", "Album", "artist-success"),
	}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{
		errors: map[string]error{"failed": errors.New("browse failed")},
		results: map[string]musicbrainz.LabelBrowseResult{
			"success": {Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-success", "Album", "artist-success", "recording-success"),
			}},
		},
	}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
		IDs:            []string{"failed", "success"},
		MissingArtists: config.MissingArtistsConfig{Enabled: true},
		Policy:         testSelectionPolicy(true),
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Stats.LabelsProcessed != 1 || plan.Stats.Failures != 1 || len(plan.Selected) != 1 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func TestPlanLabelsFailsWhenEveryLabelFails(t *testing.T) {
	client := &fakeMonitorClient{countingCatalogClient: newCountingCatalogClient()}
	mb := &fakeLabelMBClient{errors: map[string]error{
		"first":  errors.New("first failed"),
		"second": errors.New("second failed"),
	}}

	_, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
		IDs: []string{"first", "second"},
	})
	if err == nil || !strings.Contains(err.Error(), "all 2 MusicBrainz label lookups failed") {
		t.Fatalf("expected all-label failure, got %v", err)
	}
}

func TestPlanLabelsSuccessfulEmptyLabelIsNotFailure(t *testing.T) {
	client := &fakeMonitorClient{countingCatalogClient: newCountingCatalogClient()}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"empty": {},
	}}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
		IDs: []string{"empty"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Stats.LabelsProcessed != 1 || plan.Stats.Failures != 0 {
		t.Fatalf("unexpected plan: %#v", plan)
	}
}

func (f *fakeLabelMBClient) VACompilationSource(releaseGroupID string) (string, error) {
	f.vaCalls++
	if f.vaCallsByGroup == nil {
		f.vaCallsByGroup = make(map[string]int)
	}
	f.vaCallsByGroup[releaseGroupID]++
	if err := f.vaErrors[releaseGroupID]; err != nil {
		return "", err
	}
	return f.vaSources[releaseGroupID], nil
}

func TestPlanLabelsVAOnlyRejectsCompleteNonVACreditsBeforeLookup(t *testing.T) {
	catalog := newCountingCatalogClient()
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {Groups: []musicbrainz.LabelReleaseGroup{
			labelGroup("rg-ordinary", "Single", "ordinary-artist", "recording-1"),
		}},
	}}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
		IDs:            []string{"label"},
		MissingArtists: config.MissingArtistsConfig{Enabled: true},
		Policy: config.ReleaseSelectionPolicy{
			IncludeSecondaryTypes: true,
			VariousArtists:        config.VariousArtistsOnly,
			CompilationSingles:    config.CompilationSinglesExclude,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Stats.GroupsFiltered != 1 || len(plan.Selected) != 0 ||
		catalog.lookupCalls["rg-ordinary"] != 0 || len(catalog.trackCalls) != 0 || mb.vaCalls != 0 {
		t.Fatalf("unexpected plan: %#v, lookup=%v tracks=%v relationships=%d", plan, catalog.lookupCalls, catalog.trackCalls, mb.vaCalls)
	}
}

func TestPlanLabelsMixedVACreditUsesLidarrOwner(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.lookups["rg-mixed"] = []lidarr.Album{lookupAlbum("rg-mixed", "Album", "ordinary-artist")}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	group := labelGroup("rg-mixed", "Album", "ordinary-artist", "recording-1")
	group.ArtistCredits = append(group.ArtistCredits, musicbrainz.ArtistCredit{
		ArtistID: musicbrainz.VariousArtistsID,
		Name:     "Various Artists",
	})
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {Groups: []musicbrainz.LabelReleaseGroup{group}},
	}}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
		IDs:            []string{"label"},
		MissingArtists: config.MissingArtistsConfig{Enabled: true},
		Policy: config.ReleaseSelectionPolicy{
			IncludeSecondaryTypes: true,
			VariousArtists:        config.VariousArtistsOnly,
			CompilationSingles:    config.CompilationSinglesInclude,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if catalog.lookupCalls["rg-mixed"] != 1 || plan.Stats.GroupsFiltered != 1 || len(plan.Selected) != 0 {
		t.Fatalf("mixed credit should defer to Lidarr owner: lookup=%v plan=%#v", catalog.lookupCalls, plan)
	}
}

func TestPlanLabelsUsesLookupVAOwnerForSelection(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.lookups["rg-lookup-va"] = []lidarr.Album{lookupAlbum("rg-lookup-va", "Album", musicbrainz.VariousArtistsID)}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	group := labelGroup("rg-lookup-va", "Album", "ordinary-artist", "recording-1")
	group.ArtistCredits = append(group.ArtistCredits,
		musicbrainz.ArtistCredit{ArtistID: musicbrainz.VariousArtistsID, Name: "Various Artists"},
		musicbrainz.ArtistCredit{Name: "incomplete credit"},
	)
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {Groups: []musicbrainz.LabelReleaseGroup{group}},
	}}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
		IDs:            []string{"label"},
		MissingArtists: config.MissingArtistsConfig{Enabled: true},
		Policy: config.ReleaseSelectionPolicy{
			IncludeSecondaryTypes: true,
			VariousArtists:        config.VariousArtistsOnly,
			CompilationSingles:    config.CompilationSinglesInclude,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if catalog.lookupCalls["rg-lookup-va"] != 1 ||
		len(plan.Selected) != 1 ||
		!plan.Selected[0].Album.IsVariousArtists ||
		plan.Selected[0].OwnerForeignID != musicbrainz.VariousArtistsID {
		t.Fatalf("lookup VA owner was not selected: lookup=%v plan=%#v", catalog.lookupCalls, plan)
	}
}

func TestPlanLabelsExactLookupWithoutOwnerDoesNotUseVACredit(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.lookups["rg-ownerless"] = []lidarr.Album{{
		ForeignAlbumID: "rg-ownerless",
		Title:          "Ownerless Lookup",
		AlbumType:      "Album",
		Releases:       []lidarr.Release{{Format: "Digital Media"}},
	}}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {Groups: []musicbrainz.LabelReleaseGroup{
			labelGroup(
				"rg-ownerless",
				"Album",
				musicbrainz.VariousArtistsID,
				"recording-1",
			),
		}},
	}}

	var plan LabelPlan
	output := captureMonitorLogs(t, func() {
		var err error
		plan, err = NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
			IDs:            []string{"label"},
			MissingArtists: config.MissingArtistsConfig{Enabled: true},
			Policy: config.ReleaseSelectionPolicy{
				IncludeSecondaryTypes: true,
				VariousArtists:        config.VariousArtistsOnly,
				CompilationSingles:    config.CompilationSinglesInclude,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
	})

	if catalog.lookupCalls["rg-ownerless"] != 1 || plan.Stats.Failures != 1 || len(plan.Selected) != 0 {
		t.Fatalf("ownerless lookup should fail planning: lookup=%v plan=%#v", catalog.lookupCalls, plan)
	}
	if !strings.Contains(output, "Lidarr lookup did not identify an artist for rg-ownerless") {
		t.Fatalf("missing owner-resolution diagnostic:\n%s", output)
	}
}

func TestPlanLabelsVAOwnerObeysMissingArtistsPolicy(t *testing.T) {
	t.Run("disabled skips canonical VA credit before lookup", func(t *testing.T) {
		catalog := newCountingCatalogClient()
		client := &fakeMonitorClient{countingCatalogClient: catalog}
		mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
			"label": {Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-va", "Album", musicbrainz.VariousArtistsID, "recording-1"),
			}},
		}}

		plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
			IDs: []string{"label"},
			Policy: config.ReleaseSelectionPolicy{
				IncludeSecondaryTypes: true,
				VariousArtists:        config.VariousArtistsOnly,
				CompilationSingles:    config.CompilationSinglesInclude,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if plan.Stats.MissingArtistSkipped != 1 || catalog.lookupCalls["rg-va"] != 0 || len(plan.Selected) != 0 {
			t.Fatalf("missing VA owner should skip before lookup: lookup=%v plan=%#v", catalog.lookupCalls, plan)
		}
	})

	t.Run("enabled selects exact missing VA owner", func(t *testing.T) {
		catalog := newCountingCatalogClient()
		catalog.lookups["rg-va"] = []lidarr.Album{lookupAlbum("rg-va", "Album", musicbrainz.VariousArtistsID)}
		client := &fakeMonitorClient{countingCatalogClient: catalog}
		mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
			"label": {Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-va", "Album", musicbrainz.VariousArtistsID, "recording-1"),
			}},
		}}

		plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
			IDs:            []string{"label"},
			MissingArtists: config.MissingArtistsConfig{Enabled: true},
			Policy: config.ReleaseSelectionPolicy{
				IncludeSecondaryTypes: true,
				VariousArtists:        config.VariousArtistsOnly,
				CompilationSingles:    config.CompilationSinglesInclude,
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		if catalog.lookupCalls["rg-va"] != 1 || len(plan.Selected) != 1 || !plan.Selected[0].Album.IsVariousArtists {
			t.Fatalf("missing VA owner should be selectable when enabled: lookup=%v plan=%#v", catalog.lookupCalls, plan)
		}
	})
}

func TestPlanLabelsCompilationSinglesIncludeDoesNoRelationshipWork(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.lookups["rg-single"] = []lidarr.Album{lookupAlbum("rg-single", "Single", "missing-artist")}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {Groups: []musicbrainz.LabelReleaseGroup{
			labelGroup("rg-single", "Single", "missing-artist", "recording-1"),
		}},
	}}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
		IDs:            []string{"label"},
		MissingArtists: config.MissingArtistsConfig{Enabled: true},
		Policy: config.ReleaseSelectionPolicy{
			IncludeSecondaryTypes: true,
			VariousArtists:        config.VariousArtistsInclude,
			CompilationSingles:    config.CompilationSinglesInclude,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Selected) != 1 || mb.vaCalls != 0 {
		t.Fatalf("eligible single should need no relationship work: plan=%#v calls=%v", plan, mb.vaCallsByGroup)
	}
}

func TestPlanLabelsCompilationSinglesExcludeChecksOnlyOtherwiseEligibleSingles(t *testing.T) {
	catalog := newCountingCatalogClient()
	vinyl := lookupAlbum("rg-vinyl", "Single", "missing-artist")
	vinyl.Releases = []lidarr.Release{{Format: "Vinyl"}}
	catalog.lookups["rg-vinyl"] = []lidarr.Album{vinyl}
	catalog.lookups["rg-album"] = []lidarr.Album{lookupAlbum("rg-album", "Album", "missing-artist")}
	catalog.lookups["rg-related"] = []lidarr.Album{lookupAlbum("rg-related", "Single", "missing-artist")}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	vinylGroup := labelGroup("rg-vinyl", "Single", "missing-artist", "recording-vinyl")
	vinylGroup.Formats = []string{"Vinyl"}
	mb := &fakeLabelMBClient{
		results: map[string]musicbrainz.LabelBrowseResult{
			"label": {Groups: []musicbrainz.LabelReleaseGroup{
				vinylGroup,
				labelGroup("rg-album", "Album", "missing-artist", "recording-album"),
				labelGroup("rg-related", "Single", "missing-artist", "recording-related"),
			}},
		},
		vaSources: map[string]string{"rg-related": "Compilation Album"},
	}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
		IDs:            []string{"label"},
		MissingArtists: config.MissingArtistsConfig{Enabled: true},
		Policy: config.ReleaseSelectionPolicy{
			IncludeSecondaryTypes: true,
			ExcludeFormats:        []string{"Vinyl"},
			VariousArtists:        config.VariousArtistsInclude,
			CompilationSingles:    config.CompilationSinglesExclude,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Selected) != 1 || plan.Selected[0].Album.ForeignAlbumID != "rg-album" ||
		mb.vaCallsByGroup["rg-related"] != 1 || mb.vaCallsByGroup["rg-vinyl"] != 0 ||
		mb.vaCallsByGroup["rg-album"] != 0 || len(catalog.trackCalls) != 0 {
		t.Fatalf("only eligible single should be checked: plan=%#v calls=%v tracks=%v", plan, mb.vaCallsByGroup, catalog.trackCalls)
	}
}

func TestPlanLabelsCompilationSinglesDefersDiscoveredTrackAttachment(t *testing.T) {
	group := labelGroup("rg-single", "Single", "missing-artist", "recording-1")
	if album := albumFromLabelGroup(group); len(album.Tracks) != 0 {
		t.Fatalf("discovered tracks must remain deferred until the album is policy-eligible: %#v", album.Tracks)
	}
}

func TestPlanLabelsCompilationRelationshipFailureWarnsOnceAndFailsOpen(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.lookups["rg-single"] = []lidarr.Album{lookupAlbum("rg-single", "Single", "missing-artist")}
	client := &fakeMonitorClient{countingCatalogClient: catalog}
	mb := &fakeLabelMBClient{
		results: map[string]musicbrainz.LabelBrowseResult{
			"label": {Groups: []musicbrainz.LabelReleaseGroup{
				labelGroup("rg-single", "Single", "missing-artist", "recording-1"),
			}},
		},
		vaErrors: map[string]error{"rg-single": errors.New("unavailable")},
	}

	plan, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).PlanLabels(LabelOptions{
		IDs:            []string{"label"},
		MissingArtists: config.MissingArtistsConfig{Enabled: true},
		Policy: config.ReleaseSelectionPolicy{
			IncludeSecondaryTypes: true,
			VariousArtists:        config.VariousArtistsInclude,
			CompilationSingles:    config.CompilationSinglesExclude,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Selected) != 1 || plan.Stats.Failures != 1 || plan.Stats.Warnings != 1 ||
		plan.Stats.RelationshipChecks != 1 || plan.Stats.RelationshipFailures != 1 {
		t.Fatalf("relationship errors should fail open and be counted: %#v", plan)
	}
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

func TestRunLabelsReturnsAddAlbumFailure(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.artists = []lidarr.Artist{{
		ID:         1,
		ArtistName: "Existing Artist",
		ForeignID:  "existing-artist",
	}}
	catalog.lookups["rg-new"] = []lidarr.Album{
		lookupAlbum("rg-new", "Album", "existing-artist"),
	}
	client := &fakeMonitorClient{
		countingCatalogClient: catalog,
		addErr:                errors.New("add failed"),
	}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {Groups: []musicbrainz.LabelReleaseGroup{
			labelGroup("rg-new", "Album", "existing-artist", "recording-new"),
		}},
	}}

	stats, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).RunLabels(
		LabelOptions{IDs: []string{"label"}},
	)
	if err == nil || !strings.Contains(err.Error(), "rg-new") || !strings.Contains(err.Error(), "add failed") {
		t.Fatalf("expected contextual add failure, got %v", err)
	}
	if stats == nil || stats.AlbumsAdded != 0 || stats.ArtistsAdded != 0 ||
		len(client.monitorCalls) != 0 || len(client.searchCalls) != 0 {
		t.Fatalf("add failure recorded false success: stats=%#v monitor=%v search=%v", stats, client.monitorCalls, client.searchCalls)
	}
}

func TestRunLabelsReturnsBatchApplyFailure(t *testing.T) {
	catalog := newCountingCatalogClient()
	catalog.artists = []lidarr.Artist{{
		ID:         1,
		ArtistName: "Existing Artist",
		ForeignID:  "existing-artist",
	}}
	catalog.albumsByArtist[1] = []lidarr.Album{{
		ID:             10,
		ArtistID:       1,
		Artist:         &catalog.artists[0],
		Title:          "Existing Album",
		AlbumType:      "Album",
		ForeignAlbumID: "rg-existing",
	}}
	client := &fakeMonitorClient{
		countingCatalogClient: catalog,
		monitorErr:            errors.New("monitor failed"),
	}
	mb := &fakeLabelMBClient{results: map[string]musicbrainz.LabelBrowseResult{
		"label": {Groups: []musicbrainz.LabelReleaseGroup{
			labelGroup("rg-existing", "Album", "existing-artist", "recording-existing"),
		}},
	}}

	_, err := NewMonitor(MonitorOptions{Client: client, MBClient: mb}).RunLabels(
		LabelOptions{IDs: []string{"label"}},
	)
	if err == nil || !strings.Contains(err.Error(), "monitor failed") {
		t.Fatalf("expected monitor failure, got %v", err)
	}
	if len(client.searchCalls) != 0 {
		t.Fatalf("monitor failure should not submit a search: %v", client.searchCalls)
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
