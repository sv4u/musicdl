"""
Thread-safe in-memory cache with TTL expiration and LRU eviction.
"""

import threading
import time
from collections import OrderedDict
from typing import Any, Dict, Optional


class TTLCache:
    """
    Thread-safe in-memory cache with TTL expiration and LRU eviction.
    
    Features:
    - Thread-safe operations using RLock
    - TTL-based expiration
    - LRU eviction when max_size is reached
    - Cache statistics (hits, misses, hit rate)
    """

    def __init__(self, max_size: int = 1000, ttl_seconds: int = 3600):
        """
        Initialize cache.

        Args:
            max_size: Maximum number of entries (LRU eviction when exceeded)
            ttl_seconds: Time-to-live in seconds (entries expire after this time)
        """
        self.max_size = max_size
        self.ttl_seconds = ttl_seconds
        self._cache: OrderedDict[str, tuple[float, Any]] = OrderedDict()
        self._lock = threading.RLock()  # Reentrant lock for nested calls
        self._hits = 0
        self._misses = 0

    def get(self, key: str) -> Optional[Any]:
        """
        Get value from cache if not expired (thread-safe).

        Args:
            key: Cache key

        Returns:
            Cached value if found and not expired, None otherwise
        """
        with self._lock:
            if key not in self._cache:
                self._misses += 1
                return None

            timestamp, value = self._cache[key]

            # Check if expired
            if time.time() - timestamp > self.ttl_seconds:
                del self._cache[key]
                self._misses += 1
                return None

            # Move to end (most recently used)
            self._cache.move_to_end(key)
            self._hits += 1
            return value

    def set(self, key: str, value: Any) -> None:
        """
        Set value in cache with current timestamp (thread-safe).

        Args:
            key: Cache key
            value: Value to cache
        """
        with self._lock:
            # Remove if exists
            if key in self._cache:
                del self._cache[key]

            # Evict oldest if at capacity
            if len(self._cache) >= self.max_size:
                self._cache.popitem(last=False)  # Remove oldest (first item)

            # Add new entry
            self._cache[key] = (time.time(), value)

    def clear(self) -> None:
        """
        Clear all cache entries (thread-safe).
        
        Note: This does not reset statistics counters.
        """
        with self._lock:
            self._cache.clear()

    def cleanup_expired(self) -> None:
        """
        Remove all expired entries (thread-safe).
        """
        with self._lock:
            current_time = time.time()
            expired_keys = [
                key
                for key, (timestamp, _) in self._cache.items()
                if current_time - timestamp > self.ttl_seconds
            ]
            for key in expired_keys:
                del self._cache[key]

    def stats(self) -> Dict[str, Any]:
        """
        Get cache statistics (thread-safe).

        Returns:
            Dictionary with cache statistics including:
            - size: Current number of entries
            - max_size: Maximum cache size
            - ttl_seconds: Time-to-live in seconds
            - hits: Number of cache hits
            - misses: Number of cache misses
            - hit_rate: Hit rate as percentage string (e.g., "75.50%")
        """
        with self._lock:
            total = self._hits + self._misses
            hit_rate = (self._hits / total * 100) if total > 0 else 0.0

            return {
                "size": len(self._cache),
                "max_size": self.max_size,
                "ttl_seconds": self.ttl_seconds,
                "hits": self._hits,
                "misses": self._misses,
                "hit_rate": f"{hit_rate:.2f}%",
            }

    def reset_stats(self) -> None:
        """
        Reset statistics counters (thread-safe).
        
        This resets hits and misses counters but does not clear the cache.
        """
        with self._lock:
            self._hits = 0
            self._misses = 0

