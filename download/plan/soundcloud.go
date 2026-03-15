package plan

import (
	"regexp"
	"strings"
)

var (
	// soundcloud.com/artist/track-slug
	soundcloudTrackPattern = regexp.MustCompile(`(?i)soundcloud\.com/([^/]+)/([^/?#]+)`)
	// soundcloud.com/artist/sets/set-slug
	soundcloudSetPattern = regexp.MustCompile(`(?i)soundcloud\.com/([^/]+)/sets/([^/?#]+)`)
	// soundcloud.com/artist (user/artist page)
	soundcloudUserPattern = regexp.MustCompile(`(?i)soundcloud\.com/([^/]+)/?$`)
)

// IsSoundCloudURL checks if a URL is a SoundCloud URL.
func IsSoundCloudURL(url string) bool {
	if url == "" {
		return false
	}
	urlLower := strings.ToLower(url)
	return strings.Contains(urlLower, "soundcloud.com")
}

// IsSoundCloudTrack checks if a URL is a SoundCloud track URL.
// Matches: soundcloud.com/artist/track (but NOT soundcloud.com/artist/sets/...)
func IsSoundCloudTrack(url string) bool {
	if !IsSoundCloudURL(url) {
		return false
	}
	if soundcloudSetPattern.MatchString(url) {
		return false
	}
	return soundcloudTrackPattern.MatchString(url) && !soundcloudUserPattern.MatchString(url)
}

// IsSoundCloudSet checks if a URL is a SoundCloud set/playlist URL.
func IsSoundCloudSet(url string) bool {
	if !IsSoundCloudURL(url) {
		return false
	}
	return soundcloudSetPattern.MatchString(url)
}

// IsSoundCloudUser checks if a URL is a SoundCloud user/artist page URL.
func IsSoundCloudUser(url string) bool {
	if !IsSoundCloudURL(url) {
		return false
	}
	return soundcloudUserPattern.MatchString(url)
}

// ExtractSoundCloudSlug extracts the "artist/track" slug from a SoundCloud track URL.
func ExtractSoundCloudSlug(url string) string {
	if !IsSoundCloudTrack(url) {
		return ""
	}
	matches := soundcloudTrackPattern.FindStringSubmatch(url)
	if len(matches) > 2 {
		return matches[1] + "/" + matches[2]
	}
	return ""
}
