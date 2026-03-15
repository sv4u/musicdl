package plan

import (
	"regexp"
	"strings"
)

var (
	// audius.co/artist/track-slug
	audiusTrackPattern = regexp.MustCompile(`(?i)audius\.co/([^/]+)/([^/?#]+)`)
	// audius.co/artist/playlist/playlist-slug
	audiusPlaylistPattern = regexp.MustCompile(`(?i)audius\.co/([^/]+)/playlist/([^/?#]+)`)
	// audius.co/artist (user page — no second path segment beyond username)
	audiusUserPattern = regexp.MustCompile(`(?i)^https?://audius\.co/([^/]+)/?$`)
)

// IsAudiusURL checks if a URL is an Audius URL.
func IsAudiusURL(url string) bool {
	if url == "" {
		return false
	}
	urlLower := strings.ToLower(url)
	return strings.Contains(urlLower, "audius.co")
}

// IsAudiusTrack checks if a URL is an Audius track URL.
// Matches: audius.co/artist/track (but NOT audius.co/artist/playlist/...)
func IsAudiusTrack(url string) bool {
	if !IsAudiusURL(url) {
		return false
	}
	if audiusPlaylistPattern.MatchString(url) {
		return false
	}
	return audiusTrackPattern.MatchString(url) && !audiusUserPattern.MatchString(url)
}

// IsAudiusPlaylist checks if a URL is an Audius playlist URL.
func IsAudiusPlaylist(url string) bool {
	if !IsAudiusURL(url) {
		return false
	}
	return audiusPlaylistPattern.MatchString(url)
}

// IsAudiusUser checks if a URL is an Audius user page URL.
func IsAudiusUser(url string) bool {
	if !IsAudiusURL(url) {
		return false
	}
	return audiusUserPattern.MatchString(url)
}

// ExtractAudiusSlug extracts the "artist/track" slug from an Audius track URL.
func ExtractAudiusSlug(url string) string {
	if !IsAudiusTrack(url) {
		return ""
	}
	matches := audiusTrackPattern.FindStringSubmatch(url)
	if len(matches) > 2 {
		return matches[1] + "/" + matches[2]
	}
	return ""
}
