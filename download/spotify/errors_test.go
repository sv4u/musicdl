package spotify

import (
	"errors"
	"testing"
)

func TestRateLimitError(t *testing.T) {
	original := errors.New("HTTP 429")
	err := &RateLimitError{
		RetryAfter: 10,
		Original:   original,
	}

	if err.Error() == "" {
		t.Error("RateLimitError.Error() should return non-empty string")
	}

	if err.RetryAfter != 10 {
		t.Errorf("Expected RetryAfter 10, got %d", err.RetryAfter)
	}

	unwrapped := err.Unwrap()
	if unwrapped != original {
		t.Error("Unwrap() should return original error")
	}
}

func TestSpotifyError(t *testing.T) {
	original := errors.New("network error")
	err := &SpotifyError{
		Message:  "failed to get track",
		Original: original,
	}

	if err.Error() == "" {
		t.Error("SpotifyError.Error() should return non-empty string")
	}

	unwrapped := err.Unwrap()
	if unwrapped != original {
		t.Error("Unwrap() should return original error")
	}
}

func TestSpotifyError_NoOriginal(t *testing.T) {
	err := &SpotifyError{
		Message: "invalid request",
	}

	if err.Error() == "" {
		t.Error("SpotifyError.Error() should return non-empty string")
	}

	if err.Unwrap() != nil {
		t.Error("Unwrap() should return nil when no original error")
	}
}
