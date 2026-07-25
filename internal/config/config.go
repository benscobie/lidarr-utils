package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Lidarr  LidarrConfig  `mapstructure:"lidarr"`
	App     AppConfig     `mapstructure:"app"`
	Dedupe  DedupeConfig  `mapstructure:"dedupe"`
	Monitor MonitorConfig `mapstructure:"monitor"`
}

type LidarrConfig struct {
	URL    string `mapstructure:"url"`
	APIKey string `mapstructure:"api_key"`
}

type AppConfig struct {
	DryRun    bool   `mapstructure:"dry_run"`
	LogLevel  string `mapstructure:"log_level"`
	LogFile   string `mapstructure:"log_file"`
	StateFile string `mapstructure:"state_file"`
}

type DedupeConfig struct {
	AddImportExclusion bool           `mapstructure:"add_import_exclusion"`
	Schedule           ScheduleConfig `mapstructure:"schedule"`
}

type MonitorFilters struct {
	OfficialOnly          bool     `mapstructure:"official_only"`
	ExcludeSecondaryTypes []string `mapstructure:"exclude_secondary_types"`
	ExcludeFormats        []string `mapstructure:"exclude_formats"`
	ExcludeVAReleases     bool     `mapstructure:"exclude_va_releases"`
}

type ScheduleConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	Cron       string `mapstructure:"cron"`
	RunOnStart bool   `mapstructure:"run_on_start"`
}

type ArtistMonitorConfig struct {
	MonitorFilters           `mapstructure:",squash"`
	SkipFullyCoveredReleases bool           `mapstructure:"skip_fully_covered_releases"`
	Schedule                 ScheduleConfig `mapstructure:"schedule"`
}

type LabelMonitorConfig struct {
	MonitorFilters           `mapstructure:",squash"`
	IDs                      []string       `mapstructure:"ids"`
	AddMissingArtists        bool           `mapstructure:"add_missing_artists"`
	RootFolder               string         `mapstructure:"root_folder"`
	SkipFullyCoveredReleases bool           `mapstructure:"skip_fully_covered_releases"`
	Schedule                 ScheduleConfig `mapstructure:"schedule"`
}

type MonitorConfig struct {
	Artists ArtistMonitorConfig `mapstructure:"artists"`
	Labels  LabelMonitorConfig  `mapstructure:"labels"`
}

func LoadConfig(configPath string) (*Config, error) {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")

	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.AddConfigPath(".")
		v.AddConfigPath("$HOME/.lidarr-utils")
		v.AddConfigPath("/etc/lidarr-utils")
	}

	// Environment variable bindings
	v.SetEnvPrefix("LIDARR_UTILS")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Bind specific environment variables
	bindings := map[string]string{
		"lidarr.url":                                  "LIDARR_UTILS_LIDARR_URL",
		"lidarr.api_key":                              "LIDARR_UTILS_LIDARR_API_KEY",
		"app.dry_run":                                 "LIDARR_UTILS_APP_DRY_RUN",
		"app.log_level":                               "LIDARR_UTILS_APP_LOG_LEVEL",
		"app.log_file":                                "LIDARR_UTILS_APP_LOG_FILE",
		"app.state_file":                              "LIDARR_UTILS_APP_STATE_FILE",
		"dedupe.add_import_exclusion":                 "LIDARR_UTILS_DEDUPE_ADD_IMPORT_EXCLUSION",
		"dedupe.schedule.enabled":                     "LIDARR_UTILS_DEDUPE_SCHEDULE_ENABLED",
		"dedupe.schedule.cron":                        "LIDARR_UTILS_DEDUPE_SCHEDULE_CRON",
		"dedupe.schedule.run_on_start":                "LIDARR_UTILS_DEDUPE_SCHEDULE_RUN_ON_START",
		"monitor.artists.official_only":               "LIDARR_UTILS_MONITOR_ARTISTS_OFFICIAL_ONLY",
		"monitor.artists.exclude_secondary_types":     "LIDARR_UTILS_MONITOR_ARTISTS_EXCLUDE_SECONDARY_TYPES",
		"monitor.artists.exclude_formats":             "LIDARR_UTILS_MONITOR_ARTISTS_EXCLUDE_FORMATS",
		"monitor.artists.exclude_va_releases":         "LIDARR_UTILS_MONITOR_ARTISTS_EXCLUDE_VA_RELEASES",
		"monitor.artists.skip_fully_covered_releases": "LIDARR_UTILS_MONITOR_ARTISTS_SKIP_FULLY_COVERED_RELEASES",
		"monitor.artists.schedule.enabled":            "LIDARR_UTILS_MONITOR_ARTISTS_SCHEDULE_ENABLED",
		"monitor.artists.schedule.cron":               "LIDARR_UTILS_MONITOR_ARTISTS_SCHEDULE_CRON",
		"monitor.artists.schedule.run_on_start":       "LIDARR_UTILS_MONITOR_ARTISTS_SCHEDULE_RUN_ON_START",
		"monitor.labels.add_missing_artists":          "LIDARR_UTILS_MONITOR_LABELS_ADD_MISSING_ARTISTS",
		"monitor.labels.root_folder":                  "LIDARR_UTILS_MONITOR_LABELS_ROOT_FOLDER",
		"monitor.labels.official_only":                "LIDARR_UTILS_MONITOR_LABELS_OFFICIAL_ONLY",
		"monitor.labels.exclude_secondary_types":      "LIDARR_UTILS_MONITOR_LABELS_EXCLUDE_SECONDARY_TYPES",
		"monitor.labels.exclude_formats":              "LIDARR_UTILS_MONITOR_LABELS_EXCLUDE_FORMATS",
		"monitor.labels.exclude_va_releases":          "LIDARR_UTILS_MONITOR_LABELS_EXCLUDE_VA_RELEASES",
		"monitor.labels.skip_fully_covered_releases":  "LIDARR_UTILS_MONITOR_LABELS_SKIP_FULLY_COVERED_RELEASES",
		"monitor.labels.schedule.enabled":             "LIDARR_UTILS_MONITOR_LABELS_SCHEDULE_ENABLED",
		"monitor.labels.schedule.cron":                "LIDARR_UTILS_MONITOR_LABELS_SCHEDULE_CRON",
		"monitor.labels.schedule.run_on_start":        "LIDARR_UTILS_MONITOR_LABELS_SCHEDULE_RUN_ON_START",
	}
	for key, env := range bindings {
		if err := v.BindEnv(key, env); err != nil {
			return nil, fmt.Errorf("bind environment variable %s: %w", env, err)
		}
	}

	// Set defaults
	v.SetDefault("app.dry_run", false)
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.log_file", "lidarr-utils.log")
	v.SetDefault("dedupe.add_import_exclusion", false)
	v.SetDefault("monitor.artists.skip_fully_covered_releases", true)
	v.SetDefault("monitor.labels.skip_fully_covered_releases", true)
	for _, prefix := range []string{
		"monitor.artists.schedule",
		"monitor.labels.schedule",
		"dedupe.schedule",
	} {
		v.SetDefault(prefix+".enabled", false)
		v.SetDefault(prefix+".cron", "0 2 * * *")
		v.SetDefault(prefix+".run_on_start", false)
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; using defaults and environment variables
			fmt.Println("Config file not found, using defaults and environment variables")
		} else {
			return nil, fmt.Errorf("error reading config file: %w", err)
		}
	}

	// Auto-resolve state file path if not explicitly set
	if v.GetString("app.state_file") == "" {
		var stateDir string
		if cf := v.ConfigFileUsed(); cf != "" {
			stateDir = filepath.Dir(cf)
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return nil, fmt.Errorf("unable to determine home directory: %w", err)
			}
			stateDir = filepath.Join(home, ".lidarr-utils")
		}
		v.SetDefault("app.state_file", filepath.Join(stateDir, "state.json"))
	}

	var config Config
	if err := v.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode config: %w", err)
	}
	normalizedIDs, err := NormalizeLabelIDs(config.Monitor.Labels.IDs)
	if err != nil {
		return nil, err
	}
	config.Monitor.Labels.IDs = normalizedIDs

	// Validate required fields
	if config.Lidarr.URL == "" {
		return nil, fmt.Errorf("lidarr.url is required")
	}
	if config.Lidarr.APIKey == "" {
		return nil, fmt.Errorf("lidarr.api_key is required")
	}

	return &config, nil
}

var uuidPattern = regexp.MustCompile(
	`(?i)^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`,
)

func NormalizeLabelIDs(ids []string) ([]string, error) {
	seen := make(map[string]struct{}, len(ids))
	normalizedIDs := make([]string, 0, len(ids))
	for _, id := range ids {
		normalized := strings.ToLower(strings.TrimSpace(id))
		if !uuidPattern.MatchString(normalized) {
			return nil, fmt.Errorf("monitor.labels.ids contains invalid UUID %q", id)
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		normalizedIDs = append(normalizedIDs, normalized)
	}
	return normalizedIDs, nil
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
	fmt.Printf("  State File: %s\n", c.App.StateFile)
	fmt.Printf("  Dedupe:\n")
	fmt.Printf("    Add Import Exclusion: %v\n", c.Dedupe.AddImportExclusion)
	printSchedule("    Schedule", c.Dedupe.Schedule)
	fmt.Printf("  Monitor Artists:\n")
	printMonitorFilters(c.Monitor.Artists.MonitorFilters)
	fmt.Printf("    Skip Fully Covered Releases: %v\n", c.Monitor.Artists.SkipFullyCoveredReleases)
	printSchedule("    Schedule", c.Monitor.Artists.Schedule)
	fmt.Printf("  Monitor Labels:\n")
	fmt.Printf("    IDs: %v\n", c.Monitor.Labels.IDs)
	fmt.Printf("    Add Missing Artists: %v\n", c.Monitor.Labels.AddMissingArtists)
	fmt.Printf("    Root Folder: %s\n", c.Monitor.Labels.RootFolder)
	printMonitorFilters(c.Monitor.Labels.MonitorFilters)
	fmt.Printf("    Skip Fully Covered Releases: %v\n", c.Monitor.Labels.SkipFullyCoveredReleases)
	printSchedule("    Schedule", c.Monitor.Labels.Schedule)
}

func printMonitorFilters(filters MonitorFilters) {
	fmt.Printf("    Official Only: %v\n", filters.OfficialOnly)
	fmt.Printf("    Exclude Secondary Types: %v\n", filters.ExcludeSecondaryTypes)
	fmt.Printf("    Exclude Formats: %v\n", filters.ExcludeFormats)
	fmt.Printf("    Exclude VA Releases: %v\n", filters.ExcludeVAReleases)
}

func printSchedule(prefix string, schedule ScheduleConfig) {
	fmt.Printf("%s Enabled: %v\n", prefix, schedule.Enabled)
	if schedule.Enabled {
		fmt.Printf("%s Cron: %s\n", prefix, schedule.Cron)
	}
	fmt.Printf("%s Run On Start: %v\n", prefix, schedule.RunOnStart)
}
