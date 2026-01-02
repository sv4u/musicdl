"""
Unit tests for PlanGenerator.
"""
import pytest
from unittest.mock import Mock, MagicMock

from core.config import MusicDLConfig, MusicSource, DownloadSettings
from core.plan import DownloadPlan, PlanItemType, PlanItemStatus
from core.plan_generator import PlanGenerator
from core.spotify_client import SpotifyClient


@pytest.fixture
def mock_spotify_client():
    """Create a mocked SpotifyClient."""
    client = Mock(spec=SpotifyClient)
    client.get_track = Mock()
    client.get_artist = Mock()
    client.get_artist_albums = Mock()
    client.get_album = Mock()
    client.get_playlist = Mock()
    client.client = Mock()
    return client


@pytest.fixture
def sample_config():
    """Create a sample MusicDLConfig."""
    download_settings = DownloadSettings(
        client_id="test_id",
        client_secret="test_secret",
    )
    return MusicDLConfig(
        version="1.2",
        download=download_settings,
        songs=[
            MusicSource(name="Test Song", url="https://open.spotify.com/track/123"),
        ],
        artists=[
            MusicSource(name="Test Artist", url="https://open.spotify.com/artist/456"),
        ],
        playlists=[
            MusicSource(name="Test Playlist", url="https://open.spotify.com/playlist/789"),
        ],
    )


class TestPlanGenerator:
    """Test PlanGenerator."""

    def test_generate_plan_with_songs(self, mock_spotify_client):
        """Test generating plan with songs."""
        # Create config with only songs
        config = MusicDLConfig(
            version="1.2",
            download=DownloadSettings(client_id="test", client_secret="test"),
            songs=[
                MusicSource(name="Test Song", url="https://open.spotify.com/track/123"),
            ],
        )

        # Mock track data
        mock_spotify_client.get_track.return_value = {
            "id": "123",
            "name": "Test Track",
            "artists": [{"name": "Test Artist"}],
            "external_urls": {"spotify": "https://open.spotify.com/track/123"},
        }

        generator = PlanGenerator(config, mock_spotify_client)
        plan = generator.generate_plan()

        assert len(plan.items) > 0
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]
        assert len(track_items) == 1
        assert track_items[0].spotify_id == "123"

    def test_generate_plan_with_artists(self, mock_spotify_client):
        """Test generating plan with artists."""
        # Create config with only artists
        config = MusicDLConfig(
            version="1.2",
            download=DownloadSettings(client_id="test", client_secret="test"),
            artists=[
                MusicSource(name="Test Artist", url="https://open.spotify.com/artist/456"),
            ],
        )

        # Mock artist data
        mock_spotify_client.get_artist.return_value = {
            "id": "456",
            "name": "Test Artist",
            "external_urls": {"spotify": "https://open.spotify.com/artist/456"},
        }

        # Mock albums
        mock_spotify_client.get_artist_albums.return_value = [
            {
                "id": "album1",
                "name": "Album 1",
                "album_type": "album",
                "external_urls": {"spotify": "https://open.spotify.com/album/album1"},
            }
        ]

        # Mock album tracks
        mock_spotify_client.get_album.return_value = {
            "id": "album1",
            "name": "Album 1",
            "tracks": {
                "items": [
                    {
                        "id": "track1",
                        "name": "Track 1",
                        "track_number": 1,
                        "disc_number": 1,
                        "external_urls": {"spotify": "https://open.spotify.com/track/track1"},
                    }
                ],
                "next": None,
            },
        }

        generator = PlanGenerator(config, mock_spotify_client)
        plan = generator.generate_plan()

        # Should have artist, album, and track items
        artist_items = [item for item in plan.items if item.item_type == PlanItemType.ARTIST]
        album_items = [item for item in plan.items if item.item_type == PlanItemType.ALBUM]
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]

        assert len(artist_items) == 1
        assert len(album_items) == 1
        assert len(track_items) == 1

    def test_generate_plan_with_playlists(self, mock_spotify_client):
        """Test generating plan with playlists."""
        # Create config with only playlists
        config = MusicDLConfig(
            version="1.2",
            download=DownloadSettings(client_id="test", client_secret="test"),
            playlists=[
                MusicSource(name="Test Playlist", url="https://open.spotify.com/playlist/789"),
            ],
        )

        # Mock playlist data
        mock_spotify_client.get_playlist.return_value = {
            "id": "789",
            "name": "Test Playlist",
            "description": "Test Description",
            "external_urls": {"spotify": "https://open.spotify.com/playlist/789"},
            "tracks": {
                "items": [
                    {
                        "track": {
                            "id": "track1",
                            "name": "Track 1",
                            "is_local": False,
                            "external_urls": {"spotify": "https://open.spotify.com/track/track1"},
                        },
                        "added_at": "2023-01-01T00:00:00Z",
                    }
                ],
                "next": None,
            },
        }

        generator = PlanGenerator(config, mock_spotify_client)
        plan = generator.generate_plan()

        # Should have playlist, track, and M3U items
        playlist_items = [item for item in plan.items if item.item_type == PlanItemType.PLAYLIST]
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]
        m3u_items = [item for item in plan.items if item.item_type == PlanItemType.M3U]

        assert len(playlist_items) == 1
        assert len(track_items) == 1
        assert len(m3u_items) == 1

    def test_generate_plan_with_albums_with_m3u(self, mock_spotify_client):
        """Test generating plan with albums that request M3U creation."""
        # Create config with only albums
        config = MusicDLConfig(
            version="1.2",
            download=DownloadSettings(client_id="test", client_secret="test"),
            albums=[
                MusicSource(
                    name="Test Album",
                    url="https://open.spotify.com/album/album123",
                    create_m3u=True,
                ),
            ],
        )

        # Mock album data
        mock_spotify_client.get_album.return_value = {
            "id": "album123",
            "name": "Test Album",
            "album_type": "album",
            "release_date": "2023-01-01",
            "external_urls": {"spotify": "https://open.spotify.com/album/album123"},
            "tracks": {
                "items": [
                    {
                        "id": "track1",
                        "name": "Track 1",
                        "track_number": 1,
                        "disc_number": 1,
                        "external_urls": {"spotify": "https://open.spotify.com/track/track1"},
                    }
                ],
                "next": None,
            },
        }

        generator = PlanGenerator(config, mock_spotify_client)
        plan = generator.generate_plan()

        # Should have album, track, and M3U items
        album_items = [item for item in plan.items if item.item_type == PlanItemType.ALBUM]
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]
        m3u_items = [item for item in plan.items if item.item_type == PlanItemType.M3U]

        assert len(album_items) == 1
        assert len(track_items) == 1
        assert len(m3u_items) == 1
        # Verify create_m3u is stored in metadata
        assert album_items[0].metadata.get("create_m3u") is True

    def test_generate_plan_with_albums_without_m3u(self, mock_spotify_client):
        """Test generating plan with albums that don't request M3U creation."""
        # Create config with only albums
        config = MusicDLConfig(
            version="1.2",
            download=DownloadSettings(client_id="test", client_secret="test"),
            albums=[
                MusicSource(
                    name="Test Album",
                    url="https://open.spotify.com/album/album123",
                    create_m3u=False,
                ),
            ],
        )

        # Mock album data
        mock_spotify_client.get_album.return_value = {
            "id": "album123",
            "name": "Test Album",
            "album_type": "album",
            "release_date": "2023-01-01",
            "external_urls": {"spotify": "https://open.spotify.com/album/album123"},
            "tracks": {
                "items": [
                    {
                        "id": "track1",
                        "name": "Track 1",
                        "track_number": 1,
                        "disc_number": 1,
                        "external_urls": {"spotify": "https://open.spotify.com/track/track1"},
                    }
                ],
                "next": None,
            },
        }

        generator = PlanGenerator(config, mock_spotify_client)
        plan = generator.generate_plan()

        # Should have album and track items, but NO M3U items
        album_items = [item for item in plan.items if item.item_type == PlanItemType.ALBUM]
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]
        m3u_items = [item for item in plan.items if item.item_type == PlanItemType.M3U]

        assert len(album_items) == 1
        assert len(track_items) == 1
        assert len(m3u_items) == 0  # No M3U when create_m3u is False
        # Verify create_m3u is stored in metadata
        assert album_items[0].metadata.get("create_m3u") is False

    def test_generate_plan_honors_m3u_for_duplicate_album(self, mock_spotify_client):
        """Test that explicit album with create_m3u=True is honored even if album exists from artist."""
        # Create config with artist and explicit album (same album) with create_m3u=True
        config = MusicDLConfig(
            version="1.2",
            download=DownloadSettings(client_id="test", client_secret="test"),
            artists=[
                MusicSource(name="Test Artist", url="https://open.spotify.com/artist/456"),
            ],
            albums=[
                MusicSource(
                    name="Test Album",
                    url="https://open.spotify.com/album/album123",
                    create_m3u=True,
                ),
            ],
        )

        # Mock artist data
        mock_spotify_client.get_artist.return_value = {
            "id": "456",
            "name": "Test Artist",
            "external_urls": {"spotify": "https://open.spotify.com/artist/456"},
        }

        # Mock albums for artist (same album ID as explicit album)
        mock_spotify_client.get_artist_albums.return_value = [
            {
                "id": "album123",
                "name": "Test Album",
                "album_type": "album",
                "external_urls": {"spotify": "https://open.spotify.com/album/album123"},
            }
        ]

        # Mock album data (will be called twice - once for artist, once for explicit)
        mock_spotify_client.get_album.return_value = {
            "id": "album123",
            "name": "Test Album",
            "album_type": "album",
            "release_date": "2023-01-01",
            "external_urls": {"spotify": "https://open.spotify.com/album/album123"},
            "tracks": {
                "items": [
                    {
                        "id": "track1",
                        "name": "Track 1",
                        "track_number": 1,
                        "disc_number": 1,
                        "external_urls": {"spotify": "https://open.spotify.com/track/track1"},
                    }
                ],
                "next": None,
            },
        }

        generator = PlanGenerator(config, mock_spotify_client)
        plan = generator.generate_plan()

        # Should have album, track, and M3U items
        album_items = [item for item in plan.items if item.item_type == PlanItemType.ALBUM]
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]
        m3u_items = [item for item in plan.items if item.item_type == PlanItemType.M3U]

        assert len(album_items) == 1  # Only one album (duplicate was handled)
        assert len(track_items) == 1
        assert len(m3u_items) == 1  # M3U should be created despite duplicate
        # Verify create_m3u is set in metadata
        assert album_items[0].metadata.get("create_m3u") is True
        # Verify M3U item exists and is linked to album
        assert m3u_items[0].parent_id == album_items[0].item_id
        assert m3u_items[0].item_id in album_items[0].child_ids

    def test_generate_plan_removes_duplicates(self, mock_spotify_client):
        """Test that plan generator removes duplicates during generation."""
        config = MusicDLConfig(
            version="1.2",
            download=DownloadSettings(client_id="test", client_secret="test"),
            songs=[
                MusicSource(name="Song 1", url="https://open.spotify.com/track/123"),
                MusicSource(name="Song 2", url="https://open.spotify.com/track/123"),  # Duplicate
            ],
        )

        mock_spotify_client.get_track.return_value = {
            "id": "123",
            "name": "Test Track",
            "artists": [{"name": "Test Artist"}],
            "external_urls": {"spotify": "https://open.spotify.com/track/123"},
        }

        generator = PlanGenerator(config, mock_spotify_client)
        plan = generator.generate_plan()

        # Should only have one track item (duplicate removed)
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]
        assert len(track_items) == 1

    def test_generate_plan_handles_errors(self, mock_spotify_client):
        """Test that plan generator handles errors gracefully."""
        # Create config with only songs
        config = MusicDLConfig(
            version="1.2",
            download=DownloadSettings(client_id="test", client_secret="test"),
            songs=[
                MusicSource(name="Test Song", url="https://open.spotify.com/track/123"),
            ],
        )

        # Make get_track raise an error
        mock_spotify_client.get_track.side_effect = Exception("API Error")

        generator = PlanGenerator(config, mock_spotify_client)
        plan = generator.generate_plan()

        # Should have failed items
        failed_items = [item for item in plan.items if item.status == PlanItemStatus.FAILED]
        assert len(failed_items) == 1

    def test_generate_plan_builds_hierarchy(self, mock_spotify_client):
        """Test that plan generator builds parent-child relationships."""
        # Create config with only artists
        config = MusicDLConfig(
            version="1.2",
            download=DownloadSettings(client_id="test", client_secret="test"),
            artists=[
                MusicSource(name="Test Artist", url="https://open.spotify.com/artist/456"),
            ],
        )

        # Mock artist data
        mock_spotify_client.get_artist.return_value = {
            "id": "456",
            "name": "Test Artist",
            "external_urls": {"spotify": "https://open.spotify.com/artist/456"},
        }

        mock_spotify_client.get_artist_albums.return_value = [
            {
                "id": "album1",
                "name": "Album 1",
                "album_type": "album",
                "external_urls": {"spotify": "https://open.spotify.com/album/album1"},
            }
        ]

        mock_spotify_client.get_album.return_value = {
            "id": "album1",
            "name": "Album 1",
            "tracks": {
                "items": [
                    {
                        "id": "track1",
                        "name": "Track 1",
                        "track_number": 1,
                        "disc_number": 1,
                        "external_urls": {"spotify": "https://open.spotify.com/track/track1"},
                    }
                ],
                "next": None,
            },
        }

        generator = PlanGenerator(config, mock_spotify_client)
        plan = generator.generate_plan()

        # Find artist item
        artist_item = next(
            (item for item in plan.items if item.item_type == PlanItemType.ARTIST),
            None,
        )
        assert artist_item is not None
        assert len(artist_item.child_ids) > 0

        # Find album item
        album_item = plan.get_item(artist_item.child_ids[0])
        assert album_item is not None
        assert album_item.parent_id == artist_item.item_id

    def test_generate_plan_handles_pagination(self, mock_spotify_client):
        """Test that plan generator handles pagination."""
        # Create config with only playlists
        config = MusicDLConfig(
            version="1.2",
            download=DownloadSettings(client_id="test", client_secret="test"),
            playlists=[
                MusicSource(name="Test Playlist", url="https://open.spotify.com/playlist/789"),
            ],
        )

        # Mock playlist with pagination
        first_page = {
            "id": "789",
            "name": "Test Playlist",
            "external_urls": {"spotify": "https://open.spotify.com/playlist/789"},
            "tracks": {
                "items": [
                    {
                        "track": {
                            "id": "track1",
                            "name": "Track 1",
                            "is_local": False,
                            "external_urls": {"spotify": "https://open.spotify.com/track/track1"},
                        },
                        "added_at": "2023-01-01T00:00:00Z",
                    }
                ],
                "next": "next_page_url",
            },
        }

        second_page = {
            "items": [
                {
                    "track": {
                        "id": "track2",
                        "name": "Track 2",
                        "is_local": False,
                        "external_urls": {"spotify": "https://open.spotify.com/track/track2"},
                    },
                    "added_at": "2023-01-01T00:00:00Z",
                }
            ],
            "next": None,
        }

        mock_spotify_client.get_playlist.return_value = first_page
        # Mock the rate-limited pagination method
        mock_spotify_client._next_with_rate_limit.return_value = second_page

        generator = PlanGenerator(config, mock_spotify_client)
        plan = generator.generate_plan()

        # Should have both tracks
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]
        assert len(track_items) == 2

