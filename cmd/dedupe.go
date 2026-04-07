package cmd

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/benscobie/lidarr-utils/internal/dedupe"
	"github.com/benscobie/lidarr-utils/internal/lidarr"
)

var (
	addImportExclusion bool
	runOnce            bool
	cronExpr           string
)

var dedupeCmd = &cobra.Command{
	Use:   "dedupe",
	Short: "Remove duplicate singles from your Lidarr library",
	Long: `Scan your Lidarr library to find singles that are duplicated
in albums or EPs by the same artist, and optionally remove them to clean up your library.

The command supports both one-time runs and scheduled execution via cron expressions.`,
	RunE: runDedupe,
}

func init() {
	dedupeCmd.Flags().BoolVar(&addImportExclusion, "add-import-exclusion", false, "add removed singles to import exclusion list")
	dedupeCmd.Flags().BoolVar(&runOnce, "run-once", true, "run once and exit (overrides schedule config)")
	dedupeCmd.Flags().StringVar(&cronExpr, "cron", "", "cron expression for scheduled runs (overrides config)")
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
	if cmd.Flags().Changed("run-once") {
		cfg.Schedule.RunOnce = runOnce
	}
	if cmd.Flags().Changed("cron") {
		cfg.Schedule.Cron = cronExpr
		cfg.Schedule.Enabled = true
		cfg.Schedule.RunOnce = false
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

	// Determine execution mode
	if cfg.Schedule.RunOnce || !cfg.Schedule.Enabled {
		return runDedupeOnce(deduper)
	}
	return runDedupeScheduled(deduper, cfg.Schedule.Cron)
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

func runDedupeScheduled(deduper *dedupe.Deduper, cronExpression string) error {
	log.Printf("Starting scheduled deduplication with cron expression: %s", cronExpression)

	c := cron.New()

	_, err := c.AddFunc(cronExpression, func() {
		log.Println("Starting scheduled deduplication run...")

		start := time.Now()

		duplicates, err := deduper.FindDuplicates()
		if err != nil {
			log.Printf("ERROR: Failed to find duplicates: %v", err)
			return
		}

		err = deduper.ProcessDuplicates(duplicates)
		if err != nil {
			log.Printf("ERROR: Failed to process duplicates: %v", err)
			return
		}

		// Print summary
		deduper.PrintSummary(duplicates)

		duration := time.Since(start)
		log.Printf("Scheduled deduplication completed in %v", duration)
	})

	if err != nil {
		return fmt.Errorf("failed to schedule job: %w", err)
	}

	c.Start()
	log.Println("Scheduler started. Waiting for scheduled runs...")
	log.Println("Press Ctrl+C to stop")

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down scheduler...")
	ctx := c.Stop()
	<-ctx.Done()
	log.Println("Scheduler stopped")

	return nil
}
