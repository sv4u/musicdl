package metadata

// Song represents song metadata.
type Song struct {
	Title       string
	Artist      string
	Album       string
	TrackNumber int
	Duration    int
	SpotifyURL  string
	CoverURL    string
	AlbumArtist string
	Year        int
	Date        string
	DiscNumber  int
	DiscCount   int
	TracksCount int
	Genre       string
	Explicit    bool
	ISRC        string
}
