"""
Shared pytest fixtures for musicdl tests.
"""
import os
import tempfile
from pathlib import Path
from typing import Dict, Any
from unittest.mock import Mock, MagicMock

import pytest

from core.config import DownloadSettings, MusicDLConfig, MusicSource
from core.spotify_client import SpotifyClient
from core.audio_provider import AudioProvider
from core.metadata import MetadataEmbedder
from core.models import Song


# Sample Spotify API Response Data
# Using real Spotify tracks: YYZ by Rush and Crawling by Linkin Park
SAMPLE_TRACK_DATA = {
    "id": "1RKbVxcm267VdsIzqY7msi",
    "name": "YYZ",
    "artists": [{"name": "Rush"}],
    "album": {
        "id": "77CZUF57sYqgtznUe3OikQ",
        "name": "Moving Pictures (40th Anniversary Super Deluxe)",
        "images": [
            {"url": "https://example.com/cover.jpg", "width": 640, "height": 640}
        ],
        "release_date": "2022-01-01",
        "artists": [{"name": "Rush"}],
        "total_tracks": 10,
    },
    "duration_ms": 266000,  # 4:26
    "track_number": 3,
    "disc_number": 1,
    "external_urls": {"spotify": "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi"},
    "external_ids": {"isrc": "USRC12345678"},
    "explicit": False,
}

SAMPLE_TRACK_DATA_2 = {
    "id": "1BfzeCKzo8xSvJcYLmnP8f",
    "name": "Crawling",
    "artists": [{"name": "Linkin Park"}],
    "album": {
        "id": "album123",
        "name": "Hybrid Theory (20th Anniversary Edition)",
        "images": [
            {"url": "https://example.com/cover.jpg", "width": 640, "height": 640}
        ],
        "release_date": "2020-01-01",
        "artists": [{"name": "Linkin Park"}],
        "total_tracks": 15,
    },
    "duration_ms": 208000,  # 3:28
    "track_number": 5,
    "disc_number": 1,
    "external_urls": {"spotify": "https://open.spotify.com/track/1BfzeCKzo8xSvJcYLmnP8f"},
    "external_ids": {"isrc": "USRC12345679"},
    "explicit": False,
}

SAMPLE_ALBUM_DATA = {
    "id": "77CZUF57sYqgtznUe3OikQ",
    "name": "I Love My Computer",
    "artists": [{"name": "Ninajirachi"}],
    "images": [
        {"url": "https://example.com/cover.jpg", "width": 640, "height": 640}
    ],
    "release_date": "2025-01-01",
    "total_tracks": 12,
    "tracks": {
        "items": [
            {
                "id": "track1",
                "name": "London Song",
                "track_number": 1,
                "disc_number": 1,
                "duration_ms": 180000,
            },
            {
                "id": "track2",
                "name": "iPod Touch",
                "track_number": 2,
                "disc_number": 1,
                "duration_ms": 200000,
            },
        ],
        "next": None,
    },
}

SAMPLE_PLAYLIST_DATA = {
    "id": "5Xrt7Y1mwD4q107Ty56xnn",
    "name": "planet namek",
    "tracks": {
        "items": [
            {
                "track": {
                    "id": "track1",
                    "name": "stagekiss",
                    "external_urls": {"spotify": "https://open.spotify.com/track/track1"},
                    "is_local": False,
                }
            }
        ],
        "next": None,
    },
}

SAMPLE_ARTIST_ALBUMS_DATA = [
    {
        "id": "77CZUF57sYqgtznUe3OikQ",
        "name": "I Love My Computer",
        "external_urls": {"spotify": "https://open.spotify.com/album/77CZUF57sYqgtznUe3OikQ"},
    },
    {
        "id": "album2",
        "name": "BELIEVE IN ME, WHO BELIEVES IN YOU",
        "external_urls": {"spotify": "https://open.spotify.com/album/album2"},
    },
]


@pytest.fixture
def tmp_test_dir():
    """Create temporary directory for tests."""
    with tempfile.TemporaryDirectory() as tmpdir:
        yield Path(tmpdir)


@pytest.fixture
def sample_download_settings():
    """Create sample download settings."""
    return DownloadSettings(
        client_id="85d6e012bea84598ac13d3ce963a04b2",
        client_secret="1a0c452389fd4147905d753a31d1b456",
        threads=2,
        max_retries=2,
        format="mp3",
        bitrate="128k",
        output="{artist}/{album}/{track-number} - {title}.{output-ext}",
        audio_providers=["youtube-music"],
        cache_max_size=100,
        cache_ttl=3600,
        overwrite="skip",
    )


@pytest.fixture
def sample_config(sample_download_settings):
    """Create sample MusicDLConfig."""
    return MusicDLConfig(
        version="1.2",
        download=sample_download_settings,
        songs=[
            MusicSource(name="YYZ", url="https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi"),
            MusicSource(name="Crawling", url="https://open.spotify.com/track/1BfzeCKzo8xSvJcYLmnP8f")
        ],
        artists=[],
        playlists=[],
    )


@pytest.fixture
def mock_spotify_client(mocker):
    """Create mock Spotify client."""
    client = mocker.Mock(spec=SpotifyClient)
    client.get_track.return_value = SAMPLE_TRACK_DATA
    client.get_album.return_value = SAMPLE_ALBUM_DATA
    client.get_playlist.return_value = SAMPLE_PLAYLIST_DATA
    client.get_artist_albums.return_value = SAMPLE_ARTIST_ALBUMS_DATA
    client.client = mocker.Mock()
    client.cache = mocker.Mock()
    return client


@pytest.fixture
def mock_audio_provider(mocker):
    """Create mock audio provider."""
    provider = mocker.Mock(spec=AudioProvider)
    provider.search.return_value = "https://www.youtube.com/watch?v=test123"
    provider.download.return_value = Path("/tmp/test.mp3")
    provider.get_metadata.return_value = {"title": "Test Song", "duration": 180}
    return provider


@pytest.fixture
def mock_metadata_embedder(mocker):
    """Create mock metadata embedder."""
    embedder = mocker.Mock(spec=MetadataEmbedder)
    embedder.embed.return_value = None
    return embedder


@pytest.fixture
def sample_song():
    """Create sample Song object."""
    return Song(
        title="YYZ",
        artist="Rush",
        album="Moving Pictures (40th Anniversary Super Deluxe)",
        track_number=3,
        duration=266,
        spotify_url="https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi",
        cover_url="https://example.com/cover.jpg",
        album_artist="Rush",
        year=2022,
        date="2022-01-01",
        disc_number=1,
        disc_count=1,
        tracks_count=10,
    )


@pytest.fixture
def spotify_credentials():
    """Get Spotify credentials from environment or use test values."""
    return {
        "client_id": os.getenv("SPOTIFY_CLIENT_ID", "85d6e012bea84598ac13d3ce963a04b2"),
        "client_secret": os.getenv("SPOTIFY_CLIENT_SECRET", "1a0c452389fd4147905d753a31d1b456"),
    }


@pytest.fixture
def sample_audio_file(tmp_test_dir, sample_song):
    """Create a sample audio file for testing."""
    audio_file = tmp_test_dir / f"{sample_song.title}.mp3"
    audio_file.write_bytes(b"fake mp3 content")
    return audio_file


@pytest.fixture
def sample_config_yaml(tmp_test_dir):
    """Create sample config YAML file."""
    config_file = tmp_test_dir / "config.yaml"
    config_file.write_text("""
version: 1.2
download:
  client_id: 85d6e012bea84598ac13d3ce963a04b2
  client_secret: 1a0c452389fd4147905d753a31d1b456
  threads: 2
  max_retries: 2
  format: mp3
  bitrate: 128k
  output: "{artist}/{album}/{track-number} - {title}.{output-ext}"
  audio_providers: ["youtube-music"]
  cache_max_size: 100
  cache_ttl: 3600
  overwrite: skip
songs:
  - YYZ: https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi
  - Crawling: https://open.spotify.com/track/1BfzeCKzo8xSvJcYLmnP8f
artists: []
playlists: []
""")
    return str(config_file)

