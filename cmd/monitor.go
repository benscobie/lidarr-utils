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
	artistIDStrs          []string
	allArtists            bool
	officialOnly          bool
	excludeSecondaryTypes []string
	excludeFormats        []string
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
	monitorCmd.Flags().StringSliceVar(&artistIDStrs, "artist-id", nil, "Lidarr artist ID(s) — numeric or MusicBrainz UUID (repeatable, comma-separated)")
	monitorCmd.Flags().BoolVar(&allArtists, "all", false, "process all artists")
	monitorCmd.Flags().BoolVar(&officialOnly, "official-only", false, "only process albums with no secondary types")
	monitorCmd.Flags().StringSliceVar(&excludeSecondaryTypes, "exclude-secondary-types", nil, "secondary types to exclude (comma-separated)")
	monitorCmd.Flags().StringSliceVar(&excludeFormats, "exclude-formats", nil, "release formats to exclude (comma-separated, e.g. Vinyl,Cassette)")
	monitorCmd.Flags().IntVar(&maxInQueue, "max-in-queue", 2, "max concurrent searches in Lidarr queue")
	monitorCmd.Flags().IntVar(&delaySeconds, "delay-seconds", 5, "delay after each search submission")
	rootCmd.AddCommand(monitorCmd)
}

func runMonitor(cmd *cobra.Command, args []string) error {
	if len(artistIDStrs) == 0 && !allArtists {
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
	if cmd.Flags().Changed("exclude-formats") {
		cfg.Monitor.ExcludeFormats = excludeFormats
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
		cfg.Monitor.ExcludeFormats,
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
		resolved, err := resolveArtistIDs(client, artistIDStrs)
		if err != nil {
			return err
		}
		artistIDs = resolved
		log.Printf("Processing %d artist(s)", len(artistIDs))
	}

	start := time.Now()
	stats, err := mon.Run(artistIDs)
	if err != nil {
		return fmt.Errorf("monitor failed: %w", err)
	}

	mon.PrintSummary(stats, time.Since(start))
	return nil
}

// artistResolver is the subset of lidarr.Client needed for ID resolution.
type artistResolver interface {
	GetArtists() ([]lidarr.Artist, error)
}

func resolveArtistIDs(resolver artistResolver, idStrs []string) ([]int, error) {
	var numericIDs []int
	var foreignIDs []string

	for _, s := range idStrs {
		if id, err := strconv.Atoi(s); err == nil {
			numericIDs = append(numericIDs, id)
		} else {
			foreignIDs = append(foreignIDs, s)
		}
	}

	if len(foreignIDs) == 0 {
		return numericIDs, nil
	}

	// Fetch artists once for all foreign ID lookups
	log.Printf("Resolving %d foreign artist ID(s)...", len(foreignIDs))
	artists, err := resolver.GetArtists()
	if err != nil {
		return nil, fmt.Errorf("failed to get artists: %w", err)
	}

	foreignToLidarr := make(map[string]int, len(artists))
	foreignToName := make(map[string]string, len(artists))
	for _, a := range artists {
		foreignToLidarr[a.ForeignID] = a.ID
		foreignToName[a.ForeignID] = a.ArtistName
	}

	var resolvedIDs []int
	resolvedIDs = append(resolvedIDs, numericIDs...)

	for _, fid := range foreignIDs {
		id, ok := foreignToLidarr[fid]
		if !ok {
			return nil, fmt.Errorf("artist with foreign ID %q not found in Lidarr", fid)
		}
		log.Printf("Resolved foreign ID %s to: %s (ID: %d)", fid, foreignToName[fid], id)
		resolvedIDs = append(resolvedIDs, id)
	}

	return resolvedIDs, nil
}
