"""
Custom logging handler for intercepting spotipy rate limit warnings.

This handler detects spotipy.util WARNING messages about rate limits and
updates plan metadata accordingly.
"""

import logging
import re
import threading
import time
from pathlib import Path
from typing import Callable, Optional

logger = logging.getLogger(__name__)


class SpotipyRateLimitHandler(logging.Handler):
    """
    Custom logging handler that intercepts spotipy rate limit warnings.
    
    When a spotipy rate limit warning is detected, this handler extracts
    the retry_after value and calls a callback to update plan metadata.
    """

    # Pattern to match spotipy rate limit warnings
    # Example: "Your application has reached a rate/request limit. Retry will occur after: 27212 s"
    RATE_LIMIT_PATTERN = re.compile(
        r"Your application has reached a rate/request limit\.\s*Retry will occur after:\s*(\d+)\s*s",
        re.IGNORECASE
    )

    def __init__(self, callback: Optional[Callable[[int], None]] = None):
        """
        Initialize the handler.
        
        Args:
            callback: Optional callback function that takes retry_after_seconds (int)
                     and updates plan metadata. If None, warnings are just logged.
        """
        super().__init__()
        self._callback = callback
        self._callback_lock = threading.Lock()
        self.setLevel(logging.WARNING)  # Only handle WARNING and above

    @property
    def callback(self) -> Optional[Callable[[int], None]]:
        """
        Get the current callback function (thread-safe).
        
        Returns:
            Current callback function or None
        """
        with self._callback_lock:
            return self._callback

    @callback.setter
    def callback(self, value: Optional[Callable[[int], None]]) -> None:
        """
        Set the callback function (thread-safe).
        
        Args:
            value: New callback function or None
        """
        with self._callback_lock:
            self._callback = value

    def emit(self, record: logging.LogRecord) -> None:
        """
        Process a log record.
        
        Args:
            record: Log record to process
        """
        # Only process spotipy.util logger warnings
        if record.name != "spotipy.util" or record.levelno < logging.WARNING:
            return

        # Check if message contains rate limit warning
        message = record.getMessage()
        match = self.RATE_LIMIT_PATTERN.search(message)
        
        if match:
            try:
                retry_after_seconds = int(match.group(1))
                logger.warning(
                    f"Detected spotipy rate limit warning: retry after {retry_after_seconds}s"
                )
                
                # Get callback safely (thread-safe)
                cb = self.callback
                if cb:
                    try:
                        cb(retry_after_seconds)
                    except Exception as e:
                        logger.error(f"Error in rate limit callback: {e}", exc_info=True)
            except (ValueError, AttributeError) as e:
                logger.debug(f"Failed to parse retry_after from message: {message}: {e}")


def create_rate_limit_callback(
    plan_getter: Callable[[], Optional[object]],
    spotify_client: Optional[object],
    plan_path_getter: Callable[[], Path],
) -> Callable[[int], None]:
    """
    Create a callback function that updates plan metadata with rate limit info.
    
    Uses getter functions to avoid stale references when plan or path changes.
    
    Args:
        plan_getter: Function that returns the current DownloadPlan instance (or the plan itself)
        spotify_client: SpotifyClient instance (for consistency with existing rate limit tracking)
        plan_path_getter: Function that returns the current plan path (or the path itself)
        
    Returns:
        Callback function that takes retry_after_seconds and updates plan metadata
    """
    def update_rate_limit_info(retry_after_seconds: int) -> None:
        """
        Update plan metadata with rate limit information.
        
        Args:
            retry_after_seconds: Number of seconds to wait before retrying
        """
        # Get current plan and path using getters
        plan = plan_getter() if callable(plan_getter) else plan_getter
        plan_path = plan_path_getter() if callable(plan_path_getter) else plan_path_getter
        
        if plan is None:
            return
        
        current_time = time.time()
        retry_after_timestamp = current_time + retry_after_seconds
        
        # Update plan metadata
        plan.metadata["rate_limit"] = {
            "active": True,
            "retry_after_seconds": retry_after_seconds,
            "retry_after_timestamp": retry_after_timestamp,
            "detected_at": current_time,
        }
        
        # Also update spotify_client for consistency
        if spotify_client:
            spotify_client._update_rate_limit_info(retry_after_seconds)
        
        # Save plan to make it immediately visible on status page
        try:
            plan.save(plan_path)
            logger.info(
                f"Updated plan metadata with rate limit: retry after {retry_after_seconds}s "
                f"(expires at {time.strftime('%Y-%m-%d %H:%M:%S', time.localtime(retry_after_timestamp))})"
            )
        except Exception as e:
            logger.error(f"Failed to save plan with rate limit info: {e}", exc_info=True)
    
    return update_rate_limit_info
