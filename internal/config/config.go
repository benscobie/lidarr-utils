package config

import (
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	Lidarr   LidarrConfig   `mapstructure:"lidarr"`
	App      AppConfig      `mapstructure:"app"`
	Dedupe   DedupeConfig   `mapstructure:"dedupe"`
	Monitor  MonitorConfig  `mapstructure:"monitor"`
	Schedule ScheduleConfig `mapstructure:"schedule"`
}

type LidarrConfig struct {
	URL    string `mapstructure:"url"`
	APIKey string `mapstructure:"api_key"`
}

type AppConfig struct {
	DryRun   bool   `mapstructure:"dry_run"`
	LogLevel string `mapstructure:"log_level"`
	LogFile  string `mapstructure:"log_file"`
}

type DedupeConfig struct {
	AddImportExclusion bool `mapstructure:"add_import_exclusion"`
}

type MonitorConfig struct {
	OfficialOnly          bool        `mapstructure:"official_only"`
	ExcludeSecondaryTypes []string    `mapstructure:"exclude_secondary_types"`
	ExcludeFormats        []string    `mapstructure:"exclude_formats"`
	ExcludeVAReleases     bool        `mapstructure:"exclude_va_releases"`
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
		viper.AddConfigPath("$HOME/.lidarr-utils")
		viper.AddConfigPath("/etc/lidarr-utils")
	}

	// Environment variable bindings
	viper.SetEnvPrefix("LIDARR_UTILS")
	viper.AutomaticEnv()

	// Bind specific environment variables
	viper.BindEnv("lidarr.url", "LIDARR_UTILS_LIDARR_URL")
	viper.BindEnv("lidarr.api_key", "LIDARR_UTILS_LIDARR_API_KEY")
	viper.BindEnv("app.dry_run", "LIDARR_UTILS_APP_DRY_RUN")
	viper.BindEnv("app.log_level", "LIDARR_UTILS_APP_LOG_LEVEL")
	viper.BindEnv("app.log_file", "LIDARR_UTILS_APP_LOG_FILE")
	viper.BindEnv("dedupe.add_import_exclusion", "LIDARR_UTILS_DEDUPE_ADD_IMPORT_EXCLUSION")
	viper.BindEnv("monitor.official_only", "LIDARR_UTILS_MONITOR_OFFICIAL_ONLY")
	viper.BindEnv("monitor.exclude_secondary_types", "LIDARR_UTILS_MONITOR_EXCLUDE_SECONDARY_TYPES")
	viper.BindEnv("monitor.exclude_formats", "LIDARR_UTILS_MONITOR_EXCLUDE_FORMATS")
	viper.BindEnv("monitor.exclude_va_releases", "LIDARR_UTILS_MONITOR_EXCLUDE_VA_RELEASES")
	viper.BindEnv("schedule.enabled", "LIDARR_UTILS_SCHEDULE_ENABLED")
	viper.BindEnv("schedule.cron", "LIDARR_UTILS_SCHEDULE_CRON")
	viper.BindEnv("schedule.run_once", "LIDARR_UTILS_SCHEDULE_RUN_ONCE")

	// Set defaults
	viper.SetDefault("app.dry_run", false)
	viper.SetDefault("app.log_level", "info")
	viper.SetDefault("app.log_file", "lidarr-utils.log")
	viper.SetDefault("dedupe.add_import_exclusion", false)
	viper.SetDefault("monitor.official_only", false)
	viper.SetDefault("monitor.exclude_secondary_types", []string{})
	viper.SetDefault("monitor.exclude_formats", []string{})
	viper.SetDefault("monitor.exclude_va_releases", false)
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
	if len(c.Lidarr.APIKey) >= 8 {
		fmt.Printf("  API Key: %s***\n", c.Lidarr.APIKey[:8])
	} else {
		fmt.Printf("  API Key: ***\n")
	}
	fmt.Printf("  Dry Run: %v\n", c.App.DryRun)
	fmt.Printf("  Log Level: %s\n", c.App.LogLevel)
	fmt.Printf("  Log File: %s\n", c.App.LogFile)
	fmt.Printf("  Dedupe:\n")
	fmt.Printf("    Add Import Exclusion: %v\n", c.Dedupe.AddImportExclusion)
	fmt.Printf("  Monitor:\n")
	fmt.Printf("    Official Only: %v\n", c.Monitor.OfficialOnly)
	fmt.Printf("    Exclude Secondary Types: %v\n", c.Monitor.ExcludeSecondaryTypes)
	fmt.Printf("    Exclude Formats: %v\n", c.Monitor.ExcludeFormats)
	fmt.Printf("    Exclude VA Releases: %v\n", c.Monitor.ExcludeVAReleases)
	fmt.Printf("  Schedule Enabled: %v\n", c.Schedule.Enabled)
	if c.Schedule.Enabled {
		fmt.Printf("  Schedule Cron: %s\n", c.Schedule.Cron)
	}
	fmt.Printf("  Run Once: %v\n", c.Schedule.RunOnce)
}
