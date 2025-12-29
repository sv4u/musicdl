"""
Unit tests for AudioProvider with mocked dependencies.
"""
import pytest
from pathlib import Path
from unittest.mock import Mock, patch, MagicMock

from core.audio_provider import AudioProvider
from core.exceptions import DownloadError


class TestAudioProvider:
    """Test AudioProvider with mocked yt-dlp."""
    
    @pytest.fixture
    def audio_provider(self):
        """Create AudioProvider instance."""
        return AudioProvider(
            output_format="mp3",
            bitrate="128k",
            audio_providers=["youtube-music"],
        )
    
    def test_search_success(self, audio_provider, mocker):
        """Test successful search."""
        mock_info = {
            "entries": [
                {
                    "url": "https://www.youtube.com/watch?v=test123",
                    "webpage_url": "https://www.youtube.com/watch?v=test123",
                }
            ]
        }
        
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.return_value = mock_info
            mock_ydl_class.return_value = mock_ydl
            
            result = audio_provider.search("Test Artist - Test Song")
            assert result == "https://www.youtube.com/watch?v=test123"
    
    def test_search_no_results(self, audio_provider, mocker):
        """Test search with no results."""
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.return_value = None
            mock_ydl_class.return_value = mock_ydl
            
            result = audio_provider.search("Nonexistent Song")
            assert result is None
    
    def test_search_provider_fallback(self, audio_provider, mocker):
        """Test provider fallback when first provider fails."""
        # First provider fails, second succeeds
        call_count = 0
        def mock_extract_info(query, download=False):
            nonlocal call_count
            call_count += 1
            if call_count == 1:
                raise Exception("Provider 1 failed")
            return {
                "entries": [{"url": "https://www.youtube.com/watch?v=test123"}]
            }
        
        audio_provider.audio_providers = ["youtube-music", "youtube"]
        
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.side_effect = mock_extract_info
            mock_ydl_class.return_value = mock_ydl
            
            result = audio_provider.search("Test Song")
            assert result == "https://www.youtube.com/watch?v=test123"
    
    def test_download_success(self, audio_provider, tmp_test_dir, mocker):
        """Test successful download."""
        output_path = tmp_test_dir / "test.mp3"
        
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.download.return_value = None
            mock_ydl_class.return_value = mock_ydl
            
            # Create the file to simulate successful download
            output_path.write_bytes(b"fake audio content")
            
            result = audio_provider.download("https://www.youtube.com/watch?v=test123", output_path)
            assert result == output_path
            assert result.exists()
    
    def test_download_failure(self, audio_provider, tmp_test_dir, mocker):
        """Test download failure."""
        output_path = tmp_test_dir / "test.mp3"
        
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.download.side_effect = Exception("Download failed")
            mock_ydl_class.return_value = mock_ydl
            
            with pytest.raises(DownloadError, match="Failed to download"):
                audio_provider.download("https://www.youtube.com/watch?v=test123", output_path)
    
    def test_get_metadata(self, audio_provider, mocker):
        """Test getting metadata."""
        mock_info = {
            "title": "Test Song",
            "duration": 180,
            "artist": "Test Artist",
        }
        
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.return_value = mock_info
            mock_ydl_class.return_value = mock_ydl
            
            result = audio_provider.get_metadata("https://www.youtube.com/watch?v=test123")
            assert result["title"] == "Test Song"
            assert result["duration"] == 180
    
    def test_get_metadata_failure(self, audio_provider, mocker):
        """Test metadata extraction failure."""
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.side_effect = Exception("Metadata failed")
            mock_ydl_class.return_value = mock_ydl
            
            result = audio_provider.get_metadata("https://www.youtube.com/watch?v=test123")
            assert result == {}

