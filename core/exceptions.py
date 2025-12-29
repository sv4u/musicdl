"""
Custom exceptions for musicdl.
"""


class MusicDLError(Exception):
    """Base exception for all musicdl errors."""


class SpotifyError(MusicDLError):
    """Spotify API errors."""


class DownloadError(MusicDLError):
    """Download failures."""


class MetadataError(MusicDLError):
    """Metadata embedding errors."""


class ConfigError(MusicDLError):
    """Configuration errors."""

