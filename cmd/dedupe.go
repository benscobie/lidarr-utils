package cmd

import (
	"fmt"
	"log"
	"time"

	"github.com/spf13/cobra"

	"github.com/benscobie/lidarr-utils/internal/dedupe"
	"github.com/benscobie/lidarr-utils/internal/lidarr"
)

var addImportExclusion bool

var dedupeCmd = &cobra.Command{
	Use:   "dedupe",
	Short: "Remove duplicate singles from your Lidarr library",
	Long: `Scan your Lidarr library to find singles that are duplicated
in albums or EPs by the same artist, and optionally remove them to clean up your library.`,
	RunE: runDedupe,
}

func init() {
	dedupeCmd.Flags().BoolVar(&addImportExclusion, "add-import-exclusion", false, "add removed singles to import exclusion list")
	rootCmd.AddCommand(dedupeCmd)
}

func runDedupe(cmd *cobra.Command, args []string) error {
	cfg, err := getConfig(cmd)
	if err != nil {
		return err
	}

	if cmd.Flags().Changed("add-import-exclusion") {
		cfg.Dedupe.AddImportExclusion = addImportExclusion
	}
	// Setup logging to file
	logFileHandle, err := setupLoggingFromConfig(cfg)
	if err != nil {
		return err
	}
	if logFileHandle != nil {
		defer logFileHandle.Close()
	}

	// Print configuration
	cfg.Print()
	fmt.Println()

	// Create Lidarr client
	client := lidarr.NewClient(cfg.Lidarr.URL, cfg.Lidarr.APIKey)

	// Test connection
	log.Println("Testing connection to Lidarr...")
	if err := client.TestConnection(); err != nil {
		return fmt.Errorf("failed to connect to Lidarr: %w", err)
	}
	log.Println("Successfully connected to Lidarr")

	// Create deduper
	deduper := dedupe.NewDeduper(client, cfg.App.DryRun, cfg.Dedupe.AddImportExclusion)

	return runDedupeOnce(deduper)
}

func runDedupeOnce(deduper *dedupe.Deduper) error {
	log.Println("Starting single deduplication run...")

	start := time.Now()

	duplicates, err := deduper.FindDuplicates()
	if err != nil {
		return fmt.Errorf("failed to find duplicates: %w", err)
	}

	err = deduper.ProcessDuplicates(duplicates)
	if err != nil {
		return fmt.Errorf("failed to process duplicates: %w", err)
	}

	// Print summary
	deduper.PrintSummary(duplicates)

	duration := time.Since(start)
	log.Printf("Deduplication completed in %v", duration)

	return nil
}
