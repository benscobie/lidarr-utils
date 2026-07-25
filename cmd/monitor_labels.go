package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/benscobie/lidarr-utils/internal/config"
	"github.com/benscobie/lidarr-utils/internal/monitor"
	"github.com/benscobie/lidarr-utils/internal/musicbrainz"
)

var labelMonitorCmd = &cobra.Command{
	Use:   "labels [label-mbid...]",
	Short: "Discover and monitor release groups from MusicBrainz labels",
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runWithConfigLogging(cmd, func(cfg *config.Config) error {
			return runLabelJob(cfg, args)
		})
	},
}

func init() {
	monitorCmd.AddCommand(labelMonitorCmd)
}

func labelIDsForRun(positional, configured []string) ([]string, error) {
	effective := configured
	if len(positional) > 0 {
		effective = positional
	}
	ids, err := config.NormalizeLabelIDs(effective)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one MusicBrainz label ID is required")
	}
	return ids, nil
}

func runLabelJob(cfg *config.Config, labelArgs []string) error {
	labelIDs, err := labelIDsForRun(labelArgs, cfg.Monitor.Labels.IDs)
	if err != nil {
		return err
	}
	client, st, err := monitorRuntime(cfg)
	if err != nil {
		return err
	}

	mon := monitor.NewMonitor(monitor.MonitorOptions{
		Client:   client,
		DryRun:   cfg.App.DryRun,
		MBClient: musicbrainz.NewClient(version),
		State:    st,
	})
	options := monitor.LabelOptions{
		IDs:                      labelIDs,
		AddMissingArtists:        cfg.Monitor.Labels.AddMissingArtists,
		RootFolder:               cfg.Monitor.Labels.RootFolder,
		Filters:                  cfg.Monitor.Labels.MonitorFilters,
		SkipFullyCoveredReleases: cfg.Monitor.Labels.SkipFullyCoveredReleases,
		DryRun:                   cfg.App.DryRun,
	}

	start := time.Now()
	stats, err := mon.RunLabels(options)
	if err != nil {
		return fmt.Errorf("label monitor failed: %w", err)
	}
	mon.PrintLabelSummary(stats, time.Since(start))
	return nil
}
