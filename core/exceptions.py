"""
Custom exceptions for musicdl.
"""

from typing import Optional


class MusicDLError(Exception):
    """Base exception for all musicdl errors."""


class SpotifyError(MusicDLError):
    """Spotify API errors."""


class SpotifyRateLimitError(SpotifyError):
    """Spotify API rate limit error (HTTP 429)."""

    def __init__(self, message: str, retry_after: Optional[int] = None):
        """
        Initialize rate limit error.

        Args:
            message: Error message
            retry_after: Number of seconds to wait before retrying (from Retry-After header)
        """
        super().__init__(message)
        self.retry_after = retry_after
        self.status_code = 429


class DownloadError(MusicDLError):
    """Download failures."""


class MetadataError(MusicDLError):
    """Metadata embedding errors."""


class ConfigError(MusicDLError):
    """Configuration errors."""

