package spotify

import (
	"testing"
	"time"
)

func TestRateLimitTracker_Update(t *testing.T) {
	tracker := NewRateLimitTracker()

	tracker.Update(10)
	info := tracker.GetInfo()

	if info == nil {
		t.Fatal("Expected rate limit info, got nil")
	}
	if !info.Active {
		t.Error("Expected Active to be true")
	}
	if info.RetryAfterSeconds != 10 {
		t.Errorf("Expected RetryAfterSeconds 10, got %d", info.RetryAfterSeconds)
	}
	if info.RetryAfterTimestamp <= time.Now().Unix() {
		t.Error("RetryAfterTimestamp should be in the future")
	}
}

func TestRateLimitTracker_Expiration(t *testing.T) {
	tracker := NewRateLimitTracker()

	// Set a very short rate limit (1 second)
	tracker.Update(1)
	info := tracker.GetInfo()
	if info == nil {
		t.Fatal("Expected rate limit info, got nil")
	}

	// Wait for expiration
	time.Sleep(1100 * time.Millisecond)

	// Should return nil after expiration
	info = tracker.GetInfo()
	if info != nil {
		t.Error("Expected nil after expiration, got info")
	}
}

func TestRateLimitTracker_Clear(t *testing.T) {
	tracker := NewRateLimitTracker()

	tracker.Update(10)
	info := tracker.GetInfo()
	if info == nil {
		t.Fatal("Expected rate limit info, got nil")
	}

	tracker.Clear()
	info = tracker.GetInfo()
	if info != nil {
		t.Error("Expected nil after clear, got info")
	}
}

func TestRateLimitTracker_GetInfo_ReturnsCopy(t *testing.T) {
	tracker := NewRateLimitTracker()

	tracker.Update(10)
	info1 := tracker.GetInfo()
	info2 := tracker.GetInfo()

	// Should be different pointers (copies)
	if info1 == info2 {
		t.Error("GetInfo should return copies, not same pointer")
	}

	// But same values
	if info1.RetryAfterSeconds != info2.RetryAfterSeconds {
		t.Error("Copies should have same values")
	}
}

// TestRateLimitTracker_GetInfo_ExpiredDeadlock tests the fix for ISSUE-1:
// Ensures GetInfo() doesn't deadlock when rate limit is expired.
// This test specifically verifies the fix for the double-unlock bug.
func TestRateLimitTracker_GetInfo_ExpiredDeadlock(t *testing.T) {
	tracker := NewRateLimitTracker()

	// Set a very short rate limit (1 second)
	tracker.Update(1)

	// Wait for expiration
	time.Sleep(1100 * time.Millisecond)

	// This should not deadlock - test with timeout
	done := make(chan bool, 1)
	go func() {
		info := tracker.GetInfo()
		_ = info // Use the result to prevent optimization
		done <- true
	}()

	select {
	case <-done:
		// Success - no deadlock
	case <-time.After(5 * time.Second):
		t.Fatal("GetInfo() deadlocked when rate limit was expired")
	}
}

// TestRateLimitTracker_GetInfo_ConcurrentExpired tests concurrent access
// when rate limit expires to ensure no race conditions or deadlocks.
func TestRateLimitTracker_GetInfo_ConcurrentExpired(t *testing.T) {
	tracker := NewRateLimitTracker()

	// Set a very short rate limit
	tracker.Update(1)

	// Wait for expiration
	time.Sleep(1100 * time.Millisecond)

	// Launch multiple concurrent GetInfo() calls
	const numGoroutines = 10
	done := make(chan bool, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			info := tracker.GetInfo()
			_ = info
			done <- true
		}()
	}

	// Wait for all goroutines to complete
	timeout := time.After(5 * time.Second)
	for i := 0; i < numGoroutines; i++ {
		select {
		case <-done:
			// Success
		case <-timeout:
			t.Fatal("Concurrent GetInfo() calls deadlocked or timed out")
		}
	}
}
