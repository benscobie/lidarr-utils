package config

import "fmt"

type VariousArtistsPolicy string
type CompilationSinglesPolicy string

const (
	VariousArtistsInclude VariousArtistsPolicy = "include"
	VariousArtistsExclude VariousArtistsPolicy = "exclude"
	VariousArtistsOnly    VariousArtistsPolicy = "only"

	CompilationSinglesInclude CompilationSinglesPolicy = "include"
	CompilationSinglesExclude CompilationSinglesPolicy = "exclude"
)

type ReleaseSelectionPolicy struct {
	IncludeSecondaryTypes    bool                     `mapstructure:"include_secondary_types"`
	ExcludeSecondaryTypes    []string                 `mapstructure:"exclude_secondary_types"`
	ExcludeFormats           []string                 `mapstructure:"exclude_formats"`
	VariousArtists           VariousArtistsPolicy     `mapstructure:"various_artists"`
	CompilationSingles       CompilationSinglesPolicy `mapstructure:"compilation_singles"`
	SkipFullyCoveredReleases bool                     `mapstructure:"skip_fully_covered_releases"`
}

type MissingArtistsConfig struct {
	Enabled    bool   `mapstructure:"enabled"`
	RootFolder string `mapstructure:"root_folder"`
}

func validateReleaseSelection(prefix string, policy ReleaseSelectionPolicy) error {
	switch policy.VariousArtists {
	case VariousArtistsInclude, VariousArtistsExclude, VariousArtistsOnly:
	default:
		return fmt.Errorf(
			"%s.various_artists has invalid value %q; accepted values: include, exclude, only",
			prefix,
			policy.VariousArtists,
		)
	}
	switch policy.CompilationSingles {
	case CompilationSinglesInclude, CompilationSinglesExclude:
	default:
		return fmt.Errorf(
			"%s.compilation_singles has invalid value %q; accepted values: include, exclude",
			prefix,
			policy.CompilationSingles,
		)
	}
	if !policy.IncludeSecondaryTypes && len(policy.ExcludeSecondaryTypes) > 0 {
		return fmt.Errorf(
			"%s.exclude_secondary_types must be empty when %s.include_secondary_types is false",
			prefix,
			prefix,
		)
	}
	return nil
}
