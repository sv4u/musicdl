package plan

import (
	"regexp"
	"strings"
)

var (
	// artist.bandcamp.com/track/track-slug
	bandcampTrackPattern = regexp.MustCompile(`(?i)([a-z0-9-]+)\.bandcamp\.com/track/([^/?#]+)`)
	// artist.bandcamp.com/album/album-slug
	bandcampAlbumPattern = regexp.MustCompile(`(?i)([a-z0-9-]+)\.bandcamp\.com/album/([^/?#]+)`)
	// artist.bandcamp.com (artist/label page — no /track/ or /album/)
	bandcampArtistPattern = regexp.MustCompile(`(?i)^https?://([a-z0-9-]+)\.bandcamp\.com/?$`)
)

// IsBandcampURL checks if a URL is a Bandcamp URL.
func IsBandcampURL(url string) bool {
	if url == "" {
		return false
	}
	urlLower := strings.ToLower(url)
	return strings.Contains(urlLower, "bandcamp.com")
}

// IsBandcampTrack checks if a URL is a Bandcamp track URL.
func IsBandcampTrack(url string) bool {
	if !IsBandcampURL(url) {
		return false
	}
	return bandcampTrackPattern.MatchString(url)
}

// IsBandcampAlbum checks if a URL is a Bandcamp album URL.
func IsBandcampAlbum(url string) bool {
	if !IsBandcampURL(url) {
		return false
	}
	return bandcampAlbumPattern.MatchString(url)
}

// IsBandcampArtist checks if a URL is a Bandcamp artist/label page URL.
func IsBandcampArtist(url string) bool {
	if !IsBandcampURL(url) {
		return false
	}
	return bandcampArtistPattern.MatchString(url)
}

// ExtractBandcampArtist extracts the artist subdomain from a Bandcamp URL.
func ExtractBandcampArtist(url string) string {
	if !IsBandcampURL(url) {
		return ""
	}
	for _, pat := range []*regexp.Regexp{bandcampTrackPattern, bandcampAlbumPattern, bandcampArtistPattern} {
		matches := pat.FindStringSubmatch(url)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return ""
}

// ExtractBandcampTrackSlug extracts the track slug from a Bandcamp track URL.
func ExtractBandcampTrackSlug(url string) string {
	matches := bandcampTrackPattern.FindStringSubmatch(url)
	if len(matches) > 2 {
		return matches[2]
	}
	return ""
}

// ExtractBandcampAlbumSlug extracts the album slug from a Bandcamp album URL.
func ExtractBandcampAlbumSlug(url string) string {
	matches := bandcampAlbumPattern.FindStringSubmatch(url)
	if len(matches) > 2 {
		return matches[2]
	}
	return ""
}
