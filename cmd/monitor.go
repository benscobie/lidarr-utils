package cmd

import (
	"fmt"
	"log"

	"github.com/spf13/cobra"

	"github.com/benscobie/lidarr-utils/internal/config"
	"github.com/benscobie/lidarr-utils/internal/lidarr"
	"github.com/benscobie/lidarr-utils/internal/state"
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Monitor artist catalogues or MusicBrainz labels",
}

func init() {
	rootCmd.AddCommand(monitorCmd)
}

func runWithConfigLogging(
	cmd *cobra.Command,
	job func(*config.Config) error,
) error {
	cfg, err := getConfig(cmd)
	if err != nil {
		return err
	}
	return runWithLoadedConfigLogging(cfg, job)
}

func runWithLoadedConfigLogging(
	cfg *config.Config,
	job func(*config.Config) error,
) error {
	logFileHandle, err := setupLoggingFromConfig(cfg)
	if err != nil {
		return err
	}
	if logFileHandle != nil {
		defer logFileHandle.Close()
	}
	cfg.Print()
	fmt.Println()
	return job(cfg)
}

func monitorRuntime(cfg *config.Config) (*lidarr.Client, *state.State, error) {
	st, err := state.Load(cfg.App.StateFile)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to load state: %w", err)
	}
	log.Printf("Loaded state: %d previously monitored albums", len(st.MonitoredAlbums))

	client := lidarr.NewClient(cfg.Lidarr.URL, cfg.Lidarr.APIKey)
	log.Println("Testing connection to Lidarr...")
	if err := client.TestConnection(); err != nil {
		return nil, nil, fmt.Errorf("failed to connect to Lidarr: %w", err)
	}
	log.Println("Successfully connected to Lidarr")
	return client, st, nil
}
