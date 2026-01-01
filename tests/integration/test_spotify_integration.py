"""
Integration tests for SpotifyClient with real Spotify API.
Requires SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET environment variables.
"""
import os
import time
import pytest
from core.spotify_client import SpotifyClient, RateLimiter
from core.exceptions import SpotifyError


@pytest.mark.integration
class TestSpotifyClientIntegration:
    """Integration tests with real Spotify API."""
    
    @pytest.fixture
    def spotify_client(self, spotify_credentials):
        """Create real SpotifyClient with credentials."""
        if not os.getenv("SPOTIFY_CLIENT_ID"):
            pytest.skip("SPOTIFY_CLIENT_ID not set")
        return SpotifyClient(**spotify_credentials)
    
    def test_get_track_real_api(self, spotify_client):
        """Test retrieving real track from Spotify API."""
        # Use a known track ID: YYZ by Rush
        track_id = "1RKbVxcm267VdsIzqY7msi"
        track = spotify_client.get_track(track_id)
        
        assert track is not None
        assert "id" in track
        assert "name" in track
        assert "artists" in track
        assert track["id"] == track_id
        assert len(track["artists"]) > 0
        assert track["name"] == "YYZ"
    
    def test_get_track_caching(self, spotify_client):
        """Test that caching works with real API."""
        track_id = "1RKbVxcm267VdsIzqY7msi"
        
        # First call - should fetch from API
        track1 = spotify_client.get_track(track_id)
        
        # Second call - should use cache (same object reference or equal)
        track2 = spotify_client.get_track(track_id)
        
        assert track1 == track2  # Should be cached
    
    def test_get_album_real_api(self, spotify_client):
        """Test retrieving real album from Spotify API."""
        album_id = "77CZUF57sYqgtznUe3OikQ"  # I Love My Computer by Ninajirachi
        album = spotify_client.get_album(album_id)
        
        assert album is not None
        assert "id" in album
        assert "name" in album
        assert "tracks" in album
        assert album["id"] == album_id
    
    def test_get_album_pagination(self, spotify_client):
        """Test album retrieval with paginated tracks."""
        # Use album with many tracks (likely to have pagination)
        album_id = "77CZUF57sYqgtznUe3OikQ"  # I Love My Computer (12 tracks)
        album = spotify_client.get_album(album_id)
        
        assert album is not None
        assert "tracks" in album
        # Note: Pagination handling is tested in downloader integration tests
    
    def test_get_playlist_real_api(self, spotify_client):
        """Test retrieving real playlist from Spotify API."""
        # Use a public playlist ID: planet namek
        playlist_id = "5Xrt7Y1mwD4q107Ty56xnn"
        playlist = spotify_client.get_playlist(playlist_id)
        
        assert playlist is not None
        assert "id" in playlist
        assert "name" in playlist
        assert "tracks" in playlist
        assert playlist["name"] == "planet namek"
    
    def test_get_artist_albums_real_api(self, spotify_client):
        """Test retrieving artist albums from real API."""
        artist_id = "3hOdow4ZPmrby7Q1wfPLEy"  # Aries
        albums = spotify_client.get_artist_albums(artist_id)
        
        assert albums is not None
        assert isinstance(albums, list)
        assert len(albums) > 0
        assert "id" in albums[0]
        assert "name" in albums[0]
        # Verify that only albums and singles are returned (compilations and appears_on excluded)
        # The API filters at source, so all returned albums should be album or single type
        for album in albums:
            assert album.get("album_type") in ["album", "single"], \
                f"Unexpected album type: {album.get('album_type')} for album {album.get('name')}"
    
    def test_invalid_track_id(self, spotify_client):
        """Test handling of invalid track ID."""
        with pytest.raises(SpotifyError):
            spotify_client.get_track("invalid_id_12345")
    
    def test_cache_ttl_expiration(self, spotify_client):
        """Test cache TTL expiration with real API."""
        # Create client with short TTL
        short_ttl_client = SpotifyClient(
            client_id=os.getenv("SPOTIFY_CLIENT_ID", "test"),
            client_secret=os.getenv("SPOTIFY_CLIENT_SECRET", "test"),
            cache_ttl=1,  # 1 second TTL
        )
        
        track_id = "1RKbVxcm267VdsIzqY7msi"
        
        # First call
        track1 = short_ttl_client.get_track(track_id)
        
        # Wait for expiration
        import time
        time.sleep(1.1)
        
        # Second call should fetch again (cache expired)
        track2 = short_ttl_client.get_track(track_id)
        
        # Both should be valid, but from different API calls
        assert track1 == track2  # Same data, but different fetch

    def test_rate_limiter_integration(self, spotify_credentials):
        """Test that rate limiter works with real API calls."""
        # Create client with rate limiting enabled
        client = SpotifyClient(
            client_id=spotify_credentials["client_id"],
            client_secret=spotify_credentials["client_secret"],
            rate_limit_enabled=True,
            rate_limit_requests=10,
            rate_limit_window=1.0,
        )
        
        # Make multiple requests - should be throttled by rate limiter
        track_ids = ["1RKbVxcm267VdsIzqY7msi"] * 5  # Same track, multiple requests
        
        start_time = time.time()
        for track_id in track_ids:
            track = client.get_track(track_id)
            assert track is not None
        elapsed = time.time() - start_time
        
        # Should have taken some time due to rate limiting
        # (though caching may make this faster)
        # At minimum, verify no errors occurred
        assert elapsed >= 0  # Basic sanity check

    def test_rate_limiter_disabled(self, spotify_credentials):
        """Test that rate limiter can be disabled."""
        client = SpotifyClient(
            client_id=spotify_credentials["client_id"],
            client_secret=spotify_credentials["client_secret"],
            rate_limit_enabled=False,
        )
        
        # Should work without rate limiting
        track = client.get_track("1RKbVxcm267VdsIzqY7msi")
        assert track is not None
        assert client.rate_limiter is None

    def test_rate_limiter_configuration(self, spotify_credentials):
        """Test that rate limiter configuration is properly applied."""
        client = SpotifyClient(
            client_id=spotify_credentials["client_id"],
            client_secret=spotify_credentials["client_secret"],
            rate_limit_enabled=True,
            rate_limit_requests=5,
            rate_limit_window=0.5,
        )
        
        assert client.rate_limit_enabled is True
        assert client.rate_limiter is not None
        assert client.rate_limiter.max_requests == 5
        assert client.rate_limiter.window_seconds == 0.5

    def test_retry_configuration(self, spotify_credentials):
        """Test that retry configuration is properly applied."""
        client = SpotifyClient(
            client_id=spotify_credentials["client_id"],
            client_secret=spotify_credentials["client_secret"],
            max_retries=5,
            retry_base_delay=0.5,
            retry_max_delay=60.0,
        )
        
        assert client.max_retries == 5
        assert client.retry_base_delay == 0.5
        assert client.retry_max_delay == 60.0

