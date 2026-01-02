"""
Unit tests for TTLCache.
"""
import threading
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

    def test_cache_statistics_hits(self):
        """Test cache statistics tracking for hits."""
        cache = TTLCache(max_size=10, ttl_seconds=3600)
        
        # Initially no hits or misses
        stats = cache.stats()
        assert stats["hits"] == 0
        assert stats["misses"] == 0
        assert stats["hit_rate"] == "0.00%"
        
        # Set and get - should be a hit
        cache.set("key1", "value1")
        cache.get("key1")
        
        stats = cache.stats()
        assert stats["hits"] == 1
        assert stats["misses"] == 0
        assert stats["hit_rate"] == "100.00%"

    def test_cache_statistics_misses(self):
        """Test cache statistics tracking for misses."""
        cache = TTLCache(max_size=10, ttl_seconds=3600)
        
        # Get non-existent key - should be a miss
        cache.get("nonexistent")
        
        stats = cache.stats()
        assert stats["hits"] == 0
        assert stats["misses"] == 1
        assert stats["hit_rate"] == "0.00%"

    def test_cache_statistics_hit_rate(self):
        """Test cache statistics hit rate calculation."""
        cache = TTLCache(max_size=10, ttl_seconds=3600)
        
        # Set up some keys
        cache.set("key1", "value1")
        cache.set("key2", "value2")
        
        # 2 hits, 2 misses
        cache.get("key1")  # hit
        cache.get("key2")  # hit
        cache.get("nonexistent1")  # miss
        cache.get("nonexistent2")  # miss
        
        stats = cache.stats()
        assert stats["hits"] == 2
        assert stats["misses"] == 2
        assert stats["hit_rate"] == "50.00%"

    def test_cache_statistics_expired_misses(self):
        """Test that expired entries count as misses."""
        cache = TTLCache(max_size=10, ttl_seconds=1)
        
        cache.set("key1", "value1")
        cache.get("key1")  # hit
        
        # Wait for expiration
        time.sleep(1.1)
        
        cache.get("key1")  # miss (expired)
        
        stats = cache.stats()
        assert stats["hits"] == 1
        assert stats["misses"] == 1
        assert stats["hit_rate"] == "50.00%"

    def test_cache_statistics_reset(self):
        """Test resetting statistics counters."""
        cache = TTLCache(max_size=10, ttl_seconds=3600)
        
        cache.set("key1", "value1")
        cache.get("key1")  # hit
        cache.get("nonexistent")  # miss
        
        stats = cache.stats()
        assert stats["hits"] == 1
        assert stats["misses"] == 1
        
        # Reset stats
        cache.reset_stats()
        
        stats = cache.stats()
        assert stats["hits"] == 0
        assert stats["misses"] == 0
        assert stats["hit_rate"] == "0.00%"
        
        # Cache should still have data
        assert cache.get("key1") == "value1"

    def test_cache_statistics_structure(self):
        """Test that stats() returns expected structure."""
        cache = TTLCache(max_size=100, ttl_seconds=7200)
        
        stats = cache.stats()
        
        assert "size" in stats
        assert "max_size" in stats
        assert "ttl_seconds" in stats
        assert "hits" in stats
        assert "misses" in stats
        assert "hit_rate" in stats
        
        assert stats["max_size"] == 100
        assert stats["ttl_seconds"] == 7200
        assert isinstance(stats["hit_rate"], str)
        assert stats["hit_rate"].endswith("%")

    def test_cache_thread_safety_concurrent_reads(self):
        """Test thread safety with concurrent read operations."""
        cache = TTLCache(max_size=100, ttl_seconds=3600)
        
        # Set up cache with some data
        for i in range(10):
            cache.set(f"key{i}", f"value{i}")
        
        results = []
        errors = []
        
        def read_cache(thread_id: int) -> None:
            """Read from cache in thread."""
            try:
                for i in range(10):
                    value = cache.get(f"key{i}")
                    if value:
                        results.append((thread_id, i, value))
            except Exception as e:
                errors.append(e)
        
        # Create multiple threads
        threads = []
        for i in range(5):
            thread = threading.Thread(target=read_cache, args=(i,))
            threads.append(thread)
        
        # Start all threads
        for thread in threads:
            thread.start()
        
        # Wait for all threads
        for thread in threads:
            thread.join()
        
        # Should have no errors
        assert len(errors) == 0
        
        # Should have read all values successfully
        assert len(results) == 50  # 5 threads * 10 keys

    def test_cache_thread_safety_concurrent_writes(self):
        """Test thread safety with concurrent write operations."""
        cache = TTLCache(max_size=100, ttl_seconds=3600)
        
        errors = []
        
        def write_cache(thread_id: int) -> None:
            """Write to cache in thread."""
            try:
                for i in range(10):
                    cache.set(f"key_{thread_id}_{i}", f"value_{thread_id}_{i}")
            except Exception as e:
                errors.append(e)
        
        # Create multiple threads
        threads = []
        for i in range(5):
            thread = threading.Thread(target=write_cache, args=(i,))
            threads.append(thread)
        
        # Start all threads
        for thread in threads:
            thread.start()
        
        # Wait for all threads
        for thread in threads:
            thread.join()
        
        # Should have no errors
        assert len(errors) == 0
        
        # Verify all values were written
        for thread_id in range(5):
            for i in range(10):
                value = cache.get(f"key_{thread_id}_{i}")
                assert value == f"value_{thread_id}_{i}"

    def test_cache_thread_safety_mixed_operations(self):
        """Test thread safety with mixed read/write operations."""
        cache = TTLCache(max_size=50, ttl_seconds=3600)
        
        errors = []
        read_count = [0]
        write_count = [0]
        
        def read_operation() -> None:
            """Read from cache."""
            try:
                for i in range(20):
                    value = cache.get(f"key{i}")
                    if value:
                        read_count[0] += 1
            except Exception as e:
                errors.append(e)
        
        def write_operation() -> None:
            """Write to cache."""
            try:
                for i in range(20):
                    cache.set(f"key{i}", f"value{i}")
                    write_count[0] += 1
            except Exception as e:
                errors.append(e)
        
        # Create threads for both operations
        threads = []
        for _ in range(3):
            threads.append(threading.Thread(target=read_operation))
            threads.append(threading.Thread(target=write_operation))
        
        # Start all threads
        for thread in threads:
            thread.start()
        
        # Wait for all threads
        for thread in threads:
            thread.join()
        
        # Should have no errors
        assert len(errors) == 0
        
        # Verify cache is in consistent state
        stats = cache.stats()
        assert stats["size"] <= cache.max_size

