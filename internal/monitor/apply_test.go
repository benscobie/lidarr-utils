package monitor

import (
	"testing"

	"github.com/benscobie/lidarr-utils/internal/common"
	"github.com/benscobie/lidarr-utils/internal/lidarr"
)

func TestSelectRootFolderRequiresChoiceWhenMultipleAccessible(t *testing.T) {
	_, err := selectRootFolder([]lidarr.RootFolder{
		{Path: "/music-a", Accessible: true},
		{Path: "/music-b", Accessible: true},
	}, "")
	if err == nil {
		t.Fatal("expected explicit root folder requirement")
	}
}

func TestSelectRootFolderMatchesNormalizedConfiguredPath(t *testing.T) {
	root, err := selectRootFolder([]lidarr.RootFolder{
		{Path: "/unavailable", Accessible: false},
		{Path: "/music", Accessible: true},
	}, "/music/")
	if err != nil {
		t.Fatal(err)
	}
	if root.Path != "/music" {
		t.Fatalf("unexpected root: %#v", root)
	}
}

func TestApplyAlbumsDeduplicatesBatchIDs(t *testing.T) {
	client := &fakeMonitorClient{countingCatalogClient: newCountingCatalogClient()}
	stats, err := applyAlbums(client, nil, false, []common.Album{
		{ID: 1},
		{ID: 1},
		{ID: 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(client.monitorCalls) != 1 ||
		len(client.searchCalls) != 1 ||
		len(client.monitorCalls[0]) != 2 ||
		stats.AlbumsMonitored != 2 {
		t.Fatalf("unexpected batch application: monitor=%v search=%v stats=%#v",
			client.monitorCalls, client.searchCalls, stats)
	}
}
