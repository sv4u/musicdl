# Plan: Parallelize Queries and Downloads

## Executive Summary

This plan outlines the implementation of parallelization for Spotify API queries and audio downloads in the musicdl application. Currently, all operations are sequential, leading to long execution times for large playlists or artist catalogs. By implementing concurrent processing, we can reduce total execution time by 60-80% depending on the workload.**Target Audience**: Technical leads, junior engineers, and technical management**Estimated Effort**: 5-7 days**Risk Level**: Medium**Priority**: High (significant performance improvement)

## Current State Analysis

### Current Implementation

#### Sequential Processing in `download.py`

```python
# Process songs sequentially
for song in config.songs:
    downloader.download_track(song.url)

# Process artists sequentially
for artist in config.artists:
    downloader.download_artist(artist.url)

# Process playlists sequentially
for playlist in config.playlists:
    downloader.download_playlist(playlist.url)
```



#### Sequential Processing in `downloader.py`

- `download_track()`: Processes one track at a time
- `download_album()`: Downloads tracks sequentially
- `download_playlist()`: Downloads tracks sequentially
- `download_artist()`: Downloads albums sequentially

### Performance Bottlenecks

1. **Spotify API Calls**: Sequential API requests for metadata
2. **Audio Search**: Sequential searches for audio sources
3. **File Downloads**: Sequential file downloads
4. **Metadata Embedding**: Sequential metadata operations

### Current Configuration

- `config.download.threads`: Currently set to 4, but not utilized for parallelization
- No concurrent execution framework in place

## Objectives

1. **Primary**: Implement parallel processing for Spotify API queries
2. **Primary**: Implement parallel processing for audio downloads
3. **Secondary**: Respect rate limits and avoid overwhelming external services
4. **Tertiary**: Maintain error handling and retry logic
5. **Tertiary**: Provide progress reporting for parallel operations

## Technical Approach

### Phase 1: Parallelize Spotify API Queries

#### Step 1.1: Analyze Current API Usage

Identify all Spotify API calls:

- `spotify.get_track()` - Called per track
- `spotify.get_album()` - Called per track (for album metadata)
- `spotify.get_playlist()` - Called per playlist
- `spotify.get_artist_albums()` - Called per artist
- `spotify.client.next()` - Called for pagination

#### Step 1.2: Implement Thread Pool for API Queries

Use `concurrent.futures.ThreadPoolExecutor` for parallel API calls:

```python
from concurrent.futures import ThreadPoolExecutor, as_completed
from typing import List, Tuple

def fetch_track_metadata_parallel(
    spotify: SpotifyClient,
    track_urls: List[str],
    max_workers: int = 4
) -> List[Tuple[str, Dict]]:
    """Fetch track metadata in parallel."""
    results = []
    
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        future_to_url = {
            executor.submit(spotify.get_track, url): url
            for url in track_urls
        }
        
        for future in as_completed(future_to_url):
            url = future_to_url[future]
            try:
                track_data = future.result()
                results.append((url, track_data))
            except Exception as e:
                logger.error(f"Failed to fetch {url}: {e}")
                results.append((url, None))
    
    return results
```



#### Step 1.3: Add Rate Limiting

Implement rate limiting to respect Spotify API limits:

- **Spotify Rate Limits**: 
- 30 requests per second per user
- Burst capacity: ~100 requests
- Use `asyncio.Semaphore` or `threading.Semaphore` to limit concurrent requests
```python
import threading
from collections import deque
import time

class RateLimiter:
    """Rate limiter for API calls."""
    
    def __init__(self, max_calls: int, period: float):
        self.max_calls = max_calls
        self.period = period
        self.calls = deque()
        self.lock = threading.Lock()
    
    def acquire(self):
        """Acquire permission to make a call."""
        with self.lock:
            now = time.time()
            # Remove old calls outside the period
            while self.calls and self.calls[0] < now - self.period:
                self.calls.popleft()
            
            # Wait if at limit
            if len(self.calls) >= self.max_calls:
                sleep_time = self.period - (now - self.calls[0])
                if sleep_time > 0:
                    time.sleep(sleep_time)
                    # Clean up again after sleep
                    while self.calls and self.calls[0] < now - self.period:
                        self.calls.popleft()
            
            self.calls.append(time.time())
```




### Phase 2: Parallelize Audio Downloads

#### Step 2.1: Parallelize Track Downloads

Modify `download_track()` to support batch parallel execution:

```python
def download_tracks_parallel(
    self,
    track_urls: List[str],
    max_workers: int = None
) -> List[Tuple[bool, Optional[Path]]]:
    """Download multiple tracks in parallel."""
    if max_workers is None:
        max_workers = self.config.threads
    
    results = []
    
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        future_to_url = {
            executor.submit(self.download_track, url): url
            for url in track_urls
        }
        
        for future in as_completed(future_to_url):
            url = future_to_url[future]
            try:
                result = future.result()
                results.append(result)
            except Exception as e:
                logger.error(f"Failed to download {url}: {e}")
                results.append((False, None))
    
    return results
```



#### Step 2.2: Parallelize Audio Search

Parallelize the audio provider search operations:

```python
def search_audio_parallel(
    self,
    search_queries: List[str],
    max_workers: int = 4
) -> List[Tuple[str, Optional[str]]]:
    """Search for audio sources in parallel."""
    results = []
    
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        future_to_query = {
            executor.submit(self.audio.search, query): query
            for query in search_queries
        }
        
        for future in as_completed(future_to_query):
            query = future_to_query[future]
            try:
                audio_url = future.result()
                results.append((query, audio_url))
            except Exception as e:
                logger.error(f"Search failed for {query}: {e}")
                results.append((query, None))
    
    return results
```



### Phase 3: Refactor Download Methods

#### Step 3.1: Update `download_album()`

Refactor to use parallel track downloads:

```python
def download_album(self, album_url: str) -> List[Tuple[bool, Optional[Path]]]:
    """Download all tracks in an album (parallelized)."""
    try:
        album_data = self.spotify.get_album(album_url)
        
        # Collect all track URLs
        track_urls = []
        tracks_obj = album_data["tracks"]
        items = tracks_obj["items"]
        
        while tracks_obj.get("next"):
            next_data = self.spotify.client.next(tracks_obj)
            items.extend(next_data["items"])
            tracks_obj = next_data
        
        for track_item in items:
            track_id = track_item["id"]
            track_url = f"https://open.spotify.com/track/{track_id}"
            track_urls.append(track_url)
        
        # Download all tracks in parallel
        return self.download_tracks_parallel(track_urls)
        
    except Exception as e:
        logger.error(f"Failed to download album {album_url}: {e}")
        return [(False, None)]
```



#### Step 3.2: Update `download_playlist()`

Similar refactoring for playlists:

```python
def download_playlist(
    self, playlist_url: str, create_m3u: bool = False
) -> List[Tuple[bool, Optional[Path]]]:
    """Download all tracks in a playlist (parallelized)."""
    try:
        playlist_data = self.spotify.get_playlist(playlist_url)
        
        # Collect all track URLs
        track_urls = []
        tracks_obj = playlist_data["tracks"]
        items = tracks_obj["items"]
        
        while tracks_obj.get("next"):
            next_data = self.spotify.client.next(tracks_obj)
            items.extend(next_data["items"])
            tracks_obj = next_data
        
        for track_item in items:
            track = track_item.get("track")
            if not track or track.get("is_local"):
                continue
            track_url = track["external_urls"]["spotify"]
            track_urls.append(track_url)
        
        # Download all tracks in parallel
        results = self.download_tracks_parallel(track_urls)
        
        # Create M3U file if requested
        if create_m3u:
            self._create_m3u(playlist_data["name"], results)
        
        return results
        
    except Exception as e:
        logger.error(f"Failed to download playlist {playlist_url}: {e}")
        return [(False, None)]
```



#### Step 3.3: Update `download_artist()`

Parallelize album downloads:

```python
def download_artist(self, artist_url: str) -> List[Tuple[bool, Optional[Path]]]:
    """Download all albums for an artist (parallelized)."""
    try:
        albums = self.spotify.get_artist_albums(artist_url)
        all_tracks = []
        
        # Download albums in parallel
        album_urls = [album["external_urls"]["spotify"] for album in albums]
        
        with ThreadPoolExecutor(max_workers=self.config.threads) as executor:
            future_to_album = {
                executor.submit(self.download_album, album_url): album_url
                for album_url in album_urls
            }
            
            for future in as_completed(future_to_album):
                album_url = future_to_album[future]
                try:
                    tracks = future.result()
                    all_tracks.extend(tracks)
                except Exception as e:
                    logger.error(f"Failed to download album {album_url}: {e}")
        
        return all_tracks
        
    except Exception as e:
        logger.error(f"Failed to download artist {artist_url}: {e}")
        return [(False, None)]
```



### Phase 4: Update Main Orchestration

#### Step 4.1: Parallelize Config Processing

Update `process_downloads()` in `download.py`:

```python
def process_downloads(config) -> Dict[str, Dict[str, int]]:
    """Orchestrate all downloads (parallelized)."""
    downloader = Downloader(config.download)
    results = {
        "songs": {"success": 0, "failed": 0},
        "artists": {"success": 0, "failed": 0},
        "playlists": {"success": 0, "failed": 0},
    }
    
    # Process songs in parallel
    if config.songs:
        logger.info(f"Processing {len(config.songs)} songs...")
        track_urls = [song.url for song in config.songs]
        track_results = downloader.download_tracks_parallel(track_urls)
        
        for success, _ in track_results:
            if success:
                results["songs"]["success"] += 1
            else:
                results["songs"]["failed"] += 1
    
    # Process artists in parallel
    if config.artists:
        logger.info(f"Processing {len(config.artists)} artists...")
        with ThreadPoolExecutor(max_workers=config.download.threads) as executor:
            future_to_artist = {
                executor.submit(downloader.download_artist, artist.url): artist
                for artist in config.artists
            }
            
            for future in as_completed(future_to_artist):
                artist = future_to_artist[future]
                try:
                    tracks = future.result()
                    success_count = sum(1 for success, _ in tracks if success)
                    failed_count = len(tracks) - success_count
                    results["artists"]["success"] += success_count
                    results["artists"]["failed"] += failed_count
                except Exception as e:
                    logger.error(f"Error downloading artist {artist.name}: {e}")
    
    # Process playlists in parallel
    if config.playlists:
        logger.info(f"Processing {len(config.playlists)} playlists...")
        with ThreadPoolExecutor(max_workers=config.download.threads) as executor:
            future_to_playlist = {
                executor.submit(downloader.download_playlist, playlist.url, True): playlist
                for playlist in config.playlists
            }
            
            for future in as_completed(future_to_playlist):
                playlist = future_to_artist[future]
                try:
                    tracks = future.result()
                    success_count = sum(1 for success, _ in tracks if success)
                    failed_count = len(tracks) - success_count
                    results["playlists"]["success"] += success_count
                    results["playlists"]["failed"] += failed_count
                except Exception as e:
                    logger.error(f"Error downloading playlist {playlist.name}: {e}")
    
    return results
```



### Phase 5: Progress Reporting

#### Step 5.1: Add Progress Tracking

Implement progress reporting for parallel operations:

```python
from tqdm import tqdm

def download_tracks_parallel(
    self,
    track_urls: List[str],
    max_workers: int = None,
    show_progress: bool = True
) -> List[Tuple[bool, Optional[Path]]]:
    """Download multiple tracks in parallel with progress bar."""
    if max_workers is None:
        max_workers = self.config.threads
    
    results = []
    
    with ThreadPoolExecutor(max_workers=max_workers) as executor:
        future_to_url = {
            executor.submit(self.download_track, url): url
            for url in track_urls
        }
        
        if show_progress:
            with tqdm(total=len(track_urls), desc="Downloading tracks") as pbar:
                for future in as_completed(future_to_url):
                    url = future_to_url[future]
                    try:
                        result = future.result()
                        results.append(result)
                    except Exception as e:
                        logger.error(f"Failed to download {url}: {e}")
                        results.append((False, None))
                    finally:
                        pbar.update(1)
        else:
            for future in as_completed(future_to_url):
                url = future_to_url[future]
                try:
                    result = future.result()
                    results.append(result)
                except Exception as e:
                    logger.error(f"Failed to download {url}: {e}")
                    results.append((False, None))
    
    return results
```



## Implementation Details

### Thread Safety Considerations

1. **Spotify Client**: Ensure thread-safe usage

- Current `SpotifyClient` uses `TTLCache` which uses `OrderedDict` (not thread-safe)
- May need to add locks or use thread-safe cache

2. **File Operations**: Ensure no race conditions

- File downloads write to different paths (based on song metadata)
- Metadata embedding is per-file, no conflicts

3. **Logging**: Ensure thread-safe logging

- Python's `logging` module is thread-safe by default

### Error Handling

Maintain existing error handling:

- Retry logic should work per-task
- Failed tasks should not block other tasks
- Aggregate errors for reporting

### Configuration

Add/update configuration options:

- `threads`: Already exists, use for max_workers
- Consider adding separate configs:
- `api_threads`: For API calls (default: 4)
- `download_threads`: For downloads (default: 4)

## Testing Strategy

### Unit Tests

1. Test parallel API calls
2. Test parallel downloads
3. Test rate limiting
4. Test error handling in parallel context

### Integration Tests

1. Test full download flow with parallelization
2. Test with various thread counts
3. Test with rate limiting enabled

### Performance Tests

1. Benchmark sequential vs parallel execution
2. Measure speedup for different workloads:

- Small playlist (10 tracks)
- Large playlist (100+ tracks)
- Multiple artists
- Mixed workload

### Load Tests

1. Test with maximum thread count
2. Verify no resource exhaustion
3. Verify rate limits are respected

## Risk Assessment

### Low Risk

- Thread pool usage (standard library, well-tested)
- Error handling (per-task, isolated)

### Medium Risk

- Rate limiting implementation (may need tuning)
- Thread safety of cache (may need fixes)
- Resource exhaustion (too many threads)

### High Risk

- Breaking existing functionality (mitigated by comprehensive testing)
- Overwhelming external services (mitigated by rate limiting)

### Mitigation Strategies

1. **Comprehensive Testing**: Full test suite before merging
2. **Gradual Rollout**: Start with lower thread counts
3. **Monitoring**: Add logging for parallel operations
4. **Fallback**: Keep sequential code path as fallback option

## Success Criteria

1. ✅ Parallelization implemented for API queries
2. ✅ Parallelization implemented for downloads
3. ✅ 60-80% performance improvement for large workloads
4. ✅ All existing tests pass
5. ✅ Rate limiting prevents API throttling
6. ✅ Error handling maintained
7. ✅ Progress reporting works

## Rollback Plan

If issues are discovered:

1. Revert to sequential processing
2. Investigate root cause
3. Create fix branch
4. Re-test and re-deploy

## Timeline

- **Day 1-2**: Implement parallel API queries, add rate limiting
- **Day 3-4**: Implement parallel downloads, refactor download methods
- **Day 5**: Update main orchestration, add progress reporting
- **Day 6**: Comprehensive testing, fix issues
- **Day 7**: Performance testing, documentation, final validation

## Dependencies

- `concurrent.futures` (standard library)
- `tqdm` (optional, for progress bars) - may need to add to requirements.txt
- Thread-safe cache implementation (may need updates)

## Related Files

- `download.py` - Main orchestration (needs updates)
- `core/downloader.py` - Downloader class (needs major refactoring)
- `core/spotify_client.py` - May need thread-safety updates
- `core/cache.py` - May need thread-safety updates
- `core/config.py` - May need new configuration options
- `requirements.txt` - May need to add `tqdm`

## Notes for Junior Engineers

### Why Parallelization?

- **Performance**: Multiple operations at once = faster completion
- **Efficiency**: Better resource utilization
- **User Experience**: Faster downloads = better UX

### Thread Pool Executor

- Manages thread lifecycle automatically
- Handles task distribution
- Provides `as_completed()` for result processing

### Common Pitfalls

1. **Too Many Threads**: Can overwhelm system or external services
2. **Shared State**: Need to ensure thread safety
3. **Error Handling**: Errors in one thread shouldn't crash others
4. **Rate Limiting**: Must respect API limits

### Debugging Tips

- Use logging to track parallel operations
- Monitor thread count and resource usage
- Test with small workloads first
- Use `threading.current_thread().name` for debugging

## Notes for Technical Management

### Business Impact

- **Performance**: 60-80% faster execution for large workloads
- **User Experience**: Significantly reduced wait times
- **Scalability**: Better handling of large playlists/artists

### Resource Requirements