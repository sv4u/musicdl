package plan

import "testing"

func TestIsSoundCloudURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"SoundCloud track URL", "https://soundcloud.com/artist/track-name", true},
		{"SoundCloud set URL", "https://soundcloud.com/artist/sets/set-name", true},
		{"SoundCloud user URL", "https://soundcloud.com/artist", true},
		{"SoundCloud URL with www", "https://www.soundcloud.com/artist/track-name", true},
		{"SoundCloud URL uppercase", "https://SOUNDCLOUD.COM/artist/track-name", true},
		{"YouTube URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", false},
		{"Spotify URL", "https://open.spotify.com/track/123", false},
		{"Empty URL", "", false},
		{"Invalid URL", "not a url", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSoundCloudURL(tt.url)
			if result != tt.expected {
				t.Errorf("IsSoundCloudURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsSoundCloudTrack(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"SoundCloud track URL", "https://soundcloud.com/artist/track-name", true},
		{"SoundCloud track with query params", "https://soundcloud.com/artist/track-name?in=playlist", true},
		{"SoundCloud set URL", "https://soundcloud.com/artist/sets/set-name", false},
		{"SoundCloud user page", "https://soundcloud.com/artist", false},
		{"SoundCloud user page with slash", "https://soundcloud.com/artist/", false},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", false},
		{"Empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSoundCloudTrack(tt.url)
			if result != tt.expected {
				t.Errorf("IsSoundCloudTrack(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsSoundCloudSet(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"SoundCloud set URL", "https://soundcloud.com/artist/sets/set-name", true},
		{"SoundCloud set URL with query", "https://soundcloud.com/artist/sets/set-name?si=abc", true},
		{"SoundCloud track URL", "https://soundcloud.com/artist/track-name", false},
		{"SoundCloud user page", "https://soundcloud.com/artist", false},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", false},
		{"Empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSoundCloudSet(tt.url)
			if result != tt.expected {
				t.Errorf("IsSoundCloudSet(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsSoundCloudUser(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"SoundCloud user page", "https://soundcloud.com/artist", true},
		{"SoundCloud user page with slash", "https://soundcloud.com/artist/", true},
		{"SoundCloud track URL", "https://soundcloud.com/artist/track-name", false},
		{"SoundCloud set URL", "https://soundcloud.com/artist/sets/set-name", false},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", false},
		{"Empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSoundCloudUser(tt.url)
			if result != tt.expected {
				t.Errorf("IsSoundCloudUser(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractSoundCloudSlug(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"SoundCloud track URL", "https://soundcloud.com/artist-name/track-name", "artist-name/track-name"},
		{"SoundCloud track with query", "https://soundcloud.com/artist/song?in=playlist", "artist/song"},
		{"SoundCloud set URL", "https://soundcloud.com/artist/sets/set-name", ""},
		{"SoundCloud user page", "https://soundcloud.com/artist", ""},
		{"YouTube URL", "https://www.youtube.com/watch?v=test", ""},
		{"Empty URL", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSoundCloudSlug(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractSoundCloudSlug(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}
