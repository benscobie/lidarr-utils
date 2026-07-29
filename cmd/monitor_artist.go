package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/benscobie/lidarr-utils/internal/config"
	"github.com/benscobie/lidarr-utils/internal/monitor"
	"github.com/benscobie/lidarr-utils/internal/musicbrainz"
)

var artistMonitorCmd = &cobra.Command{
	Use:   "artist [lidarr-id-or-mbid...]",
	Short: "Monitor one or more artists, or all artists when no IDs are given",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWithConfigLogging(cmd, func(cfg *config.Config) error {
			return runArtistJob(cfg, args)
		})
	},
}

func init() {
	monitorCmd.AddCommand(artistMonitorCmd)
}

func runArtistJob(cfg *config.Config, artistRefs []string) error {
	client, st, err := monitorRuntime(cfg)
	if err != nil {
		return err
	}

	var mbClient *musicbrainz.Client
	if cfg.Monitor.Artists.ExcludeVAReleases {
		mbClient = musicbrainz.NewClient(version)
	}
	mon := monitor.NewMonitor(monitor.MonitorOptions{
		Client:                   client,
		DryRun:                   cfg.App.DryRun,
		Filters:                  cfg.Monitor.Artists.MonitorFilters,
		SkipFullyCoveredReleases: cfg.Monitor.Artists.SkipFullyCoveredReleases,
		MBClient:                 mbClient,
		State:                    st,
	})

	start := time.Now()
	var stats *monitor.Stats
	if len(artistRefs) == 0 {
		stats, err = mon.RunAllArtists()
	} else {
		stats, err = mon.RunArtists(artistRefs)
	}
	if err != nil {
		return fmt.Errorf("artist monitor failed: %w", err)
	}
	mon.PrintSummary(stats, time.Since(start))
	return nil
}
