package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/spf13/cobra"

	"github.com/lidarr-deduper/internal/config"
	"github.com/lidarr-deduper/internal/dedupe"
	"github.com/lidarr-deduper/internal/lidarr"
)

var (
	configFile         string
	dryRun             bool
	addImportExclusion bool
	runOnce            bool
	cronExpr           string
	logFile            string
)

var rootCmd = &cobra.Command{
	Use:   "lidarr-deduper",
	Short: "Remove duplicate singles from your Lidarr library",
	Long: `Lidarr Dedupe scans your Lidarr library to find singles that are duplicated 
in albums or EPs by the same artist, and optionally removes them to clean up your library.

The tool supports both one-time runs and scheduled execution via cron expressions.`,
	RunE: runDeduplication,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.Flags().StringVarP(&configFile, "config", "c", "", "config file path")
	rootCmd.Flags().BoolVar(&dryRun, "dry-run", true, "perform a dry run without making changes")
	rootCmd.Flags().BoolVar(&addImportExclusion, "add-import-exclusion", false, "add removed singles to import exclusion list")
	rootCmd.Flags().BoolVar(&runOnce, "run-once", true, "run once and exit (overrides schedule config)")
	rootCmd.Flags().StringVar(&cronExpr, "cron", "", "cron expression for scheduled runs (overrides config)")
	rootCmd.Flags().StringVar(&logFile, "log-file", "", "path to log file (default: lidarr-deduper.log)")
}

func setupLogging(logFilePath string) (*os.File, error) {
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file for writing (append mode)
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create a multi-writer that writes to both stdout and the log file
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	return logFile, nil
}

func runDeduplication(cmd *cobra.Command, args []string) error {
	// Load configuration
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with command line flags if provided
	if cmd.Flags().Changed("dry-run") {
		cfg.App.DryRun = dryRun
	}
	if cmd.Flags().Changed("add-import-exclusion") {
		cfg.App.AddImportExclusion = addImportExclusion
	}
	if cmd.Flags().Changed("log-file") {
		cfg.App.LogFile = logFile
	} else if cfg.App.LogFile == "" {
		// Use default log file if none specified
		cfg.App.LogFile = "lidarr-deduper.log"
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
	var logFileHandle *os.File
	if cfg.App.LogFile != "" {
		logFileHandle, err = setupLogging(cfg.App.LogFile)
		if err != nil {
			return fmt.Errorf("failed to setup logging: %w", err)
		}
		defer logFileHandle.Close()
		log.Printf("Logging to file: %s", cfg.App.LogFile)
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
	deduper := dedupe.NewDeduper(client, cfg.App.DryRun, cfg.App.AddImportExclusion)

	// Determine execution mode
	if cfg.Schedule.RunOnce || !cfg.Schedule.Enabled {
		// Run once
		return runDeduplicationOnce(deduper)
	} else {
		// Run on schedule
		return runDeduplicationScheduled(deduper, cfg.Schedule.Cron)
	}
}

func runDeduplicationOnce(deduper *dedupe.Deduper) error {
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

func runDeduplicationScheduled(deduper *dedupe.Deduper, cronExpression string) error {
	log.Printf("Starting scheduled deduplication with cron expression: %s", cronExpression)

	c := cron.New(cron.WithSeconds())

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
