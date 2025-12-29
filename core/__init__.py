"""
Core modules for musicdl download functionality.
"""

from core.audio_provider import AudioProvider
from core.cache import TTLCache
from core.downloader import Downloader
from core.exceptions import (
    DownloadError,
    MetadataError,
    MusicDLError,
    SpotifyError,
)
from core.metadata import MetadataEmbedder
from core.models import DownloadResult, Song
from core.spotify_client import SpotifyClient

__all__ = [
    "SpotifyClient",
    "AudioProvider",
    "MetadataEmbedder",
    "Downloader",
    "Song",
    "DownloadResult",
    "MusicDLError",
    "SpotifyError",
    "DownloadError",
    "MetadataError",
    "TTLCache",
]

