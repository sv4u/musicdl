"""
Rate limiting utilities for managing network impact.

Provides request rate limiting and bandwidth limiting capabilities
for controlling download behavior.
"""

import logging
import threading
import time
from collections import deque
from contextlib import contextmanager
from functools import wraps
from typing import Callable, Optional, TypeVar

logger = logging.getLogger(__name__)

T = TypeVar("T")


class RequestRateLimiter:
    """
    Request rate limiter using sliding window algorithm.
    
    Limits the number of requests per time window using a thread-safe
    sliding window approach.
    """

    def __init__(
        self,
        max_requests: int = 2,
        window_seconds: float = 1.0,
        enabled: bool = True,
    ):
        """
        Initialize request rate limiter.

        Args:
            max_requests: Maximum requests per window
            window_seconds: Window size in seconds
            enabled: Whether rate limiting is enabled
        """
        self.max_requests = max_requests
        self.window_seconds = window_seconds
        self.enabled = enabled

        # Track request timestamps using deque for efficient sliding window
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
        if not self.enabled:
            return True

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

                # Calculate wait time until oldest request expires
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
        """
        Wait if necessary to respect rate limits.
        
        This is a convenience method that calls acquire() and waits indefinitely.
        """
        self.acquire()

    @contextmanager
    def request(self):
        """
        Context manager for rate-limited requests.

        Usage:
            with rate_limiter.request():
                # Make network request here
                pass
        """
        self.wait_if_needed()
        yield


class BandwidthLimiter:
    """
    Bandwidth limiter for download streams.
    
    Tracks global bytes transferred per second and throttles individual
    download streams to respect the global bandwidth limit.
    """

    def __init__(
        self,
        bytes_per_second: Optional[int] = None,
        enabled: bool = True,
    ):
        """
        Initialize bandwidth limiter.

        Args:
            bytes_per_second: Maximum bytes per second (None = unlimited)
            enabled: Whether bandwidth limiting is enabled
        """
        self.bytes_per_second = bytes_per_second
        self.enabled = enabled and bytes_per_second is not None

        # Track bytes transferred in current second
        self.bytes_this_second = 0
        self.current_second = int(time.time())
        self.lock = threading.RLock()
        # Use condition variable for proper thread-safe waiting
        self.condition = threading.Condition(self.lock)

    def acquire(self, bytes_requested: int, timeout: Optional[float] = None) -> bool:
        """
        Acquire permission to transfer bytes.

        Args:
            bytes_requested: Number of bytes to transfer
            timeout: Maximum time to wait (None = wait indefinitely)

        Returns:
            True if permission acquired, False if timeout
        """
        if not self.enabled or self.bytes_per_second is None:
            return True

        start_time = time.time()

        with self.condition:
            while True:
                now = time.time()
                current_second = int(now)

                # Reset counter if we've moved to a new second
                if current_second != self.current_second:
                    self.bytes_this_second = 0
                    self.current_second = current_second

                # Check if we can transfer the requested bytes
                if self.bytes_this_second + bytes_requested <= self.bytes_per_second:
                    self.bytes_this_second += bytes_requested
                    return True

                # Calculate wait time until next second
                wait_time = 1.0 - (now - current_second)

                if timeout is not None:
                    elapsed = time.time() - start_time
                    if elapsed + wait_time > timeout:
                        return False

                # Wait until next second
                self.condition.wait(min(wait_time, 0.1))

    def transfer(self, bytes_count: int) -> None:
        """
        Record a transfer of bytes and throttle if necessary.

        Args:
            bytes_count: Number of bytes transferred
        """
        if not self.enabled or self.bytes_per_second is None:
            return

        # Split into chunks if needed to respect rate limit
        remaining = bytes_count
        while remaining > 0:
            chunk_size = min(remaining, self.bytes_per_second)
            self.acquire(chunk_size)
            remaining -= chunk_size

    @contextmanager
    def transfer_context(self, bytes_count: int):
        """
        Context manager for rate-limited transfers.

        Usage:
            with bandwidth_limiter.transfer_context(1024):
                # Transfer 1024 bytes here
                pass
        """
        self.transfer(bytes_count)
        yield


class RateLimiter:
    """
    Combined rate limiter managing both request rate and bandwidth limits.
    
    Provides a unified interface for rate limiting that coordinates
    between request rate limiting and bandwidth limiting.
    """

    def __init__(
        self,
        request_rate_enabled: bool = True,
        request_rate_requests: int = 2,
        request_rate_window: float = 1.0,
        bandwidth_enabled: bool = True,
        bandwidth_limit: Optional[int] = 1048576,  # 1MB/sec default
    ):
        """
        Initialize combined rate limiter.

        Args:
            request_rate_enabled: Whether request rate limiting is enabled
            request_rate_requests: Maximum requests per window
            request_rate_window: Window size in seconds
            bandwidth_enabled: Whether bandwidth limiting is enabled
            bandwidth_limit: Maximum bytes per second (None = unlimited)
        """
        self.request_limiter = RequestRateLimiter(
            max_requests=request_rate_requests,
            window_seconds=request_rate_window,
            enabled=request_rate_enabled,
        )
        self.bandwidth_limiter = BandwidthLimiter(
            bytes_per_second=bandwidth_limit,
            enabled=bandwidth_enabled,
        )

    def wait_for_request(self) -> None:
        """Wait if necessary to respect request rate limits."""
        self.request_limiter.wait_if_needed()

    def transfer_bytes(self, bytes_count: int) -> None:
        """
        Record a transfer of bytes and throttle if necessary.

        Args:
            bytes_count: Number of bytes transferred
        """
        self.bandwidth_limiter.transfer(bytes_count)

    @contextmanager
    def request(self):
        """
        Context manager for rate-limited requests.

        Usage:
            with rate_limiter.request():
                # Make network request here
                pass
        """
        self.wait_for_request()
        yield

    @contextmanager
    def transfer_context(self, bytes_count: int):
        """
        Context manager for rate-limited transfers.

        Usage:
            with rate_limiter.transfer_context(1024):
                # Transfer 1024 bytes here
                pass
        """
        self.transfer_bytes(bytes_count)
        yield

    def limit_function(self, func: Callable[..., T]) -> Callable[..., T]:
        """
        Decorator to rate-limit a function.

        Args:
            func: Function to rate limit

        Returns:
            Decorated function with rate limiting
        """
        @wraps(func)
        def wrapper(*args, **kwargs) -> T:
            with self.request():
                return func(*args, **kwargs)
        return wrapper

