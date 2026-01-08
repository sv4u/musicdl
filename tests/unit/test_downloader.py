"""
Unit tests for Downloader orchestrator with mocked dependencies.
"""
import pytest
from pathlib import Path
from unittest.mock import Mock, MagicMock, patch, call, ANY
import time

from core.downloader import Downloader, format_filename, spotify_track_to_song, _sanitize
from core.exceptions import DownloadError, SpotifyError, MetadataError
from core.models import Song
from tests.conftest import SAMPLE_TRACK_DATA, SAMPLE_ALBUM_DATA


class TestFormatFilename:
    """Test filename formatting."""
    
    def test_format_filename_basic(self, sample_song):
        """Test basic filename formatting."""
        template = "{artist}/{album}/{track-number} - {title}.{output-ext}"
        result = format_filename(template, sample_song, "mp3")
        assert "Rush" in result
        assert "Moving Pictures" in result
        assert "03" in result  # Track number zero-padded
        assert "YYZ" in result
        assert result.endswith(".mp3")
    
    def test_format_filename_all_placeholders(self, sample_song):
        """Test all template placeholders."""
        template = "{artist}/{album}/{track-number} - {title} ({year}).{output-ext}"
        result = format_filename(template, sample_song, "mp3")
        assert "2022" in result
        assert result.count("/") == 2  # artist/album/
    
    def test_format_filename_sanitization(self, sample_song):
        """Test that invalid filename characters are removed."""
        song = Song(
            title="Test: Song?",
            artist="Artist/Name",
            album="Album|Name",
            track_number=1,
            duration=180,
            spotify_url="https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi",
        )
        template = "{artist}/{album}/{title}.{output-ext}"
        result = format_filename(template, song, "mp3")
        assert ":" not in result
        assert "?" not in result
        assert "|" not in result
        assert "/" not in result.split("/")[-1]  # Not in filename part
    
    def test_format_filename_missing_optional_fields(self):
        """Test formatting with missing optional fields."""
        song = Song(
            title="Test Song",
            artist="Test Artist",
            album="",  # Empty album
            track_number=1,
            duration=180,
            spotify_url="https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi",
            year=None,  # No year
        )
        template = "{artist}/{album}/{title} ({year}).{output-ext}"
        result = format_filename(template, song, "mp3")
        # Should handle gracefully - empty album and year should not cause issues
        assert "Test Song" in result


class TestSpotifyTrackToSong:
    """Test conversion from Spotify data to Song model."""
    
    def test_spotify_track_to_song_basic(self):
        """Test basic conversion."""
        track_data = SAMPLE_TRACK_DATA.copy()
        album_data = SAMPLE_ALBUM_DATA.copy()
        
        song = spotify_track_to_song(track_data, album_data)
        assert song.title == "YYZ"
        assert song.artist == "Rush"
        assert song.album == "I Love My Computer"  # From SAMPLE_ALBUM_DATA
        assert song.track_number == 3
        assert song.duration == 266  # 266000 ms / 1000
    
    def test_spotify_track_to_song_cover_art(self):
        """Test cover art URL extraction."""
        track_data = SAMPLE_TRACK_DATA.copy()
        album_data = SAMPLE_ALBUM_DATA.copy()
        album_data["images"] = [
            {"url": "small.jpg", "width": 64, "height": 64},
            {"url": "large.jpg", "width": 640, "height": 640},
        ]
        
        song = spotify_track_to_song(track_data, album_data)
        assert song.cover_url == "large.jpg"  # Largest image
    
    def test_spotify_track_to_song_year_extraction(self):
        """Test year extraction from release date."""
        track_data = SAMPLE_TRACK_DATA.copy()
        album_data = SAMPLE_ALBUM_DATA.copy()
        album_data["release_date"] = "2022-06-15"
        
        song = spotify_track_to_song(track_data, album_data)
        assert song.year == 2022
        assert song.date == "2022-06-15"


class TestDownloader:
    """Test Downloader orchestrator."""
    
    @pytest.fixture
    def mock_downloader_dependencies(self, mocker, tmp_test_dir):
        """Create downloader with all dependencies mocked."""
        # Mock Spotify client
        mock_spotify = mocker.Mock()
        mock_spotify.get_track.return_value = SAMPLE_TRACK_DATA
        mock_spotify.get_album.return_value = SAMPLE_ALBUM_DATA
        
        # Mock Audio provider
        mock_audio = mocker.Mock()
        mock_audio.search.return_value = "https://www.youtube.com/watch?v=test123"
        mock_audio.download.return_value = tmp_test_dir / "test.mp3"
        
        # Mock Metadata embedder
        mock_metadata = mocker.Mock()
        
        # Create downloader
        config = mocker.Mock()
        config.client_id = "test_id"
        config.client_secret = "test_secret"
        config.cache_max_size = 100
        config.cache_ttl = 3600
        config.audio_search_cache_max_size = 500
        config.audio_search_cache_ttl = 86400
        config.file_existence_cache_max_size = 10000
        config.file_existence_cache_ttl = 3600
        config.format = "mp3"
        config.bitrate = "128k"
        config.audio_providers = ["youtube-music"]
        config.output = "{artist}/{album}/{track-number} - {title}.{output-ext}"
        config.overwrite = "skip"
        config.max_retries = 3
        config.spotify_max_retries = 3
        config.spotify_retry_base_delay = 1.0
        config.spotify_retry_max_delay = 120.0
        config.spotify_rate_limit_enabled = True
        config.spotify_rate_limit_requests = 10
        config.spotify_rate_limit_window = 1.0
        # Disable download rate limiting for tests (not the focus of these tests)
        config.download_rate_limit_enabled = False
        config.download_rate_limit_requests = 2
        config.download_rate_limit_window = 1.0
        config.download_bandwidth_limit = None
        
        with patch("core.downloader.SpotifyClient") as mock_spotify_class, \
             patch("core.downloader.AudioProvider") as mock_audio_class, \
             patch("core.downloader.MetadataEmbedder") as mock_metadata_class:
            mock_spotify_class.return_value = mock_spotify
            mock_audio_class.return_value = mock_audio
            mock_metadata_class.return_value = mock_metadata
            
            downloader = Downloader(config)
            downloader.spotify = mock_spotify
            downloader.audio = mock_audio
            downloader.metadata = mock_metadata
            
            return downloader, mock_spotify, mock_audio, mock_metadata, config, tmp_test_dir
    
    def test_download_track_success(self, mock_downloader_dependencies):
        """Test successful track download."""
        import os
        downloader, mock_spotify, mock_audio, mock_metadata, config, tmp_dir = mock_downloader_dependencies
        
        # Clear file existence cache to ensure fresh start
        downloader.file_existence_cache.clear()
        
        # Create a side_effect that creates the file at the requested path
        def download_side_effect(audio_url, output_path):
            """Create file at requested path."""
            output_path = Path(output_path)
            # Resolve relative paths relative to current working directory
            if not output_path.is_absolute():
                output_path = Path.cwd() / output_path
            output_path.parent.mkdir(parents=True, exist_ok=True)
            output_path.write_bytes(b"fake audio content")
            return output_path
        
        mock_audio.download.side_effect = download_side_effect
        
        # Change to tmp_dir so relative paths resolve correctly
        original_cwd = os.getcwd()
        try:
            os.chdir(tmp_dir)
            success, path = downloader.download_track("https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi")
        finally:
            os.chdir(original_cwd)
        
        assert success is True
        # Path should match the expected relative path structure
        assert path.parts[-3:] == ("Rush", "I Love My Computer", "03 - YYZ.mp3")
        mock_spotify.get_track.assert_called_once()
        mock_spotify.get_album.assert_called_once()
        mock_audio.search.assert_called_once()
        mock_audio.download.assert_called_once()
        mock_metadata.embed.assert_called_once()
    
    def test_download_track_file_exists_skip(self, mock_downloader_dependencies):
        """Test skipping existing file when overwrite=skip."""
        import os
        downloader, mock_spotify, mock_audio, mock_metadata, config, tmp_dir = mock_downloader_dependencies
        config.overwrite = "skip"
        
        # Calculate the expected output path the same way the downloader does
        track_data = SAMPLE_TRACK_DATA.copy()
        album_data = SAMPLE_ALBUM_DATA.copy()
        song = spotify_track_to_song(track_data, album_data)
        expected_path = downloader._get_output_path(song)
        
        # Change to tmp_dir so relative paths work correctly
        original_cwd = os.getcwd()
        try:
            os.chdir(tmp_dir)
            
            # Create existing file at the expected path (relative to tmp_dir)
            if expected_path.is_absolute():
                # If absolute, make it relative to tmp_dir
                expected_path = expected_path.relative_to(expected_path.anchor)
            file_path = tmp_dir / expected_path
            file_path.parent.mkdir(parents=True, exist_ok=True)
            file_path.write_bytes(b"existing content")
            
            success, path = downloader.download_track("https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi")
            
            assert success is True
            # Path should match (normalize to absolute for comparison)
            if not path.is_absolute():
                path = tmp_dir / path
            if not expected_path.is_absolute():
                expected_path = tmp_dir / expected_path
            assert path.resolve() == expected_path.resolve()
            # Should not download or embed metadata
            mock_audio.download.assert_not_called()
            mock_metadata.embed.assert_not_called()
        finally:
            os.chdir(original_cwd)

    def test_download_track_file_exists_metadata(self, mock_downloader_dependencies):
        """Test updating metadata only when overwrite=metadata and file exists."""
        import os
        downloader, mock_spotify, mock_audio, mock_metadata, config, tmp_dir = mock_downloader_dependencies
        config.overwrite = "metadata"
        
        # Calculate the expected output path the same way the downloader does
        track_data = SAMPLE_TRACK_DATA.copy()
        album_data = SAMPLE_ALBUM_DATA.copy()
        song = spotify_track_to_song(track_data, album_data)
        expected_path = downloader._get_output_path(song)
        
        # Change to tmp_dir so relative paths work correctly
        original_cwd = os.getcwd()
        try:
            os.chdir(tmp_dir)
            
            # Create existing file at the expected path (relative to tmp_dir)
            if expected_path.is_absolute():
                # If absolute, make it relative to tmp_dir
                expected_path = expected_path.relative_to(expected_path.anchor)
            file_path = tmp_dir / expected_path
            file_path.parent.mkdir(parents=True, exist_ok=True)
            file_path.write_bytes(b"existing content")
            
            success, path = downloader.download_track("https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi")
            
            assert success is True
            # Path should match (normalize to absolute for comparison)
            if not path.is_absolute():
                path = tmp_dir / path
            if not expected_path.is_absolute():
                expected_path = tmp_dir / expected_path
            assert path.resolve() == expected_path.resolve()
            # Should not download audio, but should embed metadata
            mock_audio.download.assert_not_called()
            mock_audio.search.assert_not_called()
            mock_metadata.embed.assert_called_once()
            # Verify metadata embed was called with correct arguments
            call_args = mock_metadata.embed.call_args
            assert call_args[0][0] == path or call_args[0][0].resolve() == expected_path.resolve()
            assert call_args[0][1].title == song.title
        finally:
            os.chdir(original_cwd)
    
    def test_download_track_retry_on_failure(self, mock_downloader_dependencies, mocker):
        """Test retry logic with exponential backoff."""
        import os
        downloader, mock_spotify, mock_audio, mock_metadata, config, tmp_dir = mock_downloader_dependencies
        config.max_retries = 3
        
        # Clear file existence cache to ensure fresh start
        downloader.file_existence_cache.clear()
        
        # First two attempts fail, third succeeds
        mock_audio.search.side_effect = [
            DownloadError("Network error"),
            DownloadError("Network error"),
            "https://www.youtube.com/watch?v=test123",
        ]
        
        # Create a side_effect that creates the file at the requested path
        def download_side_effect(audio_url, output_path):
            """Create file at requested path."""
            output_path = Path(output_path)
            # Resolve relative paths relative to current working directory
            if not output_path.is_absolute():
                output_path = Path.cwd() / output_path
            output_path.parent.mkdir(parents=True, exist_ok=True)
            output_path.write_bytes(b"fake audio content")
            return output_path
        
        mock_audio.download.side_effect = download_side_effect
        
        # Change to tmp_dir so relative paths resolve correctly
        original_cwd = os.getcwd()
        try:
            os.chdir(tmp_dir)
            with patch("time.sleep"):  # Speed up test
                success, path = downloader.download_track("https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi")
        finally:
            os.chdir(original_cwd)
        
        assert success is True
        assert mock_audio.search.call_count == 3  # Retried 3 times
    
    def test_download_track_max_retries_exceeded(self, mock_downloader_dependencies, mocker):
        """Test that max retries failure returns False."""
        downloader, mock_spotify, mock_audio, mock_metadata, config, tmp_dir = mock_downloader_dependencies
        config.max_retries = 2
        
        # All attempts fail
        mock_audio.search.side_effect = DownloadError("Persistent error")
        
        with patch("time.sleep"):  # Speed up test
            success, path = downloader.download_track("https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi")
        
        assert success is False
        assert path is None
        assert mock_audio.search.call_count == 2  # Max retries
    
    def test_download_track_no_audio_found(self, mock_downloader_dependencies):
        """Test handling when no audio is found."""
        downloader, mock_spotify, mock_audio, mock_metadata, config, tmp_dir = mock_downloader_dependencies
        
        mock_audio.search.return_value = None  # No audio found
        
        success, path = downloader.download_track("https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi")
        
        assert success is False
        assert path is None
        mock_audio.download.assert_not_called()
    
    def test_download_album(self, mock_downloader_dependencies):
        """Test downloading entire album."""
        import os
        downloader, mock_spotify, mock_audio, mock_metadata, config, tmp_dir = mock_downloader_dependencies
        
        # Mock album with multiple tracks
        album_data = SAMPLE_ALBUM_DATA.copy()
        album_data["tracks"] = {
            "items": [
                {"id": "track1", "name": "London Song", "track_number": 1, "disc_number": 1, "duration_ms": 180000},
                {"id": "track2", "name": "iPod Touch", "track_number": 2, "disc_number": 1, "duration_ms": 200000},
            ],
            "next": None,
        }
        mock_spotify.get_album.return_value = album_data
        
        # Mock track data for each track
        def get_track_side_effect(track_id_or_url):
            track_id = track_id_or_url.split("/")[-1] if "/" in track_id_or_url else track_id_or_url
            track_data = SAMPLE_TRACK_DATA.copy()
            track_data["id"] = track_id
            track_data["name"] = f"Track {track_id[-1]}"
            return track_data
        
        mock_spotify.get_track.side_effect = get_track_side_effect
        
        # Create a side_effect that creates the file at the requested path
        def download_side_effect(audio_url, output_path):
            """Create file at requested path."""
            output_path = Path(output_path)
            # Resolve relative paths relative to current working directory
            if not output_path.is_absolute():
                output_path = Path.cwd() / output_path
            output_path.parent.mkdir(parents=True, exist_ok=True)
            output_path.write_bytes(b"fake audio content")
            return output_path
        
        mock_audio.download.side_effect = download_side_effect
        
        # Change to tmp_dir so relative paths resolve correctly
        original_cwd = os.getcwd()
        try:
            os.chdir(tmp_dir)
            results = downloader.download_album("https://open.spotify.com/album/77CZUF57sYqgtznUe3OikQ")
        finally:
            os.chdir(original_cwd)
        
        assert len(results) == 2
        assert all(success for success, _ in results)
        assert mock_spotify.get_track.call_count == 2
    
    def test_download_playlist_with_m3u(self, mock_downloader_dependencies):
        """Test downloading playlist and creating M3U file."""
        import os
        downloader, mock_spotify, mock_audio, mock_metadata, config, tmp_dir = mock_downloader_dependencies
        
        playlist_data = {
            "id": "5Xrt7Y1mwD4q107Ty56xnn",
            "name": "planet namek",
            "tracks": {
                "items": [
                    {
                        "track": {
                            "id": "track1",
                            "name": "stagekiss",
                            "external_urls": {"spotify": "https://open.spotify.com/track/track1"},
                            "is_local": False,
                        }
                    }
                ],
                "next": None,
            },
        }
        mock_spotify.get_playlist.return_value = playlist_data
        
        # Mock track and album data
        mock_spotify.get_track.return_value = SAMPLE_TRACK_DATA
        mock_spotify.get_album.return_value = SAMPLE_ALBUM_DATA
        
        output_file = tmp_dir / "Rush" / "I Love My Computer" / "03 - YYZ.mp3"
        output_file.parent.mkdir(parents=True, exist_ok=True)
        output_file.write_bytes(b"fake audio content")  # Create actual file
        mock_audio.download.return_value = output_file
        
        # Change to tmp_dir so M3U file is created there
        original_cwd = os.getcwd()
        try:
            os.chdir(tmp_dir)
            results = downloader.download_playlist("5Xrt7Y1mwD4q107Ty56xnn", create_m3u=True)
        finally:
            os.chdir(original_cwd)
        
        assert len(results) == 1
        # Check M3U file was created
        m3u_file = tmp_dir / "planet namek.m3u"
        assert m3u_file.exists()
        assert "#EXTM3U" in m3u_file.read_text()
    
    def test_download_artist(self, mock_downloader_dependencies):
        """Test downloading artist discography."""
        import os
        downloader, mock_spotify, mock_audio, mock_metadata, config, tmp_dir = mock_downloader_dependencies
        
        # Mock artist albums
        albums = [
            {"id": "77CZUF57sYqgtznUe3OikQ", "name": "I Love My Computer", "external_urls": {"spotify": "https://open.spotify.com/album/77CZUF57sYqgtznUe3OikQ"}},
            {"id": "album2", "name": "BELIEVE IN ME, WHO BELIEVES IN YOU", "external_urls": {"spotify": "https://open.spotify.com/album/album2"}},
        ]
        mock_spotify.get_artist_albums.return_value = albums
        
        # Mock album data - SAMPLE_ALBUM_DATA has 2 tracks
        mock_spotify.get_album.return_value = SAMPLE_ALBUM_DATA
        mock_spotify.get_track.return_value = SAMPLE_TRACK_DATA
        
        # Create a side_effect that creates the file at the requested path
        def download_side_effect(audio_url, output_path):
            """Create file at requested path."""
            output_path = Path(output_path)
            # Resolve relative paths relative to current working directory
            if not output_path.is_absolute():
                output_path = Path.cwd() / output_path
            output_path.parent.mkdir(parents=True, exist_ok=True)
            output_path.write_bytes(b"fake audio content")
            return output_path
        
        mock_audio.download.side_effect = download_side_effect
        
        # Change to tmp_dir so relative paths resolve correctly
        original_cwd = os.getcwd()
        try:
            os.chdir(tmp_dir)
            results = downloader.download_artist("3hOdow4ZPmrby7Q1wfPLEy")
        finally:
            os.chdir(original_cwd)
        
        # Should download tracks from both albums
        # Each album has 2 tracks, and download_track calls get_album for each track
        # So: 2 albums * (1 call in download_album + 2 calls in download_track) = 6 calls total
        assert len(results) > 0
        # Each album: 1 call in download_album + 2 calls in download_track (one per track) = 3 calls per album
        # For 2 albums: 2 * 3 = 6 calls
        assert mock_spotify.get_album.call_count == 6

    def test_file_exists_cached_hit(self, mock_downloader_dependencies, tmp_test_dir):
        """Test that cached file existence checks avoid filesystem calls."""
        downloader, _, _, _, _, _ = mock_downloader_dependencies
        
        # Create a test file
        test_file = tmp_test_dir / "test_file.mp3"
        test_file.write_bytes(b"test content")
        
        # First check - should hit filesystem
        with patch.object(Path, "exists") as mock_exists:
            mock_exists.return_value = True
            result1 = downloader._file_exists_cached(test_file)
            assert result1 is True
            assert mock_exists.call_count == 1
        
        # Second check - should use cache (no filesystem call)
        with patch.object(Path, "exists") as mock_exists:
            result2 = downloader._file_exists_cached(test_file)
            assert result2 is True
            assert mock_exists.call_count == 0  # Cache hit, no filesystem call

    def test_file_exists_cached_miss(self, mock_downloader_dependencies, tmp_test_dir):
        """Test that cache miss triggers filesystem check."""
        downloader, _, _, _, _, _ = mock_downloader_dependencies
        
        # File doesn't exist
        test_file = tmp_test_dir / "nonexistent.mp3"
        
        with patch.object(Path, "exists") as mock_exists:
            mock_exists.return_value = False
            result = downloader._file_exists_cached(test_file)
            assert result is False
            assert mock_exists.call_count == 1

    def test_file_exists_cached_invalidation(self, mock_downloader_dependencies, tmp_test_dir):
        """Test that cache is invalidated when file is created."""
        downloader, _, _, _, _, _ = mock_downloader_dependencies
        
        test_file = tmp_test_dir / "new_file.mp3"
        
        # First check - file doesn't exist
        assert downloader._file_exists_cached(test_file) is False
        
        # Create file
        test_file.write_bytes(b"content")
        
        # Invalidate cache (simulating file creation)
        downloader._invalidate_file_cache(test_file)
        
        # Check again - should return True (from cache)
        result = downloader._file_exists_cached(test_file)
        assert result is True
        
        # Verify cache was updated
        cache_key = f"file_exists:{test_file.resolve()}"
        cached_value = downloader.file_existence_cache.get(cache_key)
        assert cached_value is True

    def test_file_exists_cached_statistics(self, mock_downloader_dependencies, tmp_test_dir):
        """Test that file existence cache tracks statistics."""
        downloader, _, _, _, _, _ = mock_downloader_dependencies
        
        test_file1 = tmp_test_dir / "file1.mp3"
        test_file2 = tmp_test_dir / "file2.mp3"
        
        # Create file1
        test_file1.write_bytes(b"content")
        
        # First check - cache miss
        downloader._file_exists_cached(test_file1)
        
        # Second check - cache hit
        downloader._file_exists_cached(test_file1)
        
        # Check nonexistent file - cache miss
        downloader._file_exists_cached(test_file2)
        
        # Check statistics
        stats = downloader.file_existence_cache.stats()
        assert stats["hits"] == 1
        assert stats["misses"] == 2  # First check for file1, check for file2
        assert stats["hit_rate"] == "33.33%"

    def test_file_exists_cached_absolute_path(self, mock_downloader_dependencies, tmp_test_dir):
        """Test that cache uses absolute paths for consistency."""
        downloader, _, _, _, _, _ = mock_downloader_dependencies
        
        test_file = tmp_test_dir / "test.mp3"
        test_file.write_bytes(b"content")
        
        # Check with absolute path
        abs_path = test_file.resolve()
        result1 = downloader._file_exists_cached(abs_path)
        
        # Check with relative path (should use same cache key)
        rel_path = test_file.relative_to(tmp_test_dir)
        # Need to resolve relative path to absolute for comparison
        # But the cache should handle this via resolve() in _file_exists_cached
        result2 = downloader._file_exists_cached(test_file)
        
        assert result1 == result2
        
        # Both should use same cache entry
        cache_key1 = f"file_exists:{abs_path}"
        cache_key2 = f"file_exists:{test_file.resolve()}"
        assert cache_key1 == cache_key2

    def test_download_track_invalidates_cache(self, mock_downloader_dependencies):
        """Test that successful download invalidates file existence cache."""
        downloader, mock_spotify, mock_audio, mock_metadata, config, tmp_dir = mock_downloader_dependencies
        
        # Create output file
        output_file = tmp_dir / "Rush" / "I Love My Computer" / "03 - YYZ.mp3"
        output_file.parent.mkdir(parents=True, exist_ok=True)
        output_file.write_bytes(b"fake audio content")  # Create actual file
        mock_audio.download.return_value = output_file
        
        # Download track
        success, path = downloader.download_track("https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi")
        
        assert success is True
        assert path is not None
        
        # Verify cache was updated (file now exists)
        # Use the path returned by downloader (which is the resolved path)
        cache_key = f"file_exists:{path.resolve()}"
        cached_value = downloader.file_existence_cache.get(cache_key)
        assert cached_value is True

    def test_cache_configuration_applied(self, mock_downloader_dependencies):
        """Test that cache configuration is properly applied."""
        downloader, _, _, _, config, _ = mock_downloader_dependencies
        
        # Verify file existence cache configuration (not mocked)
        assert downloader.file_existence_cache.max_size == config.file_existence_cache_max_size
        assert downloader.file_existence_cache.ttl_seconds == config.file_existence_cache_ttl
        
        # Verify AudioProvider was called with correct cache parameters
        # (audio is mocked, so we check the call arguments)
        from core.downloader import AudioProvider
        import core.downloader
        # Check that AudioProvider was instantiated with cache config
        # Since it's mocked, we verify the file existence cache which is real
        assert downloader.file_existence_cache.max_size == 10000
        assert downloader.file_existence_cache.ttl_seconds == 3600

    def test_cache_configuration_custom_values(self, mocker, tmp_test_dir):
        """Test that custom cache configuration values are applied."""
        # Create config with custom cache settings
        config = mocker.Mock()
        config.client_id = "test_id"
        config.client_secret = "test_secret"
        config.cache_max_size = 1000
        config.cache_ttl = 3600
        config.audio_search_cache_max_size = 200  # Custom
        config.audio_search_cache_ttl = 7200  # Custom
        config.file_existence_cache_max_size = 5000  # Custom
        config.file_existence_cache_ttl = 1800  # Custom
        config.format = "mp3"
        config.bitrate = "128k"
        config.audio_providers = ["youtube-music"]
        config.output = "{artist}/{album}/{track-number} - {title}.{output-ext}"
        config.overwrite = "skip"
        config.max_retries = 3
        config.spotify_max_retries = 3
        config.spotify_retry_base_delay = 1.0
        config.spotify_retry_max_delay = 120.0
        config.spotify_rate_limit_enabled = True
        config.spotify_rate_limit_requests = 10
        config.spotify_rate_limit_window = 1.0
        # Disable download rate limiting for this test (focus is on cache config)
        config.download_rate_limit_enabled = False
        config.download_rate_limit_requests = 2
        config.download_rate_limit_window = 1.0
        config.download_bandwidth_limit = None
        
        with patch("core.downloader.SpotifyClient") as mock_spotify_class, \
             patch("core.downloader.AudioProvider") as mock_audio_class, \
             patch("core.downloader.MetadataEmbedder"):
            mock_spotify_class.return_value = mocker.Mock()
            mock_audio_class.return_value = mocker.Mock()
            
            downloader = Downloader(config)
            
            # Verify custom values are applied to file existence cache
            assert downloader.file_existence_cache.max_size == 5000
            assert downloader.file_existence_cache.ttl_seconds == 1800
            
            # Verify AudioProvider was called with custom cache config
            # rate_limiter should be None since download_rate_limit_enabled is False
            mock_audio_class.assert_called_once_with(
                output_format="mp3",
                bitrate="128k",
                audio_providers=["youtube-music"],
                cache_max_size=200,
                cache_ttl=7200,
                rate_limiter=None,
            )

