"""
Unit tests for MetadataEmbedder with mocked dependencies.
"""
import pytest
from pathlib import Path
from unittest.mock import Mock, patch, MagicMock

from core.metadata import MetadataEmbedder
from core.exceptions import MetadataError
from core.models import Song


class TestMetadataEmbedder:
    """Test MetadataEmbedder with mocked mutagen."""
    
    @pytest.fixture
    def metadata_embedder(self):
        """Create MetadataEmbedder instance."""
        return MetadataEmbedder()
    
    @pytest.fixture
    def sample_song(self):
        """Create sample song."""
        return Song(
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
        )
    
    def test_embed_mp3(self, metadata_embedder, sample_song, tmp_test_dir, mocker):
        """Test embedding metadata in MP3 file."""
        audio_file = tmp_test_dir / "test.mp3"
        audio_file.write_bytes(b"fake mp3 content")
        
        with patch("core.metadata.ID3") as mock_id3_class, \
             patch("core.metadata.requests.get") as mock_get:
            mock_id3 = MagicMock()
            mock_id3_class.return_value = mock_id3
            mock_get.return_value.content = b"fake cover art"
            
            metadata_embedder.embed(audio_file, sample_song)
            
            # Verify ID3 tags were set
            assert mock_id3.__setitem__.called
            assert mock_id3.save.called
    
    def test_embed_flac(self, metadata_embedder, sample_song, tmp_test_dir, mocker):
        """Test embedding metadata in FLAC file."""
        audio_file = tmp_test_dir / "test.flac"
        audio_file.write_bytes(b"fake flac content")
        
        with patch("core.metadata.File") as mock_file_class, \
             patch("core.metadata.requests.get") as mock_get:
            mock_file = MagicMock()
            mock_file_class.return_value = mock_file
            mock_get.return_value.content = b"fake cover art"
            
            metadata_embedder.embed(audio_file, sample_song)
            
            # Verify tags were set
            assert mock_file.__setitem__.called
            assert mock_file.save.called
    
    def test_embed_m4a(self, metadata_embedder, sample_song, tmp_test_dir, mocker):
        """Test embedding metadata in M4A file."""
        audio_file = tmp_test_dir / "test.m4a"
        audio_file.write_bytes(b"fake m4a content")
        
        with patch("core.metadata.File") as mock_file_class, \
             patch("core.metadata.requests.get") as mock_get:
            mock_file = MagicMock()
            mock_file_class.return_value = mock_file
            mock_get.return_value.content = b"fake cover art"
            
            metadata_embedder.embed(audio_file, sample_song)
            
            # Verify tags were set
            assert mock_file.__setitem__.called
            assert mock_file.save.called
    
    def test_embed_file_not_found(self, metadata_embedder, sample_song, tmp_test_dir):
        """Test embedding metadata when file doesn't exist."""
        audio_file = tmp_test_dir / "nonexistent.mp3"
        
        with pytest.raises(MetadataError, match="File not found"):
            metadata_embedder.embed(audio_file, sample_song)
    
    def test_embed_missing_optional_fields(self, metadata_embedder, tmp_test_dir, mocker):
        """Test embedding with missing optional fields."""
        audio_file = tmp_test_dir / "test.mp3"
        audio_file.write_bytes(b"fake mp3 content")
        
        song = Song(
            title="Test Song",
            artist="Test Artist",
            album="",  # Empty album
            track_number=1,
            duration=180,
            spotify_url="https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi",
            year=None,  # No year
        )
        
        with patch("core.metadata.ID3") as mock_id3_class:
            mock_id3 = MagicMock()
            mock_id3_class.return_value = mock_id3
            
            metadata_embedder.embed(audio_file, song)
            
            # Should not raise, should handle missing fields gracefully
            assert mock_id3.save.called
    
    def test_embed_cover_art_failure(self, metadata_embedder, sample_song, tmp_test_dir, mocker):
        """Test that cover art failure doesn't break embedding."""
        audio_file = tmp_test_dir / "test.mp3"
        audio_file.write_bytes(b"fake mp3 content")
        
        with patch("core.metadata.ID3") as mock_id3_class, \
             patch("core.metadata.requests.get") as mock_get:
            mock_id3 = MagicMock()
            mock_id3_class.return_value = mock_id3
            mock_get.side_effect = Exception("Network error")
            
            # Should not raise, should log warning
            metadata_embedder.embed(audio_file, sample_song)
            assert mock_id3.save.called

