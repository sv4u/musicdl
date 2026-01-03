"""
Spotify API client wrapper with caching.
"""

import logging
import random
import re
import threading
import time
from collections import deque
from functools import wraps
from typing import Any, Callable, Dict, List, Optional, TypeVar

from spotipy import Spotify
from spotipy.exceptions import SpotifyException
from spotipy.oauth2 import SpotifyClientCredentials

from core.cache import TTLCache
from core.exceptions import SpotifyError, SpotifyRateLimitError

logger = logging.getLogger(__name__)

T = TypeVar("T")


def retry_with_backoff(
    max_retries: int = 3,
    base_delay: float = 1.0,
    max_delay: float = 120.0,
    exponential_base: float = 2.0,
    jitter: bool = True,
):
    """
    Decorator for retrying functions with exponential backoff.

    Args:
        max_retries: Maximum number of retry attempts
        base_delay: Base delay in seconds
        max_delay: Maximum delay in seconds
        exponential_base: Base for exponential backoff
        jitter: Whether to add random jitter to delays

    Returns:
        Decorated function with retry logic
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


def extract_id_from_url(url: str) -> str:
    """Extract Spotify ID from URL."""
    # Pattern: https://open.spotify.com/{type}/{id}
    match = re.search(r"spotify\.com/(\w+)/([a-zA-Z0-9]+)", url)
    if match:
        return match.group(2)
    # If already an ID, return as-is
    return url


class RateLimiter:
    """
    Rate limiter using sliding window algorithm.

    Limits requests to stay within Spotify's rate limits (10 requests/second).
    """

    def __init__(
        self,
        max_requests: int = 10,
        window_seconds: float = 1.0,
        burst_capacity: int = 100,
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
        # Use condition variable for proper thread-safe waiting
        self.condition = threading.Condition(self.lock)

    def acquire(self, timeout: Optional[float] = None) -> bool:
        """
        Acquire permission to make a request.

        Args:
            timeout: Maximum time to wait (None = wait indefinitely)

        Returns:
            True if permission acquired, False if timeout
        """
        start_time = time.time()

        with self.condition:
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

                # Wait with condition variable (releases lock, re-acquires on wake)
                # This ensures thread-safe waiting without holding the lock
                self.condition.wait(min(wait_time, 0.1))

    def wait_if_needed(self) -> None:
        """Wait if necessary to respect rate limits."""
        self.acquire()


class SpotifyClient:
    """Simplified Spotify API client wrapper with caching."""

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
        """
        Initialize with credentials and cache settings.

        Args:
            client_id: Spotify API client ID
            client_secret: Spotify API client secret
            cache_max_size: Maximum cache entries (LRU eviction)
            cache_ttl: Cache TTL in seconds (default: 1 hour)
            max_retries: Maximum number of retry attempts for rate-limited requests
            retry_base_delay: Base delay in seconds for exponential backoff
            retry_max_delay: Maximum delay in seconds for exponential backoff
            rate_limit_enabled: Whether to enable proactive rate limiting
            rate_limit_requests: Maximum requests per window
            rate_limit_window: Window size in seconds
        """
        credentials = SpotifyClientCredentials(
            client_id=client_id, client_secret=client_secret
        )
        self.client = Spotify(auth_manager=credentials)
        self.cache = TTLCache(max_size=cache_max_size, ttl_seconds=cache_ttl)
        self.max_retries = max_retries
        self.retry_base_delay = retry_base_delay
        self.retry_max_delay = retry_max_delay

        # Rate limiting
        self.rate_limit_enabled = rate_limit_enabled
        if rate_limit_enabled:
            # Set burst_capacity to match max_requests to prevent exceeding Spotify limits
            self.rate_limiter = RateLimiter(
                max_requests=rate_limit_requests,
                window_seconds=rate_limit_window,
                burst_capacity=rate_limit_requests,  # No burst beyond normal limit
            )
        else:
            self.rate_limiter = None

    def _is_rate_limit_error(self, exception: Exception) -> bool:
        """
        Check if exception is a rate limit error (HTTP 429).

        Args:
            exception: Exception to check

        Returns:
            True if exception is a rate limit error, False otherwise
        """
        if isinstance(exception, SpotifyException):
            return exception.http_status == 429
        return False

    def _extract_retry_after(self, exception: Exception) -> Optional[int]:
        """
        Extract Retry-After value from exception headers.

        Args:
            exception: SpotifyException with headers

        Returns:
            Retry-After value in seconds, or None if not available
        """
        if isinstance(exception, SpotifyException):
            if hasattr(exception, "headers") and exception.headers:
                retry_after = exception.headers.get("Retry-After")
                if retry_after:
                    try:
                        return int(retry_after)
                    except (ValueError, TypeError):
                        pass
        return None

    def _get_cached_or_fetch(self, cache_key: str, fetch_func: Callable[[], Any]) -> Any:
        """
        Get from cache or fetch and cache the result with rate limit handling.

        Args:
            cache_key: Cache key for the request
            fetch_func: Function to fetch data from Spotify API

        Returns:
            Cached or fetched result

        Raises:
            SpotifyRateLimitError: If rate limited and max retries exceeded
            SpotifyError: For other Spotify API errors
        """
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
            max_delay=self.retry_max_delay,
        )
        def fetch_with_retry() -> Any:
            try:
                return fetch_func()
            except Exception as e:
                # Convert to rate limit error if applicable
                if self._is_rate_limit_error(e):
                    retry_after = self._extract_retry_after(e)
                    raise SpotifyRateLimitError(
                        f"Spotify API rate limited: {e}",
                        retry_after=retry_after,
                    ) from e
                raise SpotifyError(f"Spotify API error: {e}") from e

        result = fetch_with_retry()

        # Cache the result
        self.cache.set(cache_key, result)

        return result

    def get_track(self, track_id_or_url: str) -> Dict[str, Any]:
        """Get track metadata (cached)."""
        track_id = extract_id_from_url(track_id_or_url)
        cache_key = f"track:{track_id}"
        return self._get_cached_or_fetch(
            cache_key, lambda: self.client.track(track_id)
        )

    def get_album(self, album_id_or_url: str) -> Dict[str, Any]:
        """Get album metadata (cached)."""
        album_id = extract_id_from_url(album_id_or_url)
        cache_key = f"album:{album_id}"
        return self._get_cached_or_fetch(
            cache_key, lambda: self.client.album(album_id)
        )

    def get_playlist(self, playlist_id_or_url: str) -> Dict[str, Any]:
        """Get playlist metadata (cached)."""
        playlist_id = extract_id_from_url(playlist_id_or_url)
        cache_key = f"playlist:{playlist_id}"
        return self._get_cached_or_fetch(
            cache_key, lambda: self.client.playlist(playlist_id)
        )

    def get_artist(self, artist_id_or_url: str) -> Dict[str, Any]:
        """Get artist metadata (cached)."""
        artist_id = extract_id_from_url(artist_id_or_url)
        cache_key = f"artist:{artist_id}"
        return self._get_cached_or_fetch(
            cache_key, lambda: self.client.artist(artist_id)
        )

    def get_artist_albums(self, artist_id_or_url: str) -> List[Dict[str, Any]]:
        """
        Get all albums and singles for an artist (cached).
        
        Excludes compilations and "Appears On" albums to focus on the
        artist's discography only.
        
        Args:
            artist_id_or_url: Spotify artist URL or ID
        
        Returns:
            List of album dictionaries (albums and singles only)
        """
        artist_id = extract_id_from_url(artist_id_or_url)
        cache_key = f"artist_albums:{artist_id}"

        def fetch_albums() -> List[Dict[str, Any]]:
            """Fetch all albums for artist with pagination, filtered to discography only."""
            albums = []
            # Filter to discography only (albums and singles)
            # Excludes compilations and "appears_on" albums where artist is featured
            results = self.client.artist_albums(
                artist_id,
                limit=50,
                include_groups="album,single"
            )
            albums.extend(results.get("items", []))

            # Handle pagination with rate limiting
            while results.get("next"):
                results = self._next_with_rate_limit(results)
                albums.extend(results.get("items", []))

            return albums

        return self._get_cached_or_fetch(cache_key, fetch_albums)

    def _next_with_rate_limit(self, results: Dict[str, Any]) -> Dict[str, Any]:
        """
        Get next page of results with rate limiting.

        Args:
            results: Current page results with 'next' field

        Returns:
            Next page results

        Raises:
            SpotifyRateLimitError: If rate limited and max retries exceeded
            SpotifyError: For other Spotify API errors
        """
        # Wait for rate limit if enabled
        if self.rate_limiter:
            self.rate_limiter.wait_if_needed()

        # Fetch next page with retry logic
        @retry_with_backoff(
            max_retries=self.max_retries,
            base_delay=self.retry_base_delay,
            max_delay=self.retry_max_delay,
        )
        def fetch_next_with_retry() -> Dict[str, Any]:
            try:
                return self.client.next(results)
            except Exception as e:
                # Convert to rate limit error if applicable
                if self._is_rate_limit_error(e):
                    retry_after = self._extract_retry_after(e)
                    raise SpotifyRateLimitError(
                        f"Spotify API rate limited during pagination: {e}",
                        retry_after=retry_after,
                    ) from e
                raise SpotifyError(f"Spotify API error during pagination: {e}") from e

        return fetch_next_with_retry()

    def clear_cache(self) -> None:
        """Clear the cache (useful for testing or forced refresh)."""
        self.cache.clear()

