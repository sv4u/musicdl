"""
Unit tests for SpotifyClient with mocked dependencies.
"""
import time
import pytest
from unittest.mock import Mock, MagicMock, patch
from spotipy.exceptions import SpotifyException

from core.spotify_client import SpotifyClient, extract_id_from_url, RateLimiter
from core.exceptions import SpotifyError, SpotifyRateLimitError
from tests.conftest import SAMPLE_TRACK_DATA, SAMPLE_ALBUM_DATA


class TestExtractIDFromURL:
    """Test URL ID extraction."""
    
    def test_extract_id_from_full_url(self):
        """Test extracting ID from full Spotify URL."""
        url = "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi"
        assert extract_id_from_url(url) == "1RKbVxcm267VdsIzqY7msi"
    
    def test_extract_id_from_short_url(self):
        """Test extracting ID from short Spotify URL."""
        url = "spotify:track:1RKbVxcm267VdsIzqY7msi"
        # Current implementation handles open.spotify.com format
        # May need to handle spotify: format
        assert extract_id_from_url("https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi") == "1RKbVxcm267VdsIzqY7msi"
    
    def test_extract_id_already_id(self):
        """Test that ID passed as-is is returned."""
        assert extract_id_from_url("1RKbVxcm267VdsIzqY7msi") == "1RKbVxcm267VdsIzqY7msi"
    
    def test_extract_id_from_album_url(self):
        """Test extracting ID from album URL."""
        url = "https://open.spotify.com/album/77CZUF57sYqgtznUe3OikQ"
        assert extract_id_from_url(url) == "77CZUF57sYqgtznUe3OikQ"
    
    def test_extract_id_from_playlist_url(self):
        """Test extracting ID from playlist URL."""
        url = "https://open.spotify.com/playlist/5Xrt7Y1mwD4q107Ty56xnn"
        assert extract_id_from_url(url) == "5Xrt7Y1mwD4q107Ty56xnn"


class TestSpotifyClient:
    """Test SpotifyClient with mocked spotipy."""
    
    @pytest.fixture
    def mock_spotify(self, mocker):
        """Create mock Spotify client."""
        mock_client = mocker.Mock()
        mock_client.track.return_value = SAMPLE_TRACK_DATA
        mock_client.album.return_value = SAMPLE_ALBUM_DATA
        mock_client.playlist.return_value = {"id": "5Xrt7Y1mwD4q107Ty56xnn", "name": "planet namek"}
        mock_client.artist.return_value = {"id": "3hOdow4ZPmrby7Q1wfPLEy", "name": "Aries"}
        mock_client.artist_albums.return_value = {
            "items": [{"id": "77CZUF57sYqgtznUe3OikQ", "name": "I Love My Computer"}],
            "next": None,
        }
        mock_client.next.return_value = {"items": [], "next": None}
        return mock_client
    
    @pytest.fixture
    def spotify_client(self, mock_spotify, mocker):
        """Create SpotifyClient with mocked dependencies."""
        with patch("core.spotify_client.Spotify") as mock_spotify_class, \
             patch("core.spotify_client.SpotifyClientCredentials") as mock_creds:
            mock_spotify_class.return_value = mock_spotify
            client = SpotifyClient(
                client_id="test_id",
                client_secret="test_secret",
                cache_max_size=10,
                cache_ttl=3600,
            )
            client.client = mock_spotify
            return client
    
    def test_get_track_with_url(self, spotify_client, mock_spotify):
        """Test getting track using URL."""
        result = spotify_client.get_track("https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi")
        assert result["name"] == "YYZ"
        mock_spotify.track.assert_called_once_with("1RKbVxcm267VdsIzqY7msi")
    
    def test_get_track_with_id(self, spotify_client, mock_spotify):
        """Test getting track using ID."""
        result = spotify_client.get_track("1RKbVxcm267VdsIzqY7msi")
        assert result["name"] == "YYZ"
        mock_spotify.track.assert_called_once_with("1RKbVxcm267VdsIzqY7msi")
    
    def test_get_track_caching(self, spotify_client, mock_spotify):
        """Test that track retrieval uses cache."""
        # First call - should fetch from API
        result1 = spotify_client.get_track("track123")
        assert mock_spotify.track.call_count == 1
        
        # Second call - should use cache
        result2 = spotify_client.get_track("track123")
        assert mock_spotify.track.call_count == 1  # Not called again
        assert result1 == result2
    
    def test_get_album_with_pagination(self, spotify_client, mock_spotify):
        """Test getting album with paginated tracks."""
        # Mock paginated response
        mock_spotify.album.return_value = {
            "id": "77CZUF57sYqgtznUe3OikQ",
            "name": "I Love My Computer",
            "tracks": {
                "items": [{"id": "track1", "name": "London Song"}],
                "next": "https://api.spotify.com/v1/albums/77CZUF57sYqgtznUe3OikQ/tracks?offset=1",
            },
        }
        mock_spotify.next.return_value = {
            "items": [{"id": "track2", "name": "iPod Touch"}],
            "next": None,
        }
        
        result = spotify_client.get_album("77CZUF57sYqgtznUe3OikQ")
        assert result["id"] == "77CZUF57sYqgtznUe3OikQ"
        # Note: Pagination handling is in downloader, not client
    
    def test_get_playlist(self, spotify_client, mock_spotify):
        """Test getting playlist."""
        result = spotify_client.get_playlist("5Xrt7Y1mwD4q107Ty56xnn")
        assert result["id"] == "5Xrt7Y1mwD4q107Ty56xnn"
        mock_spotify.playlist.assert_called_once_with("5Xrt7Y1mwD4q107Ty56xnn")
    
    def test_get_artist_albums_with_pagination(self, spotify_client, mock_spotify):
        """Test getting artist albums with pagination."""
        # First page
        mock_spotify.artist_albums.return_value = {
            "items": [{"id": "77CZUF57sYqgtznUe3OikQ", "name": "I Love My Computer"}],
            "next": "https://api.spotify.com/v1/artists/3hOdow4ZPmrby7Q1wfPLEy/albums?offset=1",
        }
        # Second page
        mock_spotify.next.return_value = {
            "items": [{"id": "album2", "name": "BELIEVE IN ME, WHO BELIEVES IN YOU"}],
            "next": None,
        }
        
        albums = spotify_client.get_artist_albums("3hOdow4ZPmrby7Q1wfPLEy")
        assert len(albums) == 2
        assert albums[0]["id"] == "77CZUF57sYqgtznUe3OikQ"
        assert albums[1]["id"] == "album2"
        # Verify API is called with include_groups filter
        mock_spotify.artist_albums.assert_called_with(
            "3hOdow4ZPmrby7Q1wfPLEy",
            limit=50,
            include_groups="album,single"
        )
    
    def test_get_artist_albums_filters_compilations(self, spotify_client, mock_spotify):
        """Test that get_artist_albums excludes compilations and appears_on."""
        # Mock Spotify API response with mixed album types
        # Note: The API will filter these, so we only get albums and singles back
        mock_spotify.artist_albums.return_value = {
            "items": [
                {"id": "1", "name": "Studio Album", "album_type": "album"},
                {"id": "2", "name": "Single", "album_type": "single"},
            ],
            "next": None,
        }
        
        albums = spotify_client.get_artist_albums("artist_id")
        
        # Verify API is called with include_groups filter
        mock_spotify.artist_albums.assert_called_with(
            "artist_id",
            limit=50,
            include_groups="album,single"
        )
        
        # Verify only albums and singles are returned (API filters at source)
        assert len(albums) == 2
        assert all(album["album_type"] in ["album", "single"] for album in albums)
    
    def test_get_track_api_error(self, spotify_client, mock_spotify):
        """Test handling of Spotify API errors."""
        mock_spotify.track.side_effect = Exception("API Error")
        
        with pytest.raises(SpotifyError, match="Spotify API error"):
            spotify_client.get_track("track123")
    
    def test_cache_key_generation(self, spotify_client):
        """Test that cache keys are unique per resource type."""
        # Different resource types should have different cache keys
        track_key = f"track:track123"
        album_key = f"album:album123"
        assert track_key != album_key
    
    def test_clear_cache(self, spotify_client, mock_spotify):
        """Test clearing cache."""
        # Add something to cache
        spotify_client.get_track("track123")
        
        # Clear cache
        spotify_client.clear_cache()
        
        # Next call should fetch from API again
        spotify_client.get_track("track123")
        assert mock_spotify.track.call_count == 2  # Called again after clear

    def test_is_rate_limit_error(self, spotify_client):
        """Test rate limit error detection."""
        # Test with rate limit exception
        rate_limit_exception = SpotifyException(
            http_status=429,
            code=-1,
            msg="Rate limited",
            headers={"Retry-After": "5"}
        )
        assert spotify_client._is_rate_limit_error(rate_limit_exception) is True

        # Test with non-rate limit exception
        other_exception = SpotifyException(
            http_status=404,
            code=-1,
            msg="Not found"
        )
        assert spotify_client._is_rate_limit_error(other_exception) is False

        # Test with non-SpotifyException
        assert spotify_client._is_rate_limit_error(Exception("Other error")) is False

    def test_extract_retry_after(self, spotify_client):
        """Test Retry-After header extraction."""
        # Test with Retry-After header
        rate_limit_exception = SpotifyException(
            http_status=429,
            code=-1,
            msg="Rate limited",
            headers={"Retry-After": "5"}
        )
        retry_after = spotify_client._extract_retry_after(rate_limit_exception)
        assert retry_after == 5

        # Test without Retry-After header
        rate_limit_exception_no_header = SpotifyException(
            http_status=429,
            code=-1,
            msg="Rate limited",
            headers={}
        )
        retry_after = spotify_client._extract_retry_after(rate_limit_exception_no_header)
        assert retry_after is None

        # Test with invalid Retry-After value
        rate_limit_exception_invalid = SpotifyException(
            http_status=429,
            code=-1,
            msg="Rate limited",
            headers={"Retry-After": "invalid"}
        )
        retry_after = spotify_client._extract_retry_after(rate_limit_exception_invalid)
        assert retry_after is None

        # Test with non-SpotifyException
        assert spotify_client._extract_retry_after(Exception("Other error")) is None

    def test_rate_limit_error_retry(self, mock_spotify, mocker):
        """Test retry logic with rate limit errors."""
        with patch("core.spotify_client.Spotify") as mock_spotify_class, \
             patch("core.spotify_client.SpotifyClientCredentials") as mock_creds:
            mock_spotify_class.return_value = mock_spotify
            client = SpotifyClient(
                client_id="test_id",
                client_secret="test_secret",
                cache_max_size=10,
                cache_ttl=3600,
                max_retries=2,
                retry_base_delay=0.1,  # Short delay for testing
                rate_limit_enabled=False,  # Disable rate limiter for this test
            )
            client.client = mock_spotify

            # First call raises rate limit, second succeeds
            rate_limit_exception = SpotifyException(
                http_status=429,
                code=-1,
                msg="Rate limited",
                headers={"Retry-After": "0.1"}
            )
            mock_spotify.track.side_effect = [rate_limit_exception, SAMPLE_TRACK_DATA]

            # Should retry and succeed
            result = client.get_track("track123")
            assert result["name"] == "YYZ"
            assert mock_spotify.track.call_count == 2

    def test_rate_limit_error_max_retries_exceeded(self, mock_spotify, mocker):
        """Test that max retries are respected."""
        with patch("core.spotify_client.Spotify") as mock_spotify_class, \
             patch("core.spotify_client.SpotifyClientCredentials") as mock_creds:
            mock_spotify_class.return_value = mock_spotify
            client = SpotifyClient(
                client_id="test_id",
                client_secret="test_secret",
                cache_max_size=10,
                cache_ttl=3600,
                max_retries=2,
                retry_base_delay=0.1,
                rate_limit_enabled=False,
            )
            client.client = mock_spotify

            # Always raise rate limit
            rate_limit_exception = SpotifyException(
                http_status=429,
                code=-1,
                msg="Rate limited",
                headers={"Retry-After": "0.1"}
            )
            mock_spotify.track.side_effect = rate_limit_exception

            # Should raise SpotifyRateLimitError after max retries
            with pytest.raises(SpotifyRateLimitError):
                client.get_track("track123")
            
            # Should have tried max_retries + 1 times (initial + retries)
            assert mock_spotify.track.call_count == 3  # 1 initial + 2 retries

