package cmd

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/benscobie/lidarr-utils/internal/config"
	appLogging "github.com/benscobie/lidarr-utils/internal/logging"
)

var (
	configFile string
	dryRun     bool
	logFile    string
)

var rootCmd = &cobra.Command{
	Use:   "lidarr-utils",
	Short: "A collection of useful Lidarr utilities",
	Long: `lidarr-utils provides a collection of commands for managing your Lidarr library,
including deduplication of singles and monitoring of new releases.`,
	Version: version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "config file path")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "perform a dry run without making changes")
	rootCmd.PersistentFlags().StringVar(&logFile, "log-file", "", "path to log file (default: lidarr-utils.log)")
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
		cfg.App.LogFile = "lidarr-utils.log"
	}

	return cfg, nil
}

func setupLogging(logFilePath string, logLevel appLogging.Level) (*os.File, error) {
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
	slog.SetDefault(appLogging.New(multiWriter, logLevel))

	return logFileHandle, nil
}

func setupLoggingFromConfig(cfg *config.Config) (*os.File, error) {
	logLevel, err := appLogging.ParseLevel(cfg.App.LogLevel)
	if err != nil {
		return nil, fmt.Errorf("invalid configured log level: %w", err)
	}
	if cfg.App.LogFile == "" {
		slog.SetDefault(appLogging.New(os.Stdout, logLevel))
		return nil, nil
	}

	logFileHandle, err := setupLogging(cfg.App.LogFile, logLevel)
	if err != nil {
		return nil, fmt.Errorf("failed to setup logging: %w", err)
	}

	slog.Info("Logging to file", "path", cfg.App.LogFile)
	return logFileHandle, nil
}
