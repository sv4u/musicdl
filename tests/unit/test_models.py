"""
Unit tests for data models.
"""
import pytest
from pathlib import Path

from core.models import Song, DownloadResult


class TestSong:
    """Test Song model."""
    
    def test_song_creation(self):
        """Test creating a Song with all fields."""
        song = Song(
            title="Test Song",
            artist="Test Artist",
            album="Test Album",
            track_number=1,
            duration=180,
            spotify_url="https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi",
            cover_url="https://example.com/cover.jpg",
            album_artist="Test Artist",
            year=2023,
            date="2023-01-01",
            disc_number=1,
            disc_count=1,
            tracks_count=10,
            genre="Rock",
            explicit=False,
            isrc="USRC12345678",
        )
        assert song.title == "Test Song"
        assert song.artist == "Test Artist"
        assert song.album == "Test Album"
        assert song.track_number == 1
        assert song.duration == 180
        assert song.cover_url == "https://example.com/cover.jpg"
        assert song.year == 2023
        assert song.explicit is False
    
    def test_song_minimal(self):
        """Test creating a Song with only required fields."""
        song = Song(
            title="Test Song",
            artist="Test Artist",
            album="Test Album",
            track_number=1,
            duration=180,
            spotify_url="https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi",
        )
        assert song.title == "Test Song"
        assert song.cover_url is None
        assert song.year is None
        assert song.disc_number == 1  # Default
        assert song.explicit is False  # Default


class TestDownloadResult:
    """Test DownloadResult model."""
    
    def test_download_result_success(self):
        """Test successful download result."""
        file_path = Path("/tmp/test.mp3")
        result = DownloadResult(
            success=True,
            file_path=file_path,
            song=Song(
                title="Test",
                artist="Test",
                album="Test",
                track_number=1,
                duration=180,
                spotify_url="https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi",
            ),
        )
        assert result.success is True
        assert result.file_path == file_path
        assert result.error is None
        assert result.song is not None
    
    def test_download_result_failure(self):
        """Test failed download result."""
        result = DownloadResult(
            success=False,
            error="Download failed",
        )
        assert result.success is False
        assert result.file_path is None
        assert result.error == "Download failed"
        assert result.song is None

