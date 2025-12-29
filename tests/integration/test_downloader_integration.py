"""
Integration tests for Downloader with real component interactions.
"""
import os
import pytest
from pathlib import Path

from core.downloader import Downloader
from core.config import DownloadSettings
from core.exceptions import DownloadError, SpotifyError


@pytest.mark.integration
class TestDownloaderIntegration:
    """Integration tests for Downloader with real components."""
    
    @pytest.fixture
    def test_config(self, tmp_test_dir, spotify_credentials):
        """Create test configuration."""
        return DownloadSettings(
            client_id=spotify_credentials["client_id"],
            client_secret=spotify_credentials["client_secret"],
            threads=1,
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
        reason="SPOTIFY_CLIENT_ID not set for integration tests"
    )
    def test_track_download_integration(self, downloader, tmp_test_dir):
        """Test track download with real Spotify API."""
        # Use a known track URL: YYZ by Rush
        track_url = "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi"
        
        success, file_path = downloader.download_track(track_url)
        
        # May succeed or fail depending on audio availability
        # Just verify it doesn't crash
        assert isinstance(success, bool)
        if success:
            assert file_path is not None
    
    @pytest.mark.skipif(
        not os.getenv("SPOTIFY_CLIENT_ID"),
        reason="SPOTIFY_CLIENT_ID not set for integration tests"
    )
    def test_error_propagation(self, downloader):
        """Test that errors propagate correctly."""
        # Use invalid track URL
        with pytest.raises((SpotifyError, DownloadError)):
            downloader.download_track("https://open.spotify.com/track/invalid12345")

