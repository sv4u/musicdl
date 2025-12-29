"""
Test helper functions and utilities.
"""
import json
from pathlib import Path
from typing import Dict, Any, List
from unittest.mock import Mock, MagicMock

from core.models import Song


def create_mock_spotify_response(response_type: str, **kwargs) -> Dict[str, Any]:
    """
    Create mock Spotify API response.
    
    Args:
        response_type: Type of response (track, album, playlist, artist_albums)
        **kwargs: Override values for the response
    
    Returns:
        Mock response dictionary
    """
    base_responses = {
        "track": {
            "id": "1RKbVxcm267VdsIzqY7msi",
            "name": "YYZ",
            "artists": [{"name": "Rush"}],
            "album": {"id": "77CZUF57sYqgtznUe3OikQ", "name": "Moving Pictures (40th Anniversary Super Deluxe)"},
            "duration_ms": 266000,
            "track_number": 3,
            "disc_number": 1,
            "external_urls": {"spotify": "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi"},
        },
        "album": {
            "id": "77CZUF57sYqgtznUe3OikQ",
            "name": "I Love My Computer",
            "artists": [{"name": "Ninajirachi"}],
            "total_tracks": 12,
            "tracks": {"items": [], "next": None},
        },
        "playlist": {
            "id": "5Xrt7Y1mwD4q107Ty56xnn",
            "name": "planet namek",
            "tracks": {"items": [], "next": None},
        },
        "artist_albums": [],
    }
    
    response = base_responses.get(response_type, {}).copy()
    response.update(kwargs)
    return response


def create_sample_song(**kwargs) -> Song:
    """Create sample Song with optional overrides."""
    defaults = {
        "title": "YYZ",
        "artist": "Rush",
        "album": "Moving Pictures (40th Anniversary Super Deluxe)",
        "track_number": 3,
        "duration": 266,
        "spotify_url": "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi",
    }
    defaults.update(kwargs)
    return Song(**defaults)


def assert_song_metadata(file_path: Path, expected_song: Song):
    """
    Assert that audio file has correct metadata.
    
    Args:
        file_path: Path to audio file
        expected_song: Expected song metadata
    """
    from mutagen import File
    
    audio_file = File(str(file_path))
    assert audio_file is not None
    
    # Verify basic tags based on format
    if file_path.suffix.lower() == ".mp3":
        assert audio_file.get("TIT2")[0].text[0] == expected_song.title
        assert audio_file.get("TPE1")[0].text[0] == expected_song.artist
    elif file_path.suffix.lower() in [".flac", ".ogg", ".opus"]:
        assert audio_file.get("title")[0] == expected_song.title
        assert audio_file.get("artist")[0] == expected_song.artist
    elif file_path.suffix.lower() == ".m4a":
        assert audio_file.get("\xa9nam")[0] == expected_song.title
        assert audio_file.get("\xa9ART")[0] == expected_song.artist


def create_test_audio_file(path: Path, format: str = "mp3") -> Path:
    """
    Create a minimal test audio file.
    
    Args:
        path: Path where file should be created
        format: Audio format (mp3, flac, m4a)
    
    Returns:
        Path to created file
    """
    # Create minimal valid audio file content
    # In real tests, use actual audio file bytes or generate minimal valid files
    test_content = b"fake audio content"
    file_path = path.with_suffix(f".{format}")
    file_path.write_bytes(test_content)
    return file_path


def verify_file_structure(output_dir: Path, expected_files: List[str]):
    """
    Verify that expected files exist in output directory.
    
    Args:
        output_dir: Output directory to check
        expected_files: List of expected file paths (relative to output_dir)
    """
    for file_path in expected_files:
        full_path = output_dir / file_path
        assert full_path.exists(), f"Expected file not found: {file_path}"
        assert full_path.is_file(), f"Expected path is not a file: {file_path}"


def extract_spotify_id(url_or_id: str) -> str:
    """Extract Spotify ID from URL or return ID as-is."""
    import re
    match = re.search(r"spotify\.com/\w+/([a-zA-Z0-9]+)", url_or_id)
    return match.group(1) if match else url_or_id

