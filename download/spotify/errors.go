package spotify

import "fmt"

// RateLimitError represents a rate limit error from Spotify API.
type RateLimitError struct {
	RetryAfter int    // Seconds to wait before retrying
	Original   error  // Original error from spotigo
}

func (e *RateLimitError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("Spotify API rate limited: retry after %d seconds: %v", e.RetryAfter, e.Original)
	}
	return fmt.Sprintf("Spotify API rate limited: %v", e.Original)
}

func (e *RateLimitError) Unwrap() error {
	return e.Original
}

// SpotifyError represents a general Spotify API error.
type SpotifyError struct {
	Message string
	Original error
}

func (e *SpotifyError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("Spotify API error: %s: %v", e.Message, e.Original)
	}
	return fmt.Sprintf("Spotify API error: %s", e.Message)
}

func (e *SpotifyError) Unwrap() error {
	return e.Original
}
