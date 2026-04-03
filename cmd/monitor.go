package cmd

import (
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/spf13/cobra"

	"github.com/lidarr-utils/internal/lidarr"
	"github.com/lidarr-utils/internal/monitor"
)

var (
	artistIDStr           string
	allArtists            bool
	officialOnly          bool
	excludeSecondaryTypes []string
	maxInQueue            int
	delaySeconds          int
)

var monitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Intelligently monitor and search an artist's catalogue",
	Long: `Selects which albums to monitor for an artist, preferring albums over EPs
over singles. Skips releases whose tracks are fully covered by higher-priority
releases. Triggers rate-limited searches via Lidarr's command queue.`,
	RunE: runMonitor,
}

func init() {
	monitorCmd.Flags().StringVar(&artistIDStr, "artist-id", "", "Lidarr artist ID (numeric) or MusicBrainz ID (UUID)")
	monitorCmd.Flags().BoolVar(&allArtists, "all", false, "process all artists")
	monitorCmd.Flags().BoolVar(&officialOnly, "official-only", false, "only process albums with no secondary types")
	monitorCmd.Flags().StringSliceVar(&excludeSecondaryTypes, "exclude-secondary-types", nil, "secondary types to exclude (comma-separated)")
	monitorCmd.Flags().IntVar(&maxInQueue, "max-in-queue", 2, "max concurrent searches in Lidarr queue")
	monitorCmd.Flags().IntVar(&delaySeconds, "delay-seconds", 5, "delay after each search submission")
	rootCmd.AddCommand(monitorCmd)
}

func runMonitor(cmd *cobra.Command, args []string) error {
	if artistIDStr == "" && !allArtists {
		return fmt.Errorf("either --artist-id or --all is required")
	}

	cfg, err := getConfig(cmd)
	if err != nil {
		return err
	}

	if cmd.Flags().Changed("official-only") {
		cfg.Monitor.OfficialOnly = officialOnly
	}
	if cmd.Flags().Changed("exclude-secondary-types") {
		cfg.Monitor.ExcludeSecondaryTypes = excludeSecondaryTypes
	}
	if cmd.Flags().Changed("max-in-queue") {
		cfg.Monitor.Queue.MaxInQueue = maxInQueue
	}
	if cmd.Flags().Changed("delay-seconds") {
		cfg.Monitor.Queue.DelaySeconds = delaySeconds
	}

	logFileHandle, err := setupLoggingFromConfig(cfg)
	if err != nil {
		return err
	}
	if logFileHandle != nil {
		defer logFileHandle.Close()
	}

	cfg.Print()
	fmt.Println()

	client := lidarr.NewClient(cfg.Lidarr.URL, cfg.Lidarr.APIKey)

	log.Println("Testing connection to Lidarr...")
	if err := client.TestConnection(); err != nil {
		return fmt.Errorf("failed to connect to Lidarr: %w", err)
	}
	log.Println("Successfully connected to Lidarr")

	queueCfg := monitor.QueueConfig{
		MaxInQueue:   cfg.Monitor.Queue.MaxInQueue,
		DelaySeconds: cfg.Monitor.Queue.DelaySeconds,
	}

	mon := monitor.NewMonitor(
		client,
		cfg.App.DryRun,
		cfg.Monitor.OfficialOnly,
		cfg.Monitor.ExcludeSecondaryTypes,
		queueCfg,
	)

	var artistIDs []int
	if allArtists {
		artists, err := client.GetArtists()
		if err != nil {
			return fmt.Errorf("failed to get artists: %w", err)
		}
		for _, a := range artists {
			artistIDs = append(artistIDs, a.ID)
		}
		log.Printf("Processing all %d artists", len(artistIDs))
	} else {
		resolvedID, err := resolveArtistID(client, artistIDStr)
		if err != nil {
			return err
		}
		artistIDs = []int{resolvedID}
	}

	start := time.Now()
	stats, err := mon.Run(artistIDs)
	if err != nil {
		return fmt.Errorf("monitor failed: %w", err)
	}

	mon.PrintSummary(stats, time.Since(start))
	return nil
}

func resolveArtistID(client *lidarr.Client, idStr string) (int, error) {
	// Try as numeric Lidarr ID first
	if id, err := strconv.Atoi(idStr); err == nil {
		return id, nil
	}

	// Otherwise treat as MusicBrainz foreign artist ID
	log.Printf("Resolving foreign artist ID: %s", idStr)
	artists, err := client.GetArtists()
	if err != nil {
		return 0, fmt.Errorf("failed to get artists: %w", err)
	}

	for _, a := range artists {
		if a.ForeignID == idStr {
			log.Printf("Resolved to: %s (ID: %d)", a.ArtistName, a.ID)
			return a.ID, nil
		}
	}

	return 0, fmt.Errorf("artist with foreign ID %q not found in Lidarr", idStr)
}
