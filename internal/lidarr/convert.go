package lidarr

import "github.com/benscobie/lidarr-utils/internal/common"

// ConvertTracks converts Lidarr API tracks to domain tracks.
func ConvertTracks(tracks []Track) []common.Track {
	result := make([]common.Track, len(tracks))
	for i, t := range tracks {
		result[i] = common.Track{
			ID:                 t.ID,
			Title:              t.Title,
			ForeignTrackID:     t.ForeignTrackID,
			ForeignRecordingID: t.ForeignRecordingID,
			TrackNumber:        t.TrackNumber,
			HasFile:            t.HasFile,
		}
	}
	return result
}

// ConvertReleases converts Lidarr API releases to domain releases.
func ConvertReleases(releases []Release) []common.Release {
	result := make([]common.Release, len(releases))
	for i, r := range releases {
		result[i] = common.Release{
			ID:        r.ID,
			Format:    r.Format,
			Monitored: r.Monitored,
		}
	}
	return result
}
