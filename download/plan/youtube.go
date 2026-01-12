package plan

import (
	"regexp"
	"strings"
)

// IsYouTubeURL checks if a URL is a YouTube URL (video or playlist).
func IsYouTubeURL(url string) bool {
	if url == "" {
		return false
	}
	urlLower := strings.ToLower(url)
	return strings.Contains(urlLower, "youtube.com") ||
		strings.Contains(urlLower, "youtu.be") ||
		strings.Contains(urlLower, "youtube-nocookie.com")
}

// IsYouTubeVideo checks if a URL is a YouTube video URL.
func IsYouTubeVideo(url string) bool {
	if !IsYouTubeURL(url) {
		return false
	}
	// Check for video patterns
	videoPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtu\.be/)([a-zA-Z0-9_-]{11})`),
		regexp.MustCompile(`youtube\.com/embed/([a-zA-Z0-9_-]{11})`),
		regexp.MustCompile(`youtube\.com/v/([a-zA-Z0-9_-]{11})`),
	}
	for _, pattern := range videoPatterns {
		if pattern.MatchString(url) {
			return true
		}
	}
	return false
}

// IsYouTubePlaylist checks if a URL is a YouTube playlist URL.
func IsYouTubePlaylist(url string) bool {
	if !IsYouTubeURL(url) {
		return false
	}
	// Check for playlist patterns
	playlistPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[?&]list=([a-zA-Z0-9_-]+)`),
		regexp.MustCompile(`youtube\.com/playlist\?list=([a-zA-Z0-9_-]+)`),
	}
	for _, pattern := range playlistPatterns {
		if pattern.MatchString(url) {
			return true
		}
	}
	return false
}

// ExtractYouTubeVideoID extracts the video ID from a YouTube URL.
func ExtractYouTubeVideoID(url string) string {
	if !IsYouTubeVideo(url) {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`(?:youtube\.com/watch\?v=|youtu\.be/)([a-zA-Z0-9_-]{11})`),
		regexp.MustCompile(`youtube\.com/embed/([a-zA-Z0-9_-]{11})`),
		regexp.MustCompile(`youtube\.com/v/([a-zA-Z0-9_-]{11})`),
	}
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// ExtractYouTubePlaylistID extracts the playlist ID from a YouTube URL.
func ExtractYouTubePlaylistID(url string) string {
	if !IsYouTubePlaylist(url) {
		return ""
	}
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`[?&]list=([a-zA-Z0-9_-]+)`),
		regexp.MustCompile(`youtube\.com/playlist\?list=([a-zA-Z0-9_-]+)`),
	}
	for _, pattern := range patterns {
		matches := pattern.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}
