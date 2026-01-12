package spotify

import (
	"testing"
	"time"
)

func TestTTLCache_Size(t *testing.T) {
	cache := NewTTLCache(10, 3600)

	// Initially empty
	if cache.Size() != 0 {
		t.Errorf("Expected size 0, got %d", cache.Size())
	}

	// Add entries
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}

	// Clear and verify
	cache.Clear()
	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", cache.Size())
	}
}

func TestTTLCache_StartCleanup(t *testing.T) {
	cache := NewTTLCache(10, 1) // 1 second TTL

	// Start cleanup
	cache.StartCleanup(100 * time.Millisecond)

	// Add entry that will expire
	cache.Set("key1", "value1")

	// Wait for expiration and cleanup
	time.Sleep(1200 * time.Millisecond)

	// Entry should be removed
	if cache.Size() != 0 {
		t.Errorf("Expected expired entry to be removed, but size is %d", cache.Size())
	}

	// Stop cleanup
	cache.StopCleanup()
}

func TestTTLCache_StopCleanup(t *testing.T) {
	cache := NewTTLCache(10, 3600)

	// Start cleanup
	cache.StartCleanup(100 * time.Millisecond)

	// Stop cleanup (should not panic)
	cache.StopCleanup()

	// Stop again (should not panic)
	cache.StopCleanup()
}

func TestTTLCache_CleanupExpired(t *testing.T) {
	cache := NewTTLCache(10, 1) // 1 second TTL

	// Add entry
	cache.Set("key1", "value1")

	// Verify it exists
	if cache.Size() != 1 {
		t.Errorf("Expected size 1, got %d", cache.Size())
	}

	// Wait for expiration
	time.Sleep(1100 * time.Millisecond)

	// Manually trigger cleanup
	cache.cleanupExpired()

	// Entry should be removed
	if cache.Size() != 0 {
		t.Errorf("Expected expired entry to be removed, but size is %d", cache.Size())
	}
}

func TestTTLCache_StartCleanup_AlreadyRunning(t *testing.T) {
	cache := NewTTLCache(10, 3600)

	// Start cleanup
	cache.StartCleanup(100 * time.Millisecond)

	// Start again (should not create duplicate goroutines)
	cache.StartCleanup(100 * time.Millisecond)

	// Stop cleanup
	cache.StopCleanup()
}
