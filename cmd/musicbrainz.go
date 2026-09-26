package cmd

import (
	"github.com/benscobie/lidarr-utils/internal/config"
	"github.com/benscobie/lidarr-utils/internal/musicbrainz"
)

func newMusicBrainzClient(cfg *config.Config) *musicbrainz.Client {
	return musicbrainz.NewClientWithBaseURL(version, cfg.MusicBrainz.URL)
}
