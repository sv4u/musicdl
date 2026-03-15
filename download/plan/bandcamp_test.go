package plan

import "testing"

func TestIsBandcampURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"Bandcamp track URL", "https://artist.bandcamp.com/track/track-name", true},
		{"Bandcamp album URL", "https://artist.bandcamp.com/album/album-name", true},
		{"Bandcamp artist page", "https://artist.bandcamp.com", true},
		{"Bandcamp artist page with slash", "https://artist.bandcamp.com/", true},
		{"Bandcamp URL uppercase", "https://ARTIST.BANDCAMP.COM/track/test", true},
		{"YouTube URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", false},
		{"SoundCloud URL", "https://soundcloud.com/artist/track", false},
		{"Spotify URL", "https://open.spotify.com/track/123", false},
		{"Empty URL", "", false},
		{"Invalid URL", "not a url", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBandcampURL(tt.url)
			if result != tt.expected {
				t.Errorf("IsBandcampURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsBandcampTrack(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"Bandcamp track URL", "https://artist.bandcamp.com/track/track-name", true},
		{"Bandcamp track hyphenated artist", "https://my-artist.bandcamp.com/track/my-song", true},
		{"Bandcamp track with query", "https://artist.bandcamp.com/track/track-name?from=embed", true},
		{"Bandcamp album URL", "https://artist.bandcamp.com/album/album-name", false},
		{"Bandcamp artist page", "https://artist.bandcamp.com", false},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", false},
		{"Empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBandcampTrack(tt.url)
			if result != tt.expected {
				t.Errorf("IsBandcampTrack(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsBandcampAlbum(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"Bandcamp album URL", "https://artist.bandcamp.com/album/album-name", true},
		{"Bandcamp album hyphenated", "https://my-artist.bandcamp.com/album/my-album", true},
		{"Bandcamp album with query", "https://artist.bandcamp.com/album/album-name?from=search", true},
		{"Bandcamp track URL", "https://artist.bandcamp.com/track/track-name", false},
		{"Bandcamp artist page", "https://artist.bandcamp.com", false},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", false},
		{"Empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBandcampAlbum(tt.url)
			if result != tt.expected {
				t.Errorf("IsBandcampAlbum(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsBandcampArtist(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"Bandcamp artist page", "https://artist.bandcamp.com", true},
		{"Bandcamp artist page with slash", "https://artist.bandcamp.com/", true},
		{"Bandcamp artist page http", "http://artist.bandcamp.com/", true},
		{"Bandcamp track URL", "https://artist.bandcamp.com/track/track-name", false},
		{"Bandcamp album URL", "https://artist.bandcamp.com/album/album-name", false},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", false},
		{"Empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsBandcampArtist(tt.url)
			if result != tt.expected {
				t.Errorf("IsBandcampArtist(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractBandcampArtist(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"From track URL", "https://myartist.bandcamp.com/track/song", "myartist"},
		{"From album URL", "https://myartist.bandcamp.com/album/album", "myartist"},
		{"From artist page", "https://myartist.bandcamp.com/", "myartist"},
		{"Hyphenated artist", "https://my-artist.bandcamp.com/track/song", "my-artist"},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", ""},
		{"Empty URL", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractBandcampArtist(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractBandcampArtist(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractBandcampTrackSlug(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"Valid track URL", "https://artist.bandcamp.com/track/my-song", "my-song"},
		{"Track with query", "https://artist.bandcamp.com/track/my-song?from=embed", "my-song"},
		{"Album URL", "https://artist.bandcamp.com/album/my-album", ""},
		{"Artist page", "https://artist.bandcamp.com", ""},
		{"Empty URL", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractBandcampTrackSlug(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractBandcampTrackSlug(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractBandcampAlbumSlug(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"Valid album URL", "https://artist.bandcamp.com/album/my-album", "my-album"},
		{"Album with query", "https://artist.bandcamp.com/album/my-album?from=search", "my-album"},
		{"Track URL", "https://artist.bandcamp.com/track/my-song", ""},
		{"Artist page", "https://artist.bandcamp.com", ""},
		{"Empty URL", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractBandcampAlbumSlug(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractBandcampAlbumSlug(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}
