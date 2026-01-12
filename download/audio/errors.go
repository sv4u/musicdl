package audio

import "fmt"

// DownloadError represents an audio download error.
type DownloadError struct {
	Message string
	Original error
}

func (e *DownloadError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("Audio download error: %s: %v", e.Message, e.Original)
	}
	return fmt.Sprintf("Audio download error: %s", e.Message)
}

func (e *DownloadError) Unwrap() error {
	return e.Original
}

// SearchError represents an audio search error.
type SearchError struct {
	Message string
	Original error
}

func (e *SearchError) Error() string {
	if e.Original != nil {
		return fmt.Sprintf("Audio search error: %s: %v", e.Message, e.Original)
	}
	return fmt.Sprintf("Audio search error: %s", e.Message)
}

func (e *SearchError) Unwrap() error {
	return e.Original
}
