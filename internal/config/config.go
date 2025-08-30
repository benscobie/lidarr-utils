package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Lidarr   LidarrConfig   `mapstructure:"lidarr"`
	App      AppConfig      `mapstructure:"app"`
	Schedule ScheduleConfig `mapstructure:"schedule"`
}

type LidarrConfig struct {
	URL    string `mapstructure:"url"`
	APIKey string `mapstructure:"api_key"`
}

type AppConfig struct {
	DryRun             bool   `mapstructure:"dry_run"`
	AddImportExclusion bool   `mapstructure:"add_import_exclusion"`
	LogLevel           string `mapstructure:"log_level"`
	LogFile            string `mapstructure:"log_file"`
}

type ScheduleConfig struct {
	Enabled bool   `mapstructure:"enabled"`
	Cron    string `mapstructure:"cron"`
	RunOnce bool   `mapstructure:"run_once"`
}

func LoadConfig(configPath string) (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		viper.AddConfigPath(".")
		viper.AddConfigPath("$HOME/.lidarr-deduper")
		viper.AddConfigPath("/etc/lidarr-deduper")
	}

	// Environment variable bindings
	viper.SetEnvPrefix("LIDARR_DEDUPE")
	viper.AutomaticEnv()

	// Bind specific environment variables
	viper.BindEnv("lidarr.url", "LIDARR_DEDUPE_LIDARR_URL")
	viper.BindEnv("lidarr.api_key", "LIDARR_DEDUPE_LIDARR_API_KEY")
	viper.BindEnv("app.dry_run", "LIDARR_DEDUPE_APP_DRY_RUN")
	viper.BindEnv("app.add_import_exclusion", "LIDARR_DEDUPE_APP_ADD_IMPORT_EXCLUSION")
	viper.BindEnv("app.log_level", "LIDARR_DEDUPE_APP_LOG_LEVEL")
	viper.BindEnv("app.log_file", "LIDARR_DEDUPE_APP_LOG_FILE")
	viper.BindEnv("schedule.enabled", "LIDARR_DEDUPE_SCHEDULE_ENABLED")
	viper.BindEnv("schedule.cron", "LIDARR_DEDUPE_SCHEDULE_CRON")
	viper.BindEnv("schedule.run_once", "LIDARR_DEDUPE_SCHEDULE_RUN_ONCE")

	// Set defaults
	viper.SetDefault("app.dry_run", true)
	viper.SetDefault("app.add_import_exclusion", false)
	viper.SetDefault("app.log_level", "info")
	viper.SetDefault("app.log_file", "lidarr-deduper.log")
	viper.SetDefault("schedule.enabled", false)
	viper.SetDefault("schedule.cron", "0 2 * * *") // 2 AM daily
	viper.SetDefault("schedule.run_once", true)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; using defaults and environment variables
			fmt.Println("Config file not found, using defaults and environment variables")
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	var config Config
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}

	// Validate required fields
	if config.Lidarr.URL == "" {
		return nil, fmt.Errorf("lidarr.url is required")
	}
	if config.Lidarr.APIKey == "" {
		return nil, fmt.Errorf("lidarr.api_key is required")
	}

	return &config, nil
}

func (c *Config) Print() {
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Lidarr URL: %s\n", c.Lidarr.URL)
	fmt.Printf("  API Key: %s***\n", c.Lidarr.APIKey[:8]) // Show only first 8 characters
	fmt.Printf("  Dry Run: %v\n", c.App.DryRun)
	fmt.Printf("  Add Import Exclusion: %v\n", c.App.AddImportExclusion)
	fmt.Printf("  Log Level: %s\n", c.App.LogLevel)
	fmt.Printf("  Log File: %s\n", c.App.LogFile)
	fmt.Printf("  Schedule Enabled: %v\n", c.Schedule.Enabled)
	if c.Schedule.Enabled {
		fmt.Printf("  Schedule Cron: %s\n", c.Schedule.Cron)
	}
	fmt.Printf("  Run Once: %v\n", c.Schedule.RunOnce)
}
