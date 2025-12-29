"""
End-to-end tests for full playlist download workflows.
"""
import os
import pytest
from pathlib import Path
from core.downloader import Downloader
from core.config import DownloadSettings


@pytest.mark.e2e
class TestPlaylistDownloadE2E:
    """End-to-end tests for playlist downloads."""
    
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
        reason="SPOTIFY_CLIENT_ID not set for E2E tests"
    )
    def test_playlist_download(self, downloader, tmp_test_dir):
        """Test downloading playlist."""
        playlist_url = "https://open.spotify.com/playlist/5Xrt7Y1mwD4q107Ty56xnn"  # planet namek
        
        results = downloader.download_playlist(playlist_url, create_m3u=True)
        
        # Should download tracks
        assert len(results) > 0
        
        # Check M3U file was created
        m3u_file = tmp_test_dir / "planet namek.m3u"
        assert m3u_file.exists()
        assert "#EXTM3U" in m3u_file.read_text()

