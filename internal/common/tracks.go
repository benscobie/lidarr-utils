package common

import "strings"

func AreTracksTheSame(track1, track2 Track) bool {
	if track1.ForeignRecordingID != "" && track2.ForeignRecordingID != "" {
		return track1.ForeignRecordingID == track2.ForeignRecordingID
	}

	if track1.ForeignTrackID != "" && track2.ForeignTrackID != "" {
		return track1.ForeignTrackID == track2.ForeignTrackID
	}

	return NormalizeTrackTitle(track1.Title) == NormalizeTrackTitle(track2.Title)
}

func NormalizeTrackTitle(title string) string {
	title = strings.ToLower(title)
	title = strings.TrimSpace(title)

	suffixes := []string{
		" (remaster)", " (remastered)", " - remaster", " - remastered",
		" (radio edit)", " - radio edit",
		" (single version)", " - single version",
		" (album version)", " - album version",
		" (explicit)", " - explicit",
		" (clean)", " - clean",
	}

	for _, suffix := range suffixes {
		title = strings.TrimSuffix(title, suffix)
	}

	return title
}
