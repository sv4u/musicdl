"""
Unit tests for TTLCache.
"""
import time
import pytest
from core.cache import TTLCache


class TestTTLCache:
    """Test suite for TTLCache implementation."""
    
    def test_cache_set_and_get(self):
        """Test basic cache set and get operations."""
        cache = TTLCache(max_size=10, ttl_seconds=3600)
        cache.set("key1", "value1")
        assert cache.get("key1") == "value1"
        assert cache.get("nonexistent") is None
    
    def test_cache_ttl_expiration(self):
        """Test that entries expire after TTL."""
        cache = TTLCache(max_size=10, ttl_seconds=1)  # 1 second TTL
        cache.set("key1", "value1")
        assert cache.get("key1") == "value1"
        
        # Wait for expiration
        time.sleep(1.1)
        assert cache.get("key1") is None
    
    def test_cache_lru_eviction(self):
        """Test LRU eviction when max_size is reached."""
        cache = TTLCache(max_size=3, ttl_seconds=3600)
        
        # Fill cache to capacity
        cache.set("key1", "value1")
        cache.set("key2", "value2")
        cache.set("key3", "value3")
        
        # All should be present
        assert cache.get("key1") == "value1"
        assert cache.get("key2") == "value2"
        assert cache.get("key3") == "value3"
        
        # Add one more - should evict oldest (key1)
        cache.set("key4", "value4")
        assert cache.get("key1") is None  # Evicted
        assert cache.get("key2") == "value2"
        assert cache.get("key3") == "value3"
        assert cache.get("key4") == "value4"
    
    def test_cache_lru_reordering(self):
        """Test that accessing a key moves it to end (most recently used)."""
        cache = TTLCache(max_size=3, ttl_seconds=3600)
        cache.set("key1", "value1")
        cache.set("key2", "value2")
        cache.set("key3", "value3")
        
        # Access key1 to make it most recently used
        cache.get("key1")
        
        # Add new key - should evict key2 (oldest unused)
        cache.set("key4", "value4")
        assert cache.get("key1") == "value1"  # Still present (was accessed)
        assert cache.get("key2") is None  # Evicted
        assert cache.get("key3") == "value3"
        assert cache.get("key4") == "value4"
    
    def test_cache_clear(self):
        """Test that clear removes all entries."""
        cache = TTLCache(max_size=10, ttl_seconds=3600)
        cache.set("key1", "value1")
        cache.set("key2", "value2")
        
        cache.clear()
        assert cache.get("key1") is None
        assert cache.get("key2") is None
    
    def test_cache_cleanup_expired(self):
        """Test cleanup_expired removes expired entries."""
        cache = TTLCache(max_size=10, ttl_seconds=1)
        cache.set("key1", "value1")
        cache.set("key2", "value2")
        
        # Wait for expiration
        time.sleep(1.1)
        
        # Cleanup should remove expired entries
        cache.cleanup_expired()
        assert cache.get("key1") is None
        assert cache.get("key2") is None
    
    def test_cache_update_existing_key(self):
        """Test that setting existing key updates it."""
        cache = TTLCache(max_size=10, ttl_seconds=3600)
        cache.set("key1", "value1")
        cache.set("key1", "value1_updated")
        assert cache.get("key1") == "value1_updated"
    
    def test_cache_empty_cache(self):
        """Test operations on empty cache."""
        cache = TTLCache(max_size=10, ttl_seconds=3600)
        assert cache.get("any_key") is None
        cache.clear()  # Should not raise
    
    def test_cache_single_entry(self):
        """Test cache with single entry."""
        cache = TTLCache(max_size=1, ttl_seconds=3600)
        cache.set("key1", "value1")
        assert cache.get("key1") == "value1"
        
        # Adding second should evict first
        cache.set("key2", "value2")
        assert cache.get("key1") is None
        assert cache.get("key2") == "value2"
    
    def test_cache_max_size_boundary(self):
        """Test cache behavior at max_size boundary."""
        cache = TTLCache(max_size=2, ttl_seconds=3600)
        cache.set("key1", "value1")
        cache.set("key2", "value2")
        
        # At capacity, both should exist
        assert len([k for k in cache._cache.keys()]) == 2
        
        # Adding third should evict first
        cache.set("key3", "value3")
        assert cache.get("key1") is None
        assert cache.get("key2") == "value2"
        assert cache.get("key3") == "value3"
    
    def test_cache_complex_values(self):
        """Test cache with complex value types."""
        cache = TTLCache(max_size=10, ttl_seconds=3600)
        
        # Test with dict
        cache.set("dict_key", {"nested": "value"})
        assert cache.get("dict_key") == {"nested": "value"}
        
        # Test with list
        cache.set("list_key", [1, 2, 3])
        assert cache.get("list_key") == [1, 2, 3]
        
        # Test with None
        cache.set("none_key", None)
        assert cache.get("none_key") is None

