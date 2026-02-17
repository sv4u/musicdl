package spotify

import (
	"sync"
	"time"
)

// RateLimitInfo holds information about an active rate limit.
type RateLimitInfo struct {
	Active              bool
	RetryAfterSeconds   int
	RetryAfterTimestamp int64
	DetectedAt          int64
}

// RateLimitTracker tracks active rate limits for status reporting.
type RateLimitTracker struct {
	mu            sync.RWMutex
	rateLimitInfo *RateLimitInfo
}

// NewRateLimitTracker creates a new rate limit tracker.
func NewRateLimitTracker() *RateLimitTracker {
	return &RateLimitTracker{}
}

// Update updates the rate limit state with retry-after information.
func (t *RateLimitTracker) Update(retryAfterSeconds int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now().Unix()
	expiresAt := now + int64(retryAfterSeconds)

	t.rateLimitInfo = &RateLimitInfo{
		Active:              true,
		RetryAfterSeconds:   retryAfterSeconds,
		RetryAfterTimestamp: expiresAt,
		DetectedAt:          now,
	}
}

// GetInfo returns the current rate limit state, or nil if expired or not active.
// Uses a single write lock to atomically check expiration and clear, avoiding a
// TOCTOU race where Update() between an unlock/re-lock could be incorrectly wiped.
func (t *RateLimitTracker) GetInfo() *RateLimitInfo {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.rateLimitInfo == nil {
		return nil
	}

	// Check if expired and clear atomically
	if time.Now().Unix() >= t.rateLimitInfo.RetryAfterTimestamp {
		t.rateLimitInfo = nil
		return nil
	}

	// Return a copy so callers cannot mutate internal state
	info := *t.rateLimitInfo
	return &info
}

// Clear clears the rate limit state.
func (t *RateLimitTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rateLimitInfo = nil
}
