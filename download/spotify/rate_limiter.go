package spotify

import (
	"context"
	"sync"
	"time"
)

// RateLimiter implements a sliding window rate limiter.
type RateLimiter struct {
	mu           sync.Mutex
	requestTimes []time.Time
	maxRequests  int
	windowSize   time.Duration
	enabled      bool
}

// NewRateLimiter creates a new rate limiter.
func NewRateLimiter(enabled bool, maxRequests int, windowSeconds float64) *RateLimiter {
	return &RateLimiter{
		requestTimes: make([]time.Time, 0),
		maxRequests:  maxRequests,
		windowSize:   time.Duration(windowSeconds * float64(time.Second)),
		enabled:      enabled,
	}
}

// WaitIfNeeded blocks until a request can be made, respecting rate limits.
// If context is provided, it respects context cancellation.
func (rl *RateLimiter) WaitIfNeeded(ctx context.Context) error {
	if !rl.enabled {
		return nil
	}

	for {
		rl.mu.Lock()
		now := time.Now()
		windowStart := now.Add(-rl.windowSize)

		// Remove old requests outside the window
		validTimes := rl.requestTimes[:0]
		for _, t := range rl.requestTimes {
			if t.After(windowStart) {
				validTimes = append(validTimes, t)
			}
		}
		rl.requestTimes = validTimes

		// Check if we can make a request
		if len(rl.requestTimes) < rl.maxRequests {
			rl.requestTimes = append(rl.requestTimes, now)
			rl.mu.Unlock()
			return nil
		}

		// At limit - calculate wait time
		oldest := rl.requestTimes[0]
		waitTime := rl.windowSize - now.Sub(oldest)
		rl.mu.Unlock()

		if waitTime <= 0 {
			// Oldest request expired, re-check immediately
			continue
		}

		// Need to wait - check context if provided
		if ctx != nil {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(waitTime):
				// Continue loop to re-check
			}
		} else {
			time.Sleep(waitTime)
		}
	}
}
