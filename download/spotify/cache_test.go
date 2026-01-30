package spotify

import (
	"testing"
	"time"
)

func TestTTLCache_GetSet(t *testing.T) {
	cache := NewTTLCache(10, 3600)

	// Test Set and Get
	cache.Set("key1", "value1")
	value := cache.Get("key1")
	if value != "value1" {
		t.Errorf("Expected 'value1', got %v", value)
	}

	// Test non-existent key
	value = cache.Get("nonexistent")
	if value != nil {
		t.Errorf("Expected nil, got %v", value)
	}
}

func TestTTLCache_Expiration(t *testing.T) {
	cache := NewTTLCache(10, 1) // 1 second TTL

	cache.Set("key1", "value1")
	value := cache.Get("key1")
	if value != "value1" {
		t.Errorf("Expected 'value1', got %v", value)
	}

	// Wait for expiration
	time.Sleep(1100 * time.Millisecond)

	value = cache.Get("key1")
	if value != nil {
		t.Errorf("Expected nil after expiration, got %v", value)
	}
}

func TestTTLCache_LRU_Eviction(t *testing.T) {
	cache := NewTTLCache(3, 3600) // Max size 3

	// Fill cache
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// Verify all are present
	if cache.Get("key1") == nil || cache.Get("key2") == nil || cache.Get("key3") == nil {
		t.Error("All keys should be present")
	}

	// Add one more - should evict LRU (key1, since it wasn't accessed)
	cache.Set("key4", "value4")

	// key1 should be evicted
	if cache.Get("key1") != nil {
		t.Error("key1 should have been evicted")
	}

	// Others should still be present
	if cache.Get("key2") == nil || cache.Get("key3") == nil || cache.Get("key4") == nil {
		t.Error("key2, key3, key4 should still be present")
	}
}

func TestTTLCache_LRU_MoveToFront(t *testing.T) {
	cache := NewTTLCache(3, 3600)

	// Fill cache
	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	// Access key1 to move it to front
	cache.Get("key1")

	// Add new key - should evict key2 (least recently used, not key1)
	cache.Set("key4", "value4")

	// key1 and key3 should still be present (key1 was accessed, key3 is newest)
	if cache.Get("key1") == nil {
		t.Error("key1 should still be present (was accessed)")
	}
	if cache.Get("key3") == nil {
		t.Error("key3 should still be present (newest)")
	}
	if cache.Get("key4") == nil {
		t.Error("key4 should be present (just added)")
	}

	// key2 should be evicted
	if cache.Get("key2") != nil {
		t.Error("key2 should have been evicted")
	}
}

func TestTTLCache_Stats(t *testing.T) {
	cache := NewTTLCache(10, 3600)

	cache.Set("key1", "value1")
	cache.Get("key1")        // Hit
	cache.Get("key1")        // Hit
	cache.Get("nonexistent") // Miss

	stats := cache.Stats()
	if stats.Hits != 2 {
		t.Errorf("Expected 2 hits, got %d", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Expected 1 miss, got %d", stats.Misses)
	}
	if stats.Size != 1 {
		t.Errorf("Expected size 1, got %d", stats.Size)
	}
	if stats.HitRate == 0 {
		t.Error("Hit rate should be > 0")
	}
}

func TestTTLCache_Clear(t *testing.T) {
	cache := NewTTLCache(10, 3600)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	if cache.Size() != 2 {
		t.Errorf("Expected size 2, got %d", cache.Size())
	}

	cache.Clear()

	if cache.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", cache.Size())
	}

	if cache.Get("key1") != nil {
		t.Error("key1 should be cleared")
	}
}

func TestTTLCache_UpdateExisting(t *testing.T) {
	cache := NewTTLCache(10, 3600)

	cache.Set("key1", "value1")
	cache.Set("key1", "value2") // Update

	value := cache.Get("key1")
	if value != "value2" {
		t.Errorf("Expected 'value2', got %v", value)
	}

	if cache.Size() != 1 {
		t.Errorf("Expected size 1, got %d", cache.Size())
	}
}
