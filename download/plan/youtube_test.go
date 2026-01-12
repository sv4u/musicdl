package plan

import "testing"

func TestIsYouTubeURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"YouTube watch URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"YouTube short URL", "https://youtu.be/dQw4w9WgXcQ", true},
		{"YouTube playlist URL", "https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", true},
		{"YouTube embed URL", "https://www.youtube.com/embed/dQw4w9WgXcQ", true},
		{"YouTube nocookie URL", "https://www.youtube-nocookie.com/watch?v=dQw4w9WgXcQ", true},
		{"YouTube URL with uppercase", "https://YOUTUBE.COM/watch?v=dQw4w9WgXcQ", true},
		{"Spotify URL", "https://open.spotify.com/track/123", false},
		{"Empty URL", "", false},
		{"Invalid URL", "not a url", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsYouTubeURL(tt.url)
			if result != tt.expected {
				t.Errorf("IsYouTubeURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsYouTubeVideo(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"YouTube watch URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", true},
		{"YouTube short URL", "https://youtu.be/dQw4w9WgXcQ", true},
		{"YouTube embed URL", "https://www.youtube.com/embed/dQw4w9WgXcQ", true},
		{"YouTube playlist URL", "https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", false},
		{"YouTube video with playlist param", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", true},
		{"YouTube URL with uppercase", "https://YOUTUBE.COM/watch?v=dQw4w9WgXcQ", true},
		{"YouTube URL with mixed case", "https://www.YOUTUBE.com/watch?v=dQw4w9WgXcQ", true},
		{"Spotify URL", "https://open.spotify.com/track/123", false},
		{"Empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsYouTubeVideo(tt.url)
			if result != tt.expected {
				t.Errorf("IsYouTubeVideo(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestIsYouTubePlaylist(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"YouTube playlist URL", "https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", true},
		{"YouTube video with playlist param", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", true},
		{"YouTube watch URL only", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", false},
		{"YouTube playlist URL with uppercase", "https://YOUTUBE.COM/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", true},
		{"YouTube video with playlist param and uppercase", "https://www.YOUTUBE.com/watch?v=dQw4w9WgXcQ&list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", true},
		{"Spotify URL", "https://open.spotify.com/track/123", false},
		{"Empty URL", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsYouTubePlaylist(tt.url)
			if result != tt.expected {
				t.Errorf("IsYouTubePlaylist(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractYouTubeVideoID(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"YouTube watch URL", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"YouTube short URL", "https://youtu.be/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"YouTube embed URL", "https://www.youtube.com/embed/dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"YouTube playlist URL", "https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", ""},
		{"YouTube URL with uppercase", "https://YOUTUBE.COM/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"YouTube URL with mixed case", "https://www.YOUTUBE.com/watch?v=dQw4w9WgXcQ", "dQw4w9WgXcQ"},
		{"Spotify URL", "https://open.spotify.com/track/123", ""},
		{"Empty URL", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractYouTubeVideoID(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractYouTubeVideoID(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}

func TestExtractYouTubePlaylistID(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected string
	}{
		{"YouTube playlist URL", "https://www.youtube.com/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", "PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf"},
		{"YouTube video with playlist param", "https://www.youtube.com/watch?v=dQw4w9WgXcQ&list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", "PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf"},
		{"YouTube watch URL only", "https://www.youtube.com/watch?v=dQw4w9WgXcQ", ""},
		{"YouTube playlist URL with uppercase", "https://YOUTUBE.COM/playlist?list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", "PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf"},
		{"YouTube video with playlist param and uppercase", "https://www.YOUTUBE.com/watch?v=dQw4w9WgXcQ&list=PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf", "PLrAXtmErZgOeiKm4sgNOknGvNjby9efdf"},
		{"Spotify URL", "https://open.spotify.com/track/123", ""},
		{"Empty URL", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractYouTubePlaylistID(tt.url)
			if result != tt.expected {
				t.Errorf("ExtractYouTubePlaylistID(%q) = %q, expected %q", tt.url, result, tt.expected)
			}
		})
	}
}
