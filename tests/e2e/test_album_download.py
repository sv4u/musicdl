"""
End-to-end tests for full album download workflows.
"""
import os
import pytest
from core.downloader import Downloader
from core.config import DownloadSettings


@pytest.mark.e2e
class TestAlbumDownloadE2E:
    """End-to-end tests for album downloads."""
    
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
    def test_complete_album_download(self, downloader, tmp_test_dir):
        """Test downloading complete album."""
        album_url = "https://open.spotify.com/album/77CZUF57sYqgtznUe3OikQ"  # I Love My Computer
        
        results = downloader.download_album(album_url)
        
        # Should download multiple tracks
        assert len(results) > 0
        # Verify all succeeded
        assert all(success for success, _ in results)
        assert all(path.exists() for _, path in results if path)
    
    @pytest.mark.skipif(
        not os.getenv("SPOTIFY_CLIENT_ID"),
        reason="SPOTIFY_CLIENT_ID not set for E2E tests"
    )
    def test_album_metadata_consistency(self, downloader, tmp_test_dir):
        """Test that all tracks in album have consistent metadata."""
        album_url = "https://open.spotify.com/album/77CZUF57sYqgtznUe3OikQ"
        
        results = downloader.download_album(album_url)
        
        # Verify all tracks have same album name in path
        if results:
            first_path = results[0][1]
            if first_path:
                album_name = first_path.parent.name
                for success, path in results:
                    if success and path:
                        assert path.parent.name == album_name

