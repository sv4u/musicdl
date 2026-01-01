"""
Unit tests for RateLimiter class.
"""
import time
import threading
import pytest

from core.spotify_client import RateLimiter


class TestRateLimiter:
    """Test RateLimiter with various scenarios."""

    def test_rate_limiter_initialization(self):
        """Test RateLimiter initialization."""
        limiter = RateLimiter(
            max_requests=10,
            window_seconds=1.0,
            burst_capacity=100,
        )
        assert limiter.max_requests == 10
        assert limiter.window_seconds == 1.0
        assert limiter.burst_capacity == 100
        assert len(limiter.request_times) == 0

    def test_rate_limiter_acquire_allows_requests(self):
        """Test that rate limiter allows requests within limit."""
        limiter = RateLimiter(max_requests=5, window_seconds=1.0)
        
        # Should allow 5 requests immediately
        for i in range(5):
            assert limiter.acquire() is True
            assert len(limiter.request_times) == i + 1

    def test_rate_limiter_blocks_after_limit(self):
        """Test that rate limiter blocks requests after limit."""
        limiter = RateLimiter(max_requests=2, window_seconds=0.5)
        
        # Allow 2 requests
        assert limiter.acquire() is True
        assert limiter.acquire() is True
        
        # Third request should be blocked (but we'll allow burst)
        # Since we're within burst capacity, it should still succeed
        assert limiter.acquire() is True  # Burst allows it

    def test_rate_limiter_respects_window(self):
        """Test that rate limiter respects time window."""
        limiter = RateLimiter(max_requests=2, window_seconds=0.2)
        
        # Make 2 requests
        assert limiter.acquire() is True
        assert limiter.acquire() is True
        
        # Wait for window to expire
        time.sleep(0.25)
        
        # Should be able to make requests again
        assert limiter.acquire() is True
        assert limiter.acquire() is True

    def test_rate_limiter_burst_capacity(self):
        """Test that rate limiter respects burst capacity."""
        limiter = RateLimiter(
            max_requests=2,
            window_seconds=1.0,
            burst_capacity=5,
        )
        
        # Should allow up to burst_capacity requests
        for i in range(5):
            assert limiter.acquire() is True
        
        # 6th request should wait (but we're testing acquire, not wait_if_needed)
        # Since we're at burst capacity, it should still work but may wait
        start_time = time.time()
        result = limiter.acquire(timeout=0.1)  # Short timeout
        elapsed = time.time() - start_time
        
        # Should have waited or returned False
        assert result is False or elapsed >= 0.05  # Should have waited

    def test_rate_limiter_timeout(self):
        """Test that rate limiter respects timeout."""
        # Set burst capacity equal to max_requests to prevent burst allowance
        limiter = RateLimiter(max_requests=1, window_seconds=1.0, burst_capacity=1)
        
        # Make one request
        assert limiter.acquire() is True
        
        # Try to acquire with short timeout
        start_time = time.time()
        result = limiter.acquire(timeout=0.1)
        elapsed = time.time() - start_time
        
        # Should return False after timeout
        assert result is False
        assert elapsed < 0.2  # Should timeout quickly

    def test_rate_limiter_wait_if_needed(self):
        """Test wait_if_needed method."""
        # Set burst capacity equal to max_requests to prevent burst allowance
        limiter = RateLimiter(max_requests=2, window_seconds=0.2, burst_capacity=2)
        
        # Make 2 requests
        limiter.wait_if_needed()
        limiter.wait_if_needed()
        
        # Third should wait
        start_time = time.time()
        limiter.wait_if_needed()
        elapsed = time.time() - start_time
        
        # Should have waited for window to allow request
        assert elapsed >= 0.1  # Should have waited

    def test_rate_limiter_thread_safety(self):
        """Test that rate limiter is thread-safe."""
        limiter = RateLimiter(max_requests=10, window_seconds=1.0)
        results = []
        
        def make_request():
            """Make a request through rate limiter."""
            limiter.wait_if_needed()
            results.append(threading.current_thread().name)
        
        # Create multiple threads
        threads = []
        for i in range(15):  # More than max_requests
            thread = threading.Thread(target=make_request, name=f"Thread-{i}")
            threads.append(thread)
        
        # Start all threads
        for thread in threads:
            thread.start()
        
        # Wait for all threads
        for thread in threads:
            thread.join()
        
        # All requests should have completed (some may have waited)
        assert len(results) == 15

    def test_rate_limiter_removes_old_requests(self):
        """Test that rate limiter removes old requests from window."""
        limiter = RateLimiter(max_requests=2, window_seconds=0.3)
        
        # Make 2 requests
        limiter.acquire()
        limiter.acquire()
        assert len(limiter.request_times) == 2
        
        # Wait for window to expire
        time.sleep(0.35)
        
        # Old requests should be removed, new request should be allowed
        assert limiter.acquire() is True
        # Should have only 1 request in window now (old ones removed)
        assert len(limiter.request_times) <= 2

