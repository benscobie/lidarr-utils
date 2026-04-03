package common

type Album struct {
	ID             int
	Title          string
	AlbumType      string
	SecondaryTypes []string
	ArtistID       int
	ArtistName     string
	Tracks         []Track
	HasFiles       bool
	Monitored      bool
}

type Track struct {
	ID                 int
	Title              string
	ForeignTrackID     string
	ForeignRecordingID string
	TrackNumber        string
	HasFile            bool
}
