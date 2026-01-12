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
func (t *RateLimitTracker) GetInfo() *RateLimitInfo {
	t.mu.RLock()
	if t.rateLimitInfo == nil {
		t.mu.RUnlock()
		return nil
	}

	// Check if expired
	now := time.Now().Unix()
	expired := now >= t.rateLimitInfo.RetryAfterTimestamp
	t.mu.RUnlock()

	if expired {
		// Clear it (will acquire write lock)
		t.Clear()
		return nil
	}

	// Return copy (need read lock again for safety)
	t.mu.RLock()
	info := *t.rateLimitInfo
	t.mu.RUnlock()
	return &info
}

// Clear clears the rate limit state.
func (t *RateLimitTracker) Clear() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.rateLimitInfo = nil
}
