package plan

import "testing"

func TestIsAudiusURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"Audius track URL", "https://audius.co/artist/track-name", true},
		{"Audius playlist URL", "https://audius.co/artist/playlist/playlist-name", true},
		{"Audius user page", "https://audius.co/artist", true},
		{"Audius URL uppercase", "https://AUDIUS.CO/artist/track", true},
		{"YouTube URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", false},
		{"SoundCloud URL", "https://soundcloud.com/artist/track", false},
		{"Bandcamp URL", "https://artist.bandcamp.com/track/test", false},
		{"Spotify URL", "https://open.spotify.com/track/123", false},
		{"Empty URL", "", false},
		{"Invalid URL", "not a url", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAudiusURL(tt.url)
			if result != tt.expected {
				t.Errorf("IsAudiusURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsAudiusTrack(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"Audius track URL", "https://audius.co/artist/track-name", true},
		{"Audius track with query", "https://audius.co/artist/track-name?ref=share", true},
		{"Audius playlist URL", "https://audius.co/artist/playlist/playlist-name", false},
		{"Audius user page", "https://audius.co/artist", false},
		{"Audius user page with slash", "https://audius.co/artist/", false},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", false},
		{"Empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAudiusTrack(tt.url)
			if result != tt.expected {
				t.Errorf("IsAudiusTrack(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsAudiusPlaylist(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"Audius playlist URL", "https://audius.co/artist/playlist/playlist-name", true},
		{"Audius playlist with query", "https://audius.co/artist/playlist/playlist-name?ref=share", true},
		{"Audius track URL", "https://audius.co/artist/track-name", false},
		{"Audius user page", "https://audius.co/artist", false},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", false},
		{"Empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAudiusPlaylist(tt.url)
			if result != tt.expected {
				t.Errorf("IsAudiusPlaylist(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsAudiusUser(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"Audius user page", "https://audius.co/artist", true},
		{"Audius user page with slash", "https://audius.co/artist/", true},
		{"Audius user page http", "http://audius.co/artist", true},
		{"Audius track URL", "https://audius.co/artist/track-name", false},
		{"Audius playlist URL", "https://audius.co/artist/playlist/playlist-name", false},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", false},
		{"Empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAudiusUser(tt.url)
			if result != tt.expected {
				t.Errorf("IsAudiusUser(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractAudiusSlug(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"Audius track URL", "https://audius.co/artist-name/track-name", "artist-name/track-name"},
		{"Audius track with query", "https://audius.co/artist/song?ref=share", "artist/song"},
		{"Audius playlist URL", "https://audius.co/artist/playlist/playlist-name", ""},
		{"Audius user page", "https://audius.co/artist", ""},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", ""},
		{"Empty URL", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractAudiusSlug(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractAudiusSlug(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}
