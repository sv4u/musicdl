"""
Spotify API client wrapper with caching.
"""

import re
from typing import Any, Callable, Dict, List, Optional

from spotipy import Spotify
from spotipy.oauth2 import SpotifyClientCredentials

from core.cache import TTLCache
from core.exceptions import SpotifyError


def extract_id_from_url(url: str) -> str:
    """Extract Spotify ID from URL."""
    # Pattern: https://open.spotify.com/{type}/{id}
    match = re.search(r"spotify\.com/(\w+)/([a-zA-Z0-9]+)", url)
    if match:
        return match.group(2)
    # If already an ID, return as-is
    return url


class SpotifyClient:
    """Simplified Spotify API client wrapper with caching."""

    def __init__(
        self,
        client_id: str,
        client_secret: str,
        cache_max_size: int = 1000,
        cache_ttl: int = 3600,
    ):
        """
        Initialize with credentials and cache settings.

        Args:
            client_id: Spotify API client ID
            client_secret: Spotify API client secret
            cache_max_size: Maximum cache entries (LRU eviction)
            cache_ttl: Cache TTL in seconds (default: 1 hour)
        """
        credentials = SpotifyClientCredentials(
            client_id=client_id, client_secret=client_secret
        )
        self.client = Spotify(auth_manager=credentials)
        self.cache = TTLCache(max_size=cache_max_size, ttl_seconds=cache_ttl)

    def _get_cached_or_fetch(self, cache_key: str, fetch_func: Callable[[], Any]) -> Any:
        """Get from cache or fetch and cache the result."""
        # Try cache first
        cached = self.cache.get(cache_key)
        if cached is not None:
            return cached

        # Fetch from API
        try:
            result = fetch_func()
        except Exception as e:
            raise SpotifyError(f"Spotify API error: {e}") from e

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
        """Get all albums for an artist (cached)."""
        artist_id = extract_id_from_url(artist_id_or_url)
        cache_key = f"artist_albums:{artist_id}"

        def fetch_albums() -> List[Dict[str, Any]]:
            """Fetch all albums for artist with pagination."""
            albums = []
            results = self.client.artist_albums(artist_id, limit=50)
            albums.extend(results.get("items", []))

            # Handle pagination
            while results.get("next"):
                results = self.client.next(results)
                albums.extend(results.get("items", []))

            return albums

        return self._get_cached_or_fetch(cache_key, fetch_albums)

    def clear_cache(self) -> None:
        """Clear the cache (useful for testing or forced refresh)."""
        self.cache.clear()

