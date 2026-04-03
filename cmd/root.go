package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/lidarr-utils/internal/config"
)

var (
	configFile string
	dryRun     bool
	runOnce    bool
	cronExpr   string
	logFile    string
)

var rootCmd = &cobra.Command{
	Use:   "lidarr-utils",
	Short: "A collection of useful Lidarr utilities",
	Long: `lidarr-utils provides a collection of commands for managing your Lidarr library,
including deduplication of singles and monitoring of new releases.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", true, "perform a dry run without making changes")
	rootCmd.PersistentFlags().BoolVar(&runOnce, "run-once", true, "run once and exit (overrides schedule config)")
	rootCmd.PersistentFlags().StringVar(&cronExpr, "cron", "", "cron expression for scheduled runs (overrides config)")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "path to log file (default: lidarr-deduper.log)")
}

func getConfig(cmd *cobra.Command) (*config.Config, error) {
	cfg, err := config.LoadConfig(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Override config with command line flags if provided
	if cmd.Flags().Changed("dry-run") {
		cfg.App.DryRun = dryRun
	}
	if cmd.Flags().Changed("log-file") {
		cfg.App.LogFile = logFile
	} else if cfg.App.LogFile == "" {
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

	return cfg, nil
}

func setupLogging(logFilePath string) (*os.File, error) {
	// Create the directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(logFilePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file for writing (append mode)
	logFileHandle, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	// Create a multi-writer that writes to both stdout and the log file
	multiWriter := io.MultiWriter(os.Stdout, logFileHandle)
	log.SetOutput(multiWriter)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	return logFileHandle, nil
}

func setupLoggingFromConfig(cfg *config.Config) (*os.File, error) {
	if cfg.App.LogFile == "" {
		return nil, nil
	}

	logFileHandle, err := setupLogging(cfg.App.LogFile)
	if err != nil {
		return nil, fmt.Errorf("failed to setup logging: %w", err)
	}

	log.Printf("Logging to file: %s", cfg.App.LogFile)
	return logFileHandle, nil
}
