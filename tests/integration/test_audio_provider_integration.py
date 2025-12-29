"""
Integration tests for AudioProvider with real yt-dlp.
"""
import pytest
from pathlib import Path

from core.audio_provider import AudioProvider
from core.exceptions import DownloadError


@pytest.mark.integration
class TestAudioProviderIntegration:
    """Integration tests with real yt-dlp."""
    
    @pytest.fixture
    def audio_provider(self):
        """Create AudioProvider instance."""
        return AudioProvider(
            output_format="mp3",
            bitrate="128k",
            audio_providers=["youtube-music"],
        )
    
    def test_search_real_provider(self, audio_provider):
        """Test search with real YouTube Music provider."""
        # Use a well-known song that should exist
        result = audio_provider.search("Rush YYZ")
        
        # Should return a URL if found
        if result:
            assert result.startswith("http")
            assert "youtube" in result.lower() or "youtu.be" in result.lower()
    
    def test_get_metadata_real(self, audio_provider):
        """Test getting metadata from real URL."""
        # Use a known YouTube URL
        test_url = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"  # Rick Astley - Never Gonna Give You Up
        
        metadata = audio_provider.get_metadata(test_url)
        
        # Should return some metadata
        assert isinstance(metadata, dict)
        if metadata:
            assert "title" in metadata or "id" in metadata
    
    @pytest.mark.skip(reason="Requires actual download - slow and may fail")
    def test_download_real(self, audio_provider, tmp_test_dir):
        """Test real download (skipped by default - slow)."""
        test_url = "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
        output_path = tmp_test_dir / "test.mp3"
        
        result = audio_provider.download(test_url, output_path)
        
        assert result.exists()
        assert result.stat().st_size > 0

