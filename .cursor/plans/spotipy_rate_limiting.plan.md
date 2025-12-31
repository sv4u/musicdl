# Plan: Properly Handle Spotipy Rate/Request Limits

## Executive Summary

This plan outlines the implementation of comprehensive rate limiting and request throttling for the Spotify API client in musicdl. Currently, the application does not properly handle HTTP 429 (Too Many Requests) responses from the Spotify API, which can lead to failed requests, poor user experience, and potential API access restrictions. This plan implements rate limiting, exponential backoff, retry logic with proper delay handling, and proactive request throttling to ensure reliable API usage within Spotify's limits.**Target Audience**: Technical leads, junior engineers, and technical management**Estimated Effort**: 4-5 days**Risk Level**: Medium**Priority**: High (prevents API failures and improves reliability)

## Current State Analysis

### Current Implementation

#### SpotifyClient (`core/spotify_client.py`)

- **No Rate Limiting**: No proactive throttling of requests
- **Basic Error Handling**: Catches exceptions but doesn't handle HTTP 429 specifically
- **No Retry-After Handling**: Doesn't respect Spotify's `Retry-After` header
- **No Exponential Backoff**: Simple retry logic without backoff strategy

#### Current Error Handling

```python
def _get_cached_or_fetch(self, cache_key: str, fetch_func: Callable[[], Any]) -> Any:
    try:
        result = fetch_func()
    except Exception as e:
        raise SpotifyError(f"Spotify API error: {e}") from e
```

**Issues**:

- Generic exception handling doesn't distinguish rate limit errors
- No retry logic for rate-limited requests
- No delay between retries
- No respect for `Retry-After` headers

### Spotify API Rate Limits

#### Official Rate Limits

- **Standard Rate Limit**: 10 requests per second per user
- **Burst Capacity**: ~100 requests in short bursts
- **Rate Limit Response**: HTTP 429 (Too Many Requests)
- **Retry-After Header**: Specifies seconds to wait before retrying
- **Rate Limit Window**: Rolling window (not fixed window)

#### Rate Limit Headers

Spotify API responses include:

- `Retry-After`: Number of seconds to wait (when rate limited)
- `X-RateLimit-Limit`: Maximum requests per window
- `X-RateLimit-Remaining`: Remaining requests in current window
- `X-RateLimit-Reset`: Unix timestamp when rate limit resets

### Current Problems

1. **No Rate Limit Detection**: Can't identify when rate limited
2. **No Backoff Strategy**: Immediate retries can worsen the situation
3. **No Request Throttling**: May exceed rate limits unintentionally
4. **Poor Error Messages**: Users don't know why requests fail
5. **No Recovery Strategy**: Failed requests aren't automatically retried

## Objectives

1. **Primary**: Detect and handle HTTP 429 rate limit responses
2. **Primary**: Implement exponential backoff with jitter for retries
3. **Primary**: Respect `Retry-After` headers from Spotify API
4. **Secondary**: Implement proactive rate limiting to prevent hitting limits
5. **Tertiary**: Add rate limit monitoring and logging
6. **Tertiary**: Provide configuration options for rate limiting behavior

## Technical Approach

### Phase 1: Detect Rate Limit Responses

#### Step 1.1: Create Rate Limit Exception

Create a specific exception for rate limit errors:

```python
# In core/exceptions.py
class SpotifyRateLimitError(SpotifyError):
    """Spotify API rate limit error (HTTP 429)."""
    
    def __init__(self, message: str, retry_after: Optional[int] = None):
        super().__init__(message)
        self.retry_after = retry_after  # Seconds to wait
        self.status_code = 429
```



#### Step 1.2: Detect Rate Limit in Spotipy

Spotipy raises `spotipy.exceptions.SpotifyException` for API errors. Check status code:

```python
from spotipy.exceptions import SpotifyException

def _is_rate_limit_error(self, exception: Exception) -> bool:
    """Check if exception is a rate limit error."""
    if isinstance(exception, SpotifyException):
        return exception.http_status == 429
    return False

def _extract_retry_after(self, exception: Exception) -> Optional[int]:
    """Extract Retry-After value from exception."""
    if isinstance(exception, SpotifyException):
        # Check if exception has headers
        if hasattr(exception, 'headers'):
            retry_after = exception.headers.get('Retry-After')
            if retry_after:
                try:
                    return int(retry_after)
                except (ValueError, TypeError):
                    pass
    return None
```



### Phase 2: Implement Retry Logic with Backoff

#### Step 2.1: Create Retry Decorator with Exponential Backoff

Implement retry logic with exponential backoff and jitter:

```python
import time
import random
from functools import wraps
from typing import Callable, Optional, TypeVar, Any
from core.exceptions import SpotifyRateLimitError, SpotifyError

T = TypeVar('T')

def retry_with_backoff(
    max_retries: int = 3,
    base_delay: float = 1.0,
    max_delay: float = 120.0,
    exponential_base: float = 1.5,
    jitter: bool = True
):
    """
    Decorator for retrying functions with exponential backoff.
    
    Args:
        max_retries: Maximum number of retry attempts
        base_delay: Base delay in seconds
        max_delay: Maximum delay in seconds
        exponential_base: Base for exponential backoff
        jitter: Whether to add random jitter to delays
    """
    def decorator(func: Callable[..., T]) -> Callable[..., T]:
        @wraps(func)
        def wrapper(*args, **kwargs) -> T:
            last_exception = None
            
            for attempt in range(max_retries + 1):
                try:
                    return func(*args, **kwargs)
                except SpotifyRateLimitError as e:
                    last_exception = e
                    
                    if attempt >= max_retries:
                        raise
                    
                    # Use Retry-After if provided, otherwise calculate delay
                    if e.retry_after:
                        delay = min(e.retry_after, max_delay)
                    else:
                        delay = min(
                            base_delay * (exponential_base ** attempt),
                            max_delay
                        )
                    
                    # Add jitter (random 0-25% of delay)
                    if jitter:
                        jitter_amount = delay * 0.25 * random.random()
                        delay += jitter_amount
                    
                    logger.warning(
                        f"Rate limited (attempt {attempt + 1}/{max_retries + 1}). "
                        f"Retrying in {delay:.2f}s..."
                    )
                    time.sleep(delay)
                    
                except SpotifyError as e:
                    # Don't retry other Spotify errors
                    raise
                except Exception as e:
                    last_exception = e
                    if attempt >= max_retries:
                        raise SpotifyError(f"Unexpected error: {e}") from e
                    
                    # Retry with exponential backoff for other errors
                    delay = min(
                        base_delay * (exponential_base ** attempt),
                        max_delay
                    )
                    if jitter:
                        jitter_amount = delay * 0.25 * random.random()
                        delay += jitter_amount
                    
                    logger.warning(
                        f"Error (attempt {attempt + 1}/{max_retries + 1}): {e}. "
                        f"Retrying in {delay:.2f}s..."
                    )
                    time.sleep(delay)
            
            # Should not reach here, but handle just in case
            if last_exception:
                raise last_exception
            raise SpotifyError("Max retries exceeded")
        
        return wrapper
    return decorator
```



#### Step 2.2: Update SpotifyClient Methods

Apply retry decorator to API methods:

```python
class SpotifyClient:
    def __init__(
        self,
        client_id: str,
        client_secret: str,
        cache_max_size: int = 1000,
        cache_ttl: int = 3600,
        max_retries: int = 3,
        retry_base_delay: float = 1.0,
        retry_max_delay: float = 120.0,
    ):
        # ... existing initialization ...
        self.max_retries = max_retries
        self.retry_base_delay = retry_base_delay
        self.retry_max_delay = retry_max_delay
    
    def _get_cached_or_fetch(
        self, 
        cache_key: str, 
        fetch_func: Callable[[], Any]
    ) -> Any:
        """Get from cache or fetch with rate limit handling."""
        # Try cache first
        cached = self.cache.get(cache_key)
        if cached is not None:
            return cached
        
        # Fetch with retry logic
        @retry_with_backoff(
            max_retries=self.max_retries,
            base_delay=self.retry_base_delay,
            max_delay=self.retry_max_delay
        )
        def fetch_with_retry():
            try:
                return fetch_func()
            except Exception as e:
                # Convert to rate limit error if applicable
                if self._is_rate_limit_error(e):
                    retry_after = self._extract_retry_after(e)
                    raise SpotifyRateLimitError(
                        f"Spotify API rate limited: {e}",
                        retry_after=retry_after
                    ) from e
                raise SpotifyError(f"Spotify API error: {e}") from e
        
        result = fetch_with_retry()
        
        # Cache the result
        self.cache.set(cache_key, result)
        return result
```



### Phase 3: Implement Proactive Rate Limiting

#### Step 3.1: Create Rate Limiter Class

Implement a token bucket or sliding window rate limiter:

```python
import time
import threading
from collections import deque
from typing import Optional

class RateLimiter:
    """
    Rate limiter using token bucket algorithm.
    Limits requests to stay within Spotify's rate limits.
    """
    
    def __init__(
        self,
        max_requests: int = 10,  # Requests per window
        window_seconds: float = 1.0,  # Window size in seconds
        burst_capacity: int = 100  # Burst capacity
    ):
        """
        Initialize rate limiter.
        
        Args:
            max_requests: Maximum requests per window
            window_seconds: Window size in seconds
            burst_capacity: Maximum burst capacity
        """
        self.max_requests = max_requests
        self.window_seconds = window_seconds
        self.burst_capacity = burst_capacity
        
        # Track request timestamps
        self.request_times: deque = deque()
        self.lock = threading.RLock()
    
    def acquire(self, timeout: Optional[float] = None) -> bool:
        """
        Acquire permission to make a request.
        
        Args:
            timeout: Maximum time to wait (None = wait indefinitely)
        
        Returns:
            True if permission acquired, False if timeout
        """
        start_time = time.time()
        
        with self.lock:
            while True:
                now = time.time()
                
                # Remove old requests outside the window
                while self.request_times and \
                      self.request_times[0] < now - self.window_seconds:
                    self.request_times.popleft()
                
                # Check if we can make a request
                if len(self.request_times) < self.max_requests:
                    self.request_times.append(now)
                    return True
                
                # Check burst capacity
                if len(self.request_times) < self.burst_capacity:
                    # Allow burst, but log warning
                    logger.warning(
                        f"Burst request allowed ({len(self.request_times) + 1}/{self.burst_capacity})"
                    )
                    self.request_times.append(now)
                    return True
                
                # Calculate wait time
                oldest_request = self.request_times[0]
                wait_time = self.window_seconds - (now - oldest_request)
                
                if timeout is not None:
                    elapsed = time.time() - start_time
                    if elapsed + wait_time > timeout:
                        return False
                
                # Wait before retrying
                time.sleep(min(wait_time, 0.1))  # Sleep in small increments
    
    def wait_if_needed(self) -> None:
        """Wait if necessary to respect rate limits."""
        self.acquire()
```



#### Step 3.2: Integrate Rate Limiter into SpotifyClient

Add rate limiter to SpotifyClient:

```python
class SpotifyClient:
    def __init__(
        self,
        client_id: str,
        client_secret: str,
        cache_max_size: int = 1000,
        cache_ttl: int = 3600,
        max_retries: int = 3,
        retry_base_delay: float = 1.0,
        retry_max_delay: float = 120.0,
        rate_limit_enabled: bool = True,
        rate_limit_requests: int = 10,
        rate_limit_window: float = 1.0,
    ):
        # ... existing initialization ...
        
        # Rate limiting
        self.rate_limit_enabled = rate_limit_enabled
        if rate_limit_enabled:
            self.rate_limiter = RateLimiter(
                max_requests=rate_limit_requests,
                window_seconds=rate_limit_window,
                burst_capacity=100
            )
        else:
            self.rate_limiter = None
    
    def _get_cached_or_fetch(
        self, 
        cache_key: str, 
        fetch_func: Callable[[], Any]
    ) -> Any:
        """Get from cache or fetch with rate limiting."""
        # Try cache first
        cached = self.cache.get(cache_key)
        if cached is not None:
            return cached
        
        # Wait for rate limit if enabled
        if self.rate_limiter:
            self.rate_limiter.wait_if_needed()
        
        # Fetch with retry logic
        @retry_with_backoff(
            max_retries=self.max_retries,
            base_delay=self.retry_base_delay,
            max_delay=self.retry_max_delay
        )
        def fetch_with_retry():
            try:
                return fetch_func()
            except Exception as e:
                if self._is_rate_limit_error(e):
                    retry_after = self._extract_retry_after(e)
                    raise SpotifyRateLimitError(
                        f"Spotify API rate limited: {e}",
                        retry_after=retry_after
                    ) from e
                raise SpotifyError(f"Spotify API error: {e}") from e
        
        result = fetch_with_retry()
        
        # Cache the result
        self.cache.set(cache_key, result)
        return result
```



### Phase 4: Add Rate Limit Monitoring

#### Step 4.1: Track Rate Limit Headers

Extract and log rate limit information from responses:

```python
class RateLimitMonitor:
    """Monitor rate limit status from API responses."""
    
    def __init__(self):
        self.limit = None
        self.remaining = None
        self.reset_time = None
        self.lock = threading.RLock()
    
    def update_from_response(self, response_headers: dict) -> None:
        """Update rate limit info from response headers."""
        with self.lock:
            if 'X-RateLimit-Limit' in response_headers:
                self.limit = int(response_headers['X-RateLimit-Limit'])
            if 'X-RateLimit-Remaining' in response_headers:
                self.remaining = int(response_headers['X-RateLimit-Remaining'])
            if 'X-RateLimit-Reset' in response_headers:
                self.reset_time = int(response_headers['X-RateLimit-Reset'])
    
    def get_status(self) -> dict:
        """Get current rate limit status."""
        with self.lock:
            return {
                "limit": self.limit,
                "remaining": self.remaining,
                "reset_time": self.reset_time,
                "percentage_used": (
                    ((self.limit - self.remaining) / self.limit * 100)
                    if self.limit and self.remaining is not None
                    else None
                )
            }
    
    def is_low(self, threshold: float = 0.2) -> bool:
        """Check if remaining requests are below threshold."""
        if self.limit and self.remaining is not None:
            return (self.remaining / self.limit) < threshold
        return False
```



#### Step 4.2: Integrate Monitoring

Add monitoring to SpotifyClient:

```python
class SpotifyClient:
    def __init__(self, ...):
        # ... existing initialization ...
        self.rate_limit_monitor = RateLimitMonitor()
    
    def _get_cached_or_fetch(self, ...):
        # ... existing code ...
        
        # Note: Spotipy may not expose headers directly
        # May need to extend Spotipy or use requests directly
        # For now, monitor through exception handling
```



### Phase 5: Configuration and Logging

#### Step 5.1: Add Configuration Options

Extend configuration to support rate limiting:

```python
# In core/config.py
class DownloadSettings(BaseModel):
    # ... existing fields ...
    
    # Rate limiting settings
    spotify_max_retries: int = 3
    spotify_retry_base_delay: float = 1.0
    spotify_retry_max_delay: float = 120.0
    spotify_rate_limit_enabled: bool = True
    spotify_rate_limit_requests: int = 10
    spotify_rate_limit_window: float = 1.0
```



#### Step 5.2: Update SpotifyClient Initialization

Use configuration values:

```python
# In core/downloader.py
class Downloader:
    def __init__(self, config):
        # ... existing initialization ...
        self.spotify = SpotifyClient(
            config.client_id,
            config.client_secret,
            cache_max_size=config.cache_max_size,
            cache_ttl=config.cache_ttl,
            max_retries=config.spotify_max_retries,
            retry_base_delay=config.spotify_retry_base_delay,
            retry_max_delay=config.spotify_retry_max_delay,
            rate_limit_enabled=config.spotify_rate_limit_enabled,
            rate_limit_requests=config.spotify_rate_limit_requests,
            rate_limit_window=config.spotify_rate_limit_window,
        )
```



#### Step 5.3: Add Logging

Log rate limit events:

```python
# In SpotifyClient methods
if self._is_rate_limit_error(e):
    retry_after = self._extract_retry_after(e)
    logger.warning(
        f"Spotify API rate limited. "
        f"Retry after {retry_after}s if provided, "
        f"otherwise using exponential backoff."
    )
    raise SpotifyRateLimitError(...)
```



## Implementation Details

### Error Detection Strategy

1. **Check Exception Type**: `isinstance(exception, SpotifyException)`
2. **Check Status Code**: `exception.http_status == 429`
3. **Extract Headers**: Get `Retry-After` from exception headers
4. **Convert to Custom Exception**: Raise `SpotifyRateLimitError`

### Backoff Strategy

1. **Exponential Backoff**: `delay = base_delay * (2 ^ attempt)`
2. **Jitter**: Add random 0-25% to prevent thundering herd
3. **Max Delay**: Cap at 60 seconds
4. **Retry-After Priority**: Use `Retry-After` header if available

### Rate Limiting Strategy

1. **Token Bucket**: Track request timestamps in sliding window
2. **Burst Handling**: Allow bursts up to capacity
3. **Thread Safety**: Use locks for concurrent access
4. **Non-Blocking**: Wait with small sleep increments

## Testing Strategy

### Unit Tests

1. Test rate limit error detection
2. Test retry logic with backoff
3. Test rate limiter token bucket
4. Test Retry-After header extraction
5. Test exponential backoff calculation

### Integration Tests

1. Test with mock rate limit responses
2. Test retry behavior
3. Test rate limiter under load
4. Test concurrent requests

### Manual Testing

1. Test with actual Spotify API (careful not to abuse)
2. Verify retry behavior on rate limit
3. Verify rate limiter prevents hitting limits
4. Test with various configurations

## Risk Assessment

### Low Risk

- Retry logic (standard pattern)
- Exponential backoff (well-understood)

### Medium Risk

- Rate limiter accuracy (may need tuning)
- Thread safety (mitigated by proper locking)
- Spotipy header access (may need workarounds)

### High Risk

- Breaking existing functionality (mitigated by testing)
- Incorrect rate limit handling (could cause more issues)

### Mitigation Strategies

1. **Comprehensive Testing**: Test all rate limit scenarios
2. **Gradual Rollout**: Enable rate limiting gradually
3. **Monitoring**: Log all rate limit events
4. **Fallback**: Disable rate limiting if issues occur

## Success Criteria

1. ✅ HTTP 429 errors detected and handled
2. ✅ Exponential backoff with jitter implemented
3. ✅ Retry-After headers respected
4. ✅ Proactive rate limiting prevents hitting limits
5. ✅ All existing tests pass
6. ✅ No increase in API failures
7. ✅ Configuration options available

## Rollback Plan

If issues are discovered:

1. Disable rate limiting via configuration
2. Revert to basic error handling
3. Investigate root cause
4. Create fix branch
5. Re-test and re-deploy

## Timeline

- **Day 1**: Implement rate limit detection and exceptions
- **Day 2**: Implement retry logic with exponential backoff
- **Day 3**: Implement proactive rate limiting
- **Day 4**: Add monitoring, configuration, and logging
- **Day 5**: Testing, documentation, final validation

## Dependencies

- `spotipy` library (already in use)
- `threading` (standard library)
- `time` (standard library)
- `random` (standard library, for jitter)

## Related Files

- `core/spotify_client.py` - Major updates needed
- `core/exceptions.py` - Add `SpotifyRateLimitError`
- `core/config.py` - Add rate limiting configuration
- `core/downloader.py` - Update SpotifyClient initialization
- `tests/unit/test_spotify_client.py` - Add rate limit tests

## Notes for Junior Engineers

### Why Rate Limiting?

- **API Reliability**: Prevents hitting limits and getting blocked
- **User Experience**: Automatic retries with backoff
- **Cost**: Avoids wasted requests
- **Compliance**: Respects API terms of service

### Rate Limit Concepts

1. **HTTP 429**: Too Many Requests status code
2. **Retry-After**: Header specifying wait time
3. **Exponential Backoff**: Increasing delays between retries
4. **Jitter**: Random variation to prevent synchronized retries
5. **Token Bucket**: Algorithm for rate limiting

### Common Pitfalls

1. **Immediate Retries**: Can worsen rate limiting
2. **No Jitter**: Can cause thundering herd problem
3. **Ignoring Retry-After**: Should respect API's guidance
4. **Too Aggressive**: Rate limiter should be conservative

### Debugging Tips

- Log all rate limit events
- Monitor retry counts and delays
- Check rate limit headers in responses
- Test with rate limiting disabled to compare

## Notes for Technical Management

### Business Impact

- **Reliability**: Prevents API failures and improves uptime
- **User Experience**: Automatic recovery from rate limits
- **Compliance**: Respects API terms and prevents blocking
- **Cost**: Reduces wasted API calls

### Resource Requirements

- **Development Time**: 4-5 days