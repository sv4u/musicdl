"""
Unit tests for AudioProvider with mocked dependencies.
"""
import time
import pytest
from pathlib import Path
from unittest.mock import Mock, patch, MagicMock

from core.audio_provider import AudioProvider, _CACHE_NOT_FOUND
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

    def test_search_caching_hit(self, audio_provider, mocker):
        """Test that cached search results are returned without actual search."""
        mock_info = {
            "entries": [
                {
                    "url": "https://www.youtube.com/watch?v=cached123",
                    "webpage_url": "https://www.youtube.com/watch?v=cached123",
                }
            ]
        }
        
        # First search - should perform actual search
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.return_value = mock_info
            mock_ydl_class.return_value = mock_ydl
            
            result1 = audio_provider.search("Test Artist - Test Song")
            assert result1 == "https://www.youtube.com/watch?v=cached123"
            assert mock_ydl.extract_info.call_count == 1
        
        # Second search with same query - should use cache
        result2 = audio_provider.search("Test Artist - Test Song")
        assert result2 == "https://www.youtube.com/watch?v=cached123"
        # Should not have called extract_info again
        assert mock_ydl.extract_info.call_count == 1

    def test_search_caching_miss(self, audio_provider, mocker):
        """Test that cache miss triggers actual search."""
        mock_info = {
            "entries": [
                {
                    "url": "https://www.youtube.com/watch?v=new123",
                    "webpage_url": "https://www.youtube.com/watch?v=new123",
                }
            ]
        }
        
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.return_value = mock_info
            mock_ydl_class.return_value = mock_ydl
            
            # First search
            result1 = audio_provider.search("New Song")
            assert result1 == "https://www.youtube.com/watch?v=new123"
            
            # Different query - should perform new search
            result2 = audio_provider.search("Different Song")
            assert result2 == "https://www.youtube.com/watch?v=new123"
            assert mock_ydl.extract_info.call_count == 2

    def test_search_caching_failed_search(self, audio_provider, mocker):
        """Test that failed searches are cached."""
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.return_value = None
            mock_ydl_class.return_value = mock_ydl
            
            # First search - should return None
            result1 = audio_provider.search("Nonexistent Song")
            assert result1 is None
            assert mock_ydl.extract_info.call_count == 1
        
        # Second search - should use cache (return None without searching)
        result2 = audio_provider.search("Nonexistent Song")
        assert result2 is None
        # Should not have called extract_info again
        assert mock_ydl.extract_info.call_count == 1

    def test_search_caching_query_normalization(self, audio_provider, mocker):
        """Test that cache keys are normalized (case-insensitive, whitespace)."""
        mock_info = {
            "entries": [
                {
                    "url": "https://www.youtube.com/watch?v=normalized123",
                    "webpage_url": "https://www.youtube.com/watch?v=normalized123",
                }
            ]
        }
        
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.return_value = mock_info
            mock_ydl_class.return_value = mock_ydl
            
            # First search with one format
            result1 = audio_provider.search("Test Artist - Test Song")
            assert result1 == "https://www.youtube.com/watch?v=normalized123"
            assert mock_ydl.extract_info.call_count == 1
        
        # Second search with different case/whitespace - should use cache
        result2 = audio_provider.search("  TEST ARTIST - TEST SONG  ")
        assert result2 == "https://www.youtube.com/watch?v=normalized123"
        # Should not have called extract_info again
        assert mock_ydl.extract_info.call_count == 1

    def test_search_caching_ttl_expiration(self, mocker):
        """Test that cache respects TTL expiration."""
        audio_provider = AudioProvider(
            output_format="mp3",
            bitrate="128k",
            audio_providers=["youtube-music"],
            cache_ttl=1,  # 1 second TTL
        )
        
        mock_info = {
            "entries": [
                {
                    "url": "https://www.youtube.com/watch?v=ttl123",
                    "webpage_url": "https://www.youtube.com/watch?v=ttl123",
                }
            ]
        }
        
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.return_value = mock_info
            mock_ydl_class.return_value = mock_ydl
            
            # First search
            result1 = audio_provider.search("TTL Test")
            assert result1 == "https://www.youtube.com/watch?v=ttl123"
            assert mock_ydl.extract_info.call_count == 1
        
        # Wait for TTL expiration
        time.sleep(1.1)
        
        # Second search - should perform new search after expiration
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.return_value = mock_info
            mock_ydl_class.return_value = mock_ydl
            
            result2 = audio_provider.search("TTL Test")
            assert result2 == "https://www.youtube.com/watch?v=ttl123"
            # Should have called extract_info again
            assert mock_ydl.extract_info.call_count == 1

    def test_search_cache_statistics(self, audio_provider, mocker):
        """Test that cache statistics are tracked."""
        mock_info = {
            "entries": [
                {
                    "url": "https://www.youtube.com/watch?v=stats123",
                    "webpage_url": "https://www.youtube.com/watch?v=stats123",
                }
            ]
        }
        
        with patch("core.audio_provider.yt_dlp.YoutubeDL") as mock_ydl_class:
            mock_ydl = MagicMock()
            mock_ydl.__enter__.return_value = mock_ydl
            mock_ydl.__exit__.return_value = None
            mock_ydl.extract_info.return_value = mock_info
            mock_ydl_class.return_value = mock_ydl
            
            # First search - cache miss
            audio_provider.search("Stats Test")
            
            # Second search - cache hit
            audio_provider.search("Stats Test")
            
            # Check cache statistics
            stats = audio_provider.search_cache.stats()
            assert stats["hits"] == 1
            assert stats["misses"] == 1
            assert stats["hit_rate"] == "50.00%"

    def test_search_cache_custom_config(self):
        """Test AudioProvider with custom cache configuration."""
        audio_provider = AudioProvider(
            output_format="mp3",
            bitrate="128k",
            cache_max_size=100,
            cache_ttl=7200,  # 2 hours
        )
        
        assert audio_provider.search_cache.max_size == 100
        assert audio_provider.search_cache.ttl_seconds == 7200

