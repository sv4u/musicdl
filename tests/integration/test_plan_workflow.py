"""
Integration tests for plan-based architecture workflow.

Tests the complete workflow from configuration to plan execution.
"""
import pytest
from pathlib import Path
from unittest.mock import Mock, MagicMock, patch

from core.config import MusicDLConfig, MusicSource, DownloadSettings
from core.downloader import Downloader
from core.plan import DownloadPlan, PlanItemType, PlanItemStatus
from core.plan_executor import PlanExecutor
from core.plan_generator import PlanGenerator
from core.plan_optimizer import PlanOptimizer
from core.spotify_client import SpotifyClient


@pytest.fixture
def sample_config():
    """Create a sample MusicDLConfig."""
    download_settings = DownloadSettings(
        client_id="test_id",
        client_secret="test_secret",
        use_plan_architecture=True,
        plan_generation_enabled=True,
        plan_optimization_enabled=True,
        plan_execution_enabled=True,
        overwrite="skip",
    )
    return MusicDLConfig(
        version="1.2",
        download=download_settings,
        songs=[
            MusicSource(name="Test Song", url="https://open.spotify.com/track/123"),
        ],
    )


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
def mock_downloader(sample_config):
    """Create a mocked Downloader."""
    downloader = Mock(spec=Downloader)
    downloader.config = sample_config.download
    downloader.download_track = Mock(return_value=(True, Path("test.mp3")))
    return downloader


class TestPlanWorkflow:
    """Test complete plan-based workflow."""

    def test_full_workflow_song(self, sample_config, mock_spotify_client, mock_downloader):
        """Test full workflow with a single song."""
        # Mock track data
        mock_spotify_client.get_track.return_value = {
            "id": "123",
            "name": "Test Track",
            "artists": [{"name": "Test Artist"}],
            "external_urls": {"spotify": "https://open.spotify.com/track/123"},
        }

        # Generate plan
        generator = PlanGenerator(sample_config, mock_spotify_client)
        plan = generator.generate_plan()

        assert len(plan.items) > 0
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]
        assert len(track_items) == 1

        # Optimize plan
        optimizer = PlanOptimizer(
            sample_config.download,
            mock_spotify_client,
            check_file_existence=False,
        )
        optimized_plan = optimizer.optimize(plan)

        assert len(optimized_plan.items) == 1

        # Execute plan
        executor = PlanExecutor(mock_downloader, max_workers=2)
        stats = executor.execute(optimized_plan)

        assert stats["completed"] == 1
        assert stats["failed"] == 0

        # Verify track was downloaded
        track_item = optimized_plan.items[0]
        assert track_item.status == PlanItemStatus.COMPLETED
        assert track_item.file_path is not None

    def test_full_workflow_with_duplicates(self, sample_config, mock_spotify_client, mock_downloader):
        """Test full workflow with duplicate songs."""
        # Add duplicate song
        sample_config.songs.append(
            MusicSource(name="Test Song 2", url="https://open.spotify.com/track/123")
        )

        # Mock track data
        mock_spotify_client.get_track.return_value = {
            "id": "123",
            "name": "Test Track",
            "artists": [{"name": "Test Artist"}],
            "external_urls": {"spotify": "https://open.spotify.com/track/123"},
        }

        # Generate plan
        generator = PlanGenerator(sample_config, mock_spotify_client)
        plan = generator.generate_plan()

        # Should have 2 track items initially (generator tracks duplicates)
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]
        assert len(track_items) == 1  # Generator already removes duplicates

        # Optimize plan
        optimizer = PlanOptimizer(
            sample_config.download,
            mock_spotify_client,
            check_file_existence=False,
        )
        optimized_plan = optimizer.optimize(plan)

        # Should still have 1 track (duplicate removed)
        track_items = [item for item in optimized_plan.items if item.item_type == PlanItemType.TRACK]
        assert len(track_items) == 1

        # Execute plan
        executor = PlanExecutor(mock_downloader, max_workers=2)
        stats = executor.execute(optimized_plan)

        # Should only download once
        assert mock_downloader.download_track.call_count == 1
        assert stats["completed"] == 1

    def test_full_workflow_with_artist(self, sample_config, mock_spotify_client, mock_downloader):
        """Test full workflow with an artist."""
        sample_config.artists = [
            MusicSource(name="Test Artist", url="https://open.spotify.com/artist/456"),
        ]
        sample_config.songs = []

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

        # Generate plan
        generator = PlanGenerator(sample_config, mock_spotify_client)
        plan = generator.generate_plan()

        # Should have artist, album, and track items
        artist_items = [item for item in plan.items if item.item_type == PlanItemType.ARTIST]
        album_items = [item for item in plan.items if item.item_type == PlanItemType.ALBUM]
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]

        assert len(artist_items) == 1
        assert len(album_items) == 1
        assert len(track_items) == 1

        # Optimize plan
        optimizer = PlanOptimizer(
            sample_config.download,
            mock_spotify_client,
            check_file_existence=False,
        )
        optimized_plan = optimizer.optimize(plan)

        # Execute plan
        executor = PlanExecutor(mock_downloader, max_workers=2)
        stats = executor.execute(optimized_plan)

        # Should have completed items (track + containers)
        assert stats["completed"] >= 1

        # Verify track was completed
        track_items = optimized_plan.get_items_by_type(PlanItemType.TRACK)
        assert len(track_items) == 1
        assert track_items[0].status == PlanItemStatus.COMPLETED

        # Container items should be marked as completed
        artist_item = optimized_plan.get_items_by_type(PlanItemType.ARTIST)[0]
        album_item = optimized_plan.get_items_by_type(PlanItemType.ALBUM)[0]
        assert artist_item.status == PlanItemStatus.COMPLETED
        assert album_item.status == PlanItemStatus.COMPLETED

    def test_full_workflow_with_playlist(self, sample_config, mock_spotify_client, mock_downloader):
        """Test full workflow with a playlist."""
        sample_config.playlists = [
            MusicSource(name="Test Playlist", url="https://open.spotify.com/playlist/789"),
        ]
        sample_config.songs = []

        # Mock playlist data
        mock_spotify_client.get_playlist.return_value = {
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
                "next": None,
            },
        }

        # Generate plan
        generator = PlanGenerator(sample_config, mock_spotify_client)
        plan = generator.generate_plan()

        # Should have playlist, track, and M3U items
        playlist_items = [item for item in plan.items if item.item_type == PlanItemType.PLAYLIST]
        track_items = [item for item in plan.items if item.item_type == PlanItemType.TRACK]
        m3u_items = [item for item in plan.items if item.item_type == PlanItemType.M3U]

        assert len(playlist_items) == 1
        assert len(track_items) == 1
        assert len(m3u_items) == 1

        # Optimize plan
        optimizer = PlanOptimizer(
            sample_config.download,
            mock_spotify_client,
            check_file_existence=False,
        )
        optimized_plan = optimizer.optimize(plan)

        # Execute plan
        executor = PlanExecutor(mock_downloader, max_workers=2)
        
        with patch("pathlib.Path.exists", return_value=True):
            with patch("builtins.open", create=True) as mock_open:
                mock_file = MagicMock()
                mock_open.return_value.__enter__.return_value = mock_file
                
                stats = executor.execute(optimized_plan)

                # Should have completed track and M3U
                assert stats["completed"] >= 1

                # M3U should be created
                m3u_item = optimized_plan.get_items_by_type(PlanItemType.M3U)[0]
                assert m3u_item.status == PlanItemStatus.COMPLETED

    def test_full_workflow_plan_persistence(self, sample_config, mock_spotify_client, tmp_path):
        """Test plan persistence (save/load)."""
        # Mock track data
        mock_spotify_client.get_track.return_value = {
            "id": "123",
            "name": "Test Track",
            "artists": [{"name": "Test Artist"}],
            "external_urls": {"spotify": "https://open.spotify.com/track/123"},
        }

        # Generate plan
        generator = PlanGenerator(sample_config, mock_spotify_client)
        plan = generator.generate_plan()

        # Save plan
        plan_path = tmp_path / "test_plan.json"
        plan.save(plan_path)
        assert plan_path.exists()

        # Load plan
        loaded_plan = DownloadPlan.load(plan_path)
        assert len(loaded_plan.items) == len(plan.items)
        assert loaded_plan.items[0].item_id == plan.items[0].item_id

