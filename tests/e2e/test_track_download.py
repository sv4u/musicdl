"""
End-to-end tests for full track download workflows.
Requires full environment setup with credentials and network access.
"""
import os
import pytest
from pathlib import Path
from core.downloader import Downloader
from core.config import DownloadSettings, load_config
from tests.helpers import assert_song_metadata, verify_file_structure


@pytest.mark.e2e
class TestTrackDownloadE2E:
    """End-to-end tests for track downloads."""
    
    @pytest.fixture
    def test_config(self, tmp_test_dir, spotify_credentials):
        """Create test configuration."""
        return DownloadSettings(
            client_id=spotify_credentials["client_id"],
            client_secret=spotify_credentials["client_secret"],
            threads=1,  # Sequential for E2E tests
            max_retries=2,
            format="mp3",
            bitrate="128k",
            output=str(tmp_test_dir / "{artist}" / "{album}" / "{track-number} - {title}.{output-ext}"),
            audio_providers=["youtube-music"],
            cache_max_size=100,
            cache_ttl=3600,
            overwrite="skip",
        )
    
    @pytest.fixture
    def downloader(self, test_config):
        """Create downloader instance."""
        return Downloader(test_config)
    
    @pytest.mark.skipif(
        not os.getenv("SPOTIFY_CLIENT_ID"),
        reason="SPOTIFY_CLIENT_ID not set for E2E tests"
    )
    def test_single_track_download(self, downloader, tmp_test_dir):
        """Test downloading a single track end-to-end."""
        # Use a known track URL: YYZ by Rush
        track_url = "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi"
        
        success, file_path = downloader.download_track(track_url)
        
        assert success is True
        assert file_path is not None
        assert file_path.exists()
        assert file_path.suffix == ".mp3"
        
        # Verify file is not empty
        assert file_path.stat().st_size > 0
        
        # Verify metadata (if helper available)
        # Note: This requires actual audio file with metadata
        # assert_song_metadata(file_path, expected_song)
    
    @pytest.mark.skipif(
        not os.getenv("SPOTIFY_CLIENT_ID"),
        reason="SPOTIFY_CLIENT_ID not set for E2E tests"
    )
    def test_multiple_tracks_batch(self, downloader, tmp_test_dir):
        """Test downloading multiple tracks in batch."""
        track_urls = [
            "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi",  # YYZ by Rush
            "https://open.spotify.com/track/1BfzeCKzo8xSvJcYLmnP8f",  # Crawling by Linkin Park
        ]
        
        results = []
        for url in track_urls:
            success, path = downloader.download_track(url)
            results.append((success, path))
        
        # Verify all downloads succeeded
        assert all(success for success, _ in results)
        assert all(path.exists() for _, path in results if path)
        
        # Verify file organization
        verify_file_structure(tmp_test_dir, [
            # Expected file paths based on template
        ])
    
    @pytest.mark.skipif(
        not os.getenv("SPOTIFY_CLIENT_ID"),
        reason="SPOTIFY_CLIENT_ID not set for E2E tests"
    )
    def test_track_download_file_organization(self, downloader, tmp_test_dir):
        """Test that files are organized according to template."""
        track_url = "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi"  # YYZ by Rush
        
        success, file_path = downloader.download_track(track_url)
        
        assert success is True
        # Verify path matches template structure
        # Template: {artist}/{album}/{track-number} - {title}.{output-ext}
        assert file_path.parent.parent.name  # Artist directory
        assert file_path.parent.name  # Album directory
        assert file_path.name.startswith("03 -")  # Track number and title
    
    @pytest.mark.skipif(
        not os.getenv("SPOTIFY_CLIENT_ID"),
        reason="SPOTIFY_CLIENT_ID not set for E2E tests"
    )
    def test_track_download_error_recovery(self, downloader):
        """Test error recovery in batch downloads."""
        # Mix valid and invalid URLs
        track_urls = [
            "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi",  # Valid: YYZ by Rush
            "https://open.spotify.com/track/invalid12345",  # Invalid
            "https://open.spotify.com/track/1BfzeCKzo8xSvJcYLmnP8f",  # Valid: Crawling by Linkin Park
        ]
        
        results = []
        for url in track_urls:
            success, path = downloader.download_track(url)
            results.append((success, path))
        
        # First and third should succeed, second should fail
        assert results[0][0] is True
        assert results[1][0] is False  # Invalid track
        assert results[2][0] is True

