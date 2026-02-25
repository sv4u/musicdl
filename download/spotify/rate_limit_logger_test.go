package spotify

import (
	"testing"
	"time"
)

// TestRateLimitLogger_Warn_UpdatesTrackerOnRateLimitMessage verifies that when
// the logger receives spotigo's rate limit warning (logged during 429 retry),
// the RateLimitTracker is updated so the web UI can display the active rate limit.
func TestRateLimitLogger_Warn_UpdatesTrackerOnRateLimitMessage(t *testing.T) {
	tracker := NewRateLimitTracker()
	logger := &rateLimitLogger{tracker: tracker}

	// Simulate spotigo's logRetry call for 429 - exact format from spotigo client.go
	logger.Warn("Your application has reached a rate/request limit. Retry will occur after: %.0f s", 35707.0)

	info := tracker.GetInfo()
	if info == nil {
		t.Fatal("Expected rate limit info after Warn, got nil")
	}
	if !info.Active {
		t.Error("Expected Active to be true")
	}
	if info.RetryAfterSeconds != 35707 {
		t.Errorf("Expected RetryAfterSeconds 35707, got %d", info.RetryAfterSeconds)
	}
	if info.RetryAfterTimestamp <= time.Now().Unix() {
		t.Error("RetryAfterTimestamp should be in the future")
	}
}

// TestRateLimitLogger_Warn_IgnoresNonRateLimitMessages verifies that other
// Warn messages do not update the tracker.
func TestRateLimitLogger_Warn_IgnoresNonRateLimitMessages(t *testing.T) {
	tracker := NewRateLimitTracker()
	logger := &rateLimitLogger{tracker: tracker}

	logger.Warn("Retry attempt %d after %v: %v", 1, "5s", "connection refused")

	info := tracker.GetInfo()
	if info != nil {
		t.Errorf("Expected nil (no rate limit), got info with RetryAfterSeconds=%d", info.RetryAfterSeconds)
	}
}

// TestRateLimitLogger_Warn_HandlesMissingArgs verifies that the rate limit
// message with no args does not panic and does not update the tracker.
func TestRateLimitLogger_Warn_HandlesMissingArgs(t *testing.T) {
	tracker := NewRateLimitTracker()
	logger := &rateLimitLogger{tracker: tracker}

	logger.Warn("Your application has reached a rate/request limit. Retry will occur after: %.0f s")
	// No panic; tracker should remain empty
	info := tracker.GetInfo()
	if info != nil {
		t.Errorf("Expected nil with missing args, got info")
	}
}

// TestRateLimitLogger_Warn_IgnoresZeroOrNegativeSeconds verifies that zero
// or negative retry-after values do not update the tracker.
func TestRateLimitLogger_Warn_IgnoresZeroOrNegativeSeconds(t *testing.T) {
	tracker := NewRateLimitTracker()
	logger := &rateLimitLogger{tracker: tracker}

	logger.Warn("Your application has reached a rate/request limit. Retry will occur after: %.0f s", 0.0)
	info := tracker.GetInfo()
	if info != nil {
		t.Error("Expected nil for 0 seconds, got info")
	}

	logger.Warn("Your application has reached a rate/request limit. Retry will occur after: %.0f s", -10.0)
	info = tracker.GetInfo()
	if info != nil {
		t.Error("Expected nil for negative seconds, got info")
	}
}

// TestRateLimitLogger_ClientIntegration verifies that when the rate limit
// logger (used internally by SpotifyClient) receives spotigo's rate limit
// warning, the client's GetRateLimitInfo returns the updated state. This
// ensures the /api/rate-limit-status endpoint and web UI display correctly.
func TestRateLimitLogger_ClientIntegration(t *testing.T) {
	config := &Config{
		ClientID:     "test_id",
		ClientSecret:  "test_secret",
	}
	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	// Simulate spotigo calling the logger during 429 retry - the client uses
	// rateLimitLogger with the same tracker, so this mirrors real behavior
	logger := &rateLimitLogger{tracker: client.rateLimitTracker}
	logger.Warn("Your application has reached a rate/request limit. Retry will occur after: %.0f s", 90.0)

	info := client.GetRateLimitInfo()
	if info == nil {
		t.Fatal("GetRateLimitInfo() should return info when logger updated tracker")
	}
	if info.RetryAfterSeconds != 90 {
		t.Errorf("Expected RetryAfterSeconds 90, got %d", info.RetryAfterSeconds)
	}
}

// TestToIntSeconds verifies the helper correctly converts various numeric types.
func TestToIntSeconds(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected int
		ok       bool
	}{
		{"float64", 35707.0, 35707, true},
		{"int", 120, 120, true},
		{"int64", int64(60), 60, true},
		{"string", "30", 0, false},
		{"nil", nil, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toIntSeconds(tt.input)
			if ok != tt.ok || got != tt.expected {
				t.Errorf("toIntSeconds(%v) = (%d, %v), want (%d, %v)", tt.input, got, ok, tt.expected, tt.ok)
			}
		})
	}
}
