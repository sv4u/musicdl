# Plan: Add Comprehensive Caching

## Executive Summary

This plan outlines the expansion of the current caching implementation to cover all expensive operations in the musicdl application. Currently, only Spotify API responses are cached. This plan extends caching to audio search results, downloaded file metadata, and configuration parsing to significantly reduce redundant operations and improve performance.**Target Audience**: Technical leads, junior engineers, and technical management**Estimated Effort**: 4-5 days**Risk Level**: Low-Medium**Priority**: Medium-High (performance and cost optimization)

## Current State Analysis

### Existing Caching Implementation

#### Current Cache: `core/cache.py`

- **Type**: `TTLCache` - In-memory cache with TTL expiration and LRU eviction
- **Features**:
- Time-based expiration (TTL)
- Size-based eviction (LRU)
- Thread-safe operations (basic)
- **Usage**: Currently only used in `SpotifyClient` for API responses

#### Current Cache Usage

```python
# In core/spotify_client.py
self.cache = TTLCache(max_size=cache_max_size, ttl_seconds=cache_ttl)

# Cached operations:
- get_track() - Spotify track metadata
- get_album() - Spotify album metadata
- get_playlist() - Spotify playlist metadata
- get_artist_albums() - Artist album list
```



### Uncached Operations (Performance Impact)

1. **Audio Search Results** (`core/audio_provider.py`)

- Searches YouTube Music for audio sources
- Network I/O, can be slow
- Same query may be repeated (e.g., re-downloading playlists)

2. **File Existence Checks** (`core/downloader.py`)

- Checks if file already exists before download
- File system I/O, repeated for same files

3. **Metadata Embedding Results**

- May re-embed metadata for existing files
- File I/O operations

4. **Configuration Parsing**

- YAML parsing (minimal impact, but can be cached)

5. **Album/Playlist Track Lists**

- Spotify API pagination results
- Currently cached at track level, but not at collection level

## Objectives

1. **Primary**: Cache audio search results to avoid redundant searches
2. **Primary**: Cache file existence checks to reduce filesystem I/O
3. **Secondary**: Implement persistent cache (optional, for cross-run persistence)
4. **Tertiary**: Add cache statistics and monitoring
5. **Tertiary**: Optimize cache hit rates through better key strategies

## Technical Approach

### Phase 1: Enhance Current Cache Implementation

#### Step 1.1: Add Thread Safety

Current `TTLCache` uses `OrderedDict` which is not thread-safe. Add locking:

```python
import threading
from collections import OrderedDict
import time
from typing import Any, Optional

class TTLCache:
    """Thread-safe cache with TTL expiration and LRU eviction."""
    
    def __init__(self, max_size: int = 1000, ttl_seconds: int = 3600):
        self.max_size = max_size
        self.ttl_seconds = ttl_seconds
        self._cache: OrderedDict[str, tuple[float, Any]] = OrderedDict()
        self._lock = threading.RLock()  # Reentrant lock for nested calls
    
    def get(self, key: str) -> Optional[Any]:
        """Get value from cache if not expired (thread-safe)."""
        with self._lock:
            if key not in self._cache:
                return None
            
            timestamp, value = self._cache[key]
            
            # Check if expired
            if time.time() - timestamp > self.ttl_seconds:
                del self._cache[key]
                return None
            
            # Move to end (most recently used)
            self._cache.move_to_end(key)
            return value
    
    def set(self, key: str, value: Any) -> None:
        """Set value in cache with current timestamp (thread-safe)."""
        with self._lock:
            # Remove if exists
            if key in self._cache:
                del self._cache[key]
            
            # Evict oldest if at capacity
            if len(self._cache) >= self.max_size:
                self._cache.popitem(last=False)
            
            # Add new entry
            self._cache[key] = (time.time(), value)
    
    def clear(self) -> None:
        """Clear all cache entries (thread-safe)."""
        with self._lock:
            self._cache.clear()
    
    def stats(self) -> dict:
        """Get cache statistics."""
        with self._lock:
            return {
                "size": len(self._cache),
                "max_size": self.max_size,
                "ttl_seconds": self.ttl_seconds,
            }
```



#### Step 1.2: Add Cache Statistics

Track cache hits/misses for monitoring:

```python
class TTLCache:
    def __init__(self, max_size: int = 1000, ttl_seconds: int = 3600):
        # ... existing code ...
        self._hits = 0
        self._misses = 0
    
    def get(self, key: str) -> Optional[Any]:
        with self._lock:
            if key not in self._cache:
                self._misses += 1
                return None
            
            timestamp, value = self._cache[key]
            
            if time.time() - timestamp > self.ttl_seconds:
                del self._cache[key]
                self._misses += 1
                return None
            
            self._cache.move_to_end(key)
            self._hits += 1
            return value
    
    def stats(self) -> dict:
        """Get cache statistics including hit rate."""
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
```



### Phase 2: Cache Audio Search Results

#### Step 2.1: Add Cache to AudioProvider

Modify `core/audio_provider.py` to cache search results:

```python
from core.cache import TTLCache

class AudioProvider:
    def __init__(
        self,
        output_format: str = "mp3",
        bitrate: str = "128k",
        audio_providers: List[str] = None,
        cache_max_size: int = 500,
        cache_ttl: int = 86400,  # 24 hours for search results
    ):
        # ... existing initialization ...
        self.search_cache = TTLCache(
            max_size=cache_max_size,
            ttl_seconds=cache_ttl
        )
    
    def search(self, query: str) -> Optional[str]:
        """Search for audio source (cached)."""
        # Create cache key from query
        cache_key = f"audio_search:{query.lower().strip()}"
        
        # Check cache
        cached_result = self.search_cache.get(cache_key)
        if cached_result is not None:
            logger.debug(f"Cache hit for audio search: {query}")
            return cached_result
        
        # Perform search
        logger.debug(f"Cache miss for audio search: {query}")
        audio_url = self._perform_search(query)
        
        # Cache result (even if None, to avoid repeated failed searches)
        if audio_url:
            self.search_cache.set(cache_key, audio_url)
        else:
            # Cache negative results with shorter TTL
            self.search_cache.set(cache_key, None)
        
        return audio_url
```



#### Step 2.2: Cache Key Strategy

Use normalized query strings as keys:

- Lowercase
- Strip whitespace
- Normalize special characters
- Consider including audio provider in key if multiple providers used

### Phase 3: Cache File Existence Checks

#### Step 3.1: Add File Existence Cache

Cache file existence checks to avoid repeated filesystem I/O:

```python
class Downloader:
    def __init__(self, config):
        # ... existing initialization ...
        self.file_existence_cache = TTLCache(
            max_size=10000,  # Larger cache for file paths
            ttl_seconds=3600  # 1 hour TTL
        )
    
    def _file_exists_cached(self, file_path: Path) -> bool:
        """Check if file exists (cached)."""
        cache_key = f"file_exists:{file_path.absolute()}"
        
        cached_result = self.file_existence_cache.get(cache_key)
        if cached_result is not None:
            return cached_result
        
        # Check filesystem
        exists = file_path.exists()
        self.file_existence_cache.set(cache_key, exists)
        
        return exists
    
    def download_track(self, track_url: str) -> Tuple[bool, Optional[Path]]:
        # ... existing code ...
        
        # Use cached file existence check
        output_path = self._get_output_path(song)
        if self._file_exists_cached(output_path) and self.config.overwrite == "skip":
            logger.info(f"Skipping (already exists): {output_path}")
            return True, output_path
        
        # ... rest of download logic ...
```



#### Step 3.2: Invalidate Cache on File Creation

When a file is created, invalidate the cache entry:

```python
def download_track(self, track_url: str) -> Tuple[bool, Optional[Path]]:
    # ... download logic ...
    
    # After successful download
    if downloaded_path:
        # Invalidate cache entry (file now exists)
        cache_key = f"file_exists:{downloaded_path.absolute()}"
        self.file_existence_cache.set(cache_key, True)
    
    return True, downloaded_path
```



### Phase 4: Persistent Cache (Optional)

#### Step 4.1: Implement Persistent Cache Backend

Add optional persistent cache using JSON file or SQLite:

```python
import json
import sqlite3
from pathlib import Path
from typing import Optional

class PersistentCache:
    """Persistent cache using SQLite."""
    
    def __init__(self, cache_file: Path, ttl_seconds: int = 3600):
        self.cache_file = cache_file
        self.ttl_seconds = ttl_seconds
        self._init_db()
    
    def _init_db(self):
        """Initialize SQLite database."""
        self.cache_file.parent.mkdir(parents=True, exist_ok=True)
        conn = sqlite3.connect(self.cache_file)
        conn.execute("""
            CREATE TABLE IF NOT EXISTS cache (
                key TEXT PRIMARY KEY,
                value TEXT NOT NULL,
                timestamp REAL NOT NULL
            )
        """)
        conn.execute("CREATE INDEX IF NOT EXISTS idx_timestamp ON cache(timestamp)")
        conn.commit()
        conn.close()
    
    def get(self, key: str) -> Optional[Any]:
        """Get value from persistent cache."""
        conn = sqlite3.connect(self.cache_file)
        cursor = conn.execute(
            "SELECT value, timestamp FROM cache WHERE key = ?",
            (key,)
        )
        row = cursor.fetchone()
        conn.close()
        
        if row is None:
            return None
        
        value_str, timestamp = row
        current_time = time.time()
        
        # Check if expired
        if current_time - timestamp > self.ttl_seconds:
            self.delete(key)
            return None
        
        # Deserialize value
        try:
            return json.loads(value_str)
        except json.JSONDecodeError:
            return None
    
    def set(self, key: str, value: Any) -> None:
        """Set value in persistent cache."""
        conn = sqlite3.connect(self.cache_file)
        value_str = json.dumps(value)
        timestamp = time.time()
        
        conn.execute(
            "INSERT OR REPLACE INTO cache (key, value, timestamp) VALUES (?, ?, ?)",
            (key, value_str, timestamp)
        )
        conn.commit()
        conn.close()
    
    def delete(self, key: str) -> None:
        """Delete key from cache."""
        conn = sqlite3.connect(self.cache_file)
        conn.execute("DELETE FROM cache WHERE key = ?", (key,))
        conn.commit()
        conn.close()
    
    def cleanup_expired(self) -> None:
        """Remove expired entries."""
        conn = sqlite3.connect(self.cache_file)
        current_time = time.time()
        conn.execute(
            "DELETE FROM cache WHERE ? - timestamp > ?",
            (current_time, self.ttl_seconds)
        )
        conn.commit()
        conn.close()
```



#### Step 4.2: Add Configuration Option

Add persistent cache configuration:

```python
# In core/config.py
class DownloadSettings(BaseModel):
    # ... existing fields ...
    cache_persistent: bool = False
    cache_file: Optional[str] = None  # Path to persistent cache file
```



### Phase 5: Cache Configuration and Monitoring

#### Step 5.1: Add Cache Configuration

Extend configuration to support cache settings:

```python
class DownloadSettings(BaseModel):
    # ... existing fields ...
    
    # Cache settings
    cache_max_size: int = 1000
    cache_ttl: int = 3600
    
    # Audio search cache
    audio_search_cache_max_size: int = 500
    audio_search_cache_ttl: int = 86400  # 24 hours
    
    # File existence cache
    file_cache_max_size: int = 10000
    file_cache_ttl: int = 3600
    
    # Persistent cache
    cache_persistent: bool = False
    cache_file: Optional[str] = None
```



#### Step 5.2: Add Cache Statistics Logging

Log cache statistics at end of run:

```python
def print_cache_stats(downloader: Downloader) -> None:
    """Print cache statistics."""
    logger.info("Cache Statistics:")
    logger.info(f"  Spotify API Cache: {downloader.spotify.cache.stats()}")
    logger.info(f"  Audio Search Cache: {downloader.audio.search_cache.stats()}")
    logger.info(f"  File Existence Cache: {downloader.file_existence_cache.stats()}")
```



## Implementation Details

### Cache Key Strategies

1. **Spotify API**: `{resource_type}:{id}` (e.g., `track:4iV5W9uYEdYUVa79Axb7Rh`)
2. **Audio Search**: `audio_search:{normalized_query}` (e.g., `audio_search:artist - title`)
3. **File Existence**: `file_exists:{absolute_path}` (e.g., `file_exists:/download/Artist/Album/01 - Title.mp3`)

### Cache Invalidation Strategies

1. **TTL-based**: Automatic expiration after TTL
2. **Manual**: Invalidate on file creation/deletion
3. **Size-based**: LRU eviction when cache is full

### Thread Safety

- All cache operations must be thread-safe for parallel execution
- Use `threading.RLock()` for reentrant locking
- Consider using `queue.Queue` for thread-safe operations if needed

## Testing Strategy

### Unit Tests

1. Test cache get/set operations
2. Test TTL expiration
3. Test LRU eviction
4. Test thread safety
5. Test cache statistics

### Integration Tests

1. Test cached audio searches
2. Test cached file existence checks
3. Test cache persistence (if implemented)
4. Test cache invalidation

### Performance Tests

1. Benchmark cache hit rates
2. Measure performance improvement with caching
3. Test cache under load (parallel operations)

## Risk Assessment

### Low Risk

- In-memory cache implementation (standard pattern)
- TTL-based expiration (well-understood)

### Medium Risk

- Thread safety issues (mitigated by proper locking)
- Cache invalidation bugs (mitigated by testing)
- Memory usage (mitigated by size limits)

### High Risk

- Persistent cache corruption (mitigated by validation)
- Cache key collisions (mitigated by unique key strategies)

### Mitigation Strategies

1. **Comprehensive Testing**: Test all cache operations
2. **Monitoring**: Log cache statistics
3. **Fallback**: Graceful degradation if cache fails
4. **Validation**: Validate cache data on read

## Success Criteria

1. ✅ Audio search results cached
2. ✅ File existence checks cached
3. ✅ Cache statistics available
4. ✅ Thread-safe cache operations
5. ✅ 30-50% reduction in redundant operations
6. ✅ All existing tests pass
7. ✅ No memory leaks

## Rollback Plan

If issues are discovered:

1. Disable caching via configuration
2. Investigate root cause
3. Create fix branch
4. Re-test and re-deploy

## Timeline

- **Day 1**: Enhance cache implementation (thread safety, statistics)
- **Day 2**: Implement audio search caching
- **Day 3**: Implement file existence caching
- **Day 4**: Add persistent cache (optional), configuration, monitoring
- **Day 5**: Testing, documentation, final validation

## Dependencies

- `threading` (standard library)
- `sqlite3` (standard library, for persistent cache)
- `json` (standard library, for serialization)

## Related Files

- `core/cache.py` - Cache implementation (needs updates)
- `core/spotify_client.py` - Already uses cache (may need updates)
- `core/audio_provider.py` - Needs cache integration
- `core/downloader.py` - Needs file existence cache
- `core/config.py` - Needs cache configuration options
- `download.py` - May need cache statistics logging

## Notes for Junior Engineers

### Why Caching?

- **Performance**: Avoid redundant operations
- **Cost**: Reduce API calls and network I/O
- **User Experience**: Faster execution

### Cache Types

1. **In-Memory**: Fast, but lost on restart
2. **Persistent**: Survives restarts, but slower

### Cache Key Design

- **Unique**: Each key should map to one value
- **Normalized**: Consistent formatting
- **Descriptive**: Include context (e.g., `audio_search:` prefix)

### Common Pitfalls

1. **Stale Data**: Ensure TTL is appropriate
2. **Memory Usage**: Set reasonable size limits
3. **Thread Safety**: Use locks for concurrent access
4. **Key Collisions**: Use unique, descriptive keys

### Debugging Tips

- Log cache hits/misses
- Monitor cache statistics
- Test with cache disabled to compare performance
- Check cache size and memory usage

## Notes for Technical Management

### Business Impact

- **Performance**: 30-50% reduction in redundant operations
- **Cost**: Reduced API calls and network usage
- **Scalability**: Better handling of repeated operations