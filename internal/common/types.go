package common

type Release struct {
	ID        int
	Format    string
	Monitored bool
}

type Album struct {
	ID              int
	Title           string
	AlbumType       string
	SecondaryTypes  []string
	ArtistID        int
	ArtistName      string
	ForeignAlbumID  string
	Tracks          []Track
	Releases        []Release
	HasFiles        bool
	Monitored       bool
	IsVACompilation bool
}

type Track struct {
	ID                 int
	Title              string
	ForeignTrackID     string
	ForeignRecordingID string
	TrackNumber        string
	HasFile            bool
}
