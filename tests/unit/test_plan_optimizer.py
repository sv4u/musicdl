"""
Unit tests for PlanOptimizer.
"""
import pytest
from pathlib import Path
from unittest.mock import Mock, MagicMock, patch

from core.config import DownloadSettings
from core.plan import DownloadPlan, PlanItem, PlanItemType, PlanItemStatus
from core.plan_optimizer import PlanOptimizer
from core.spotify_client import SpotifyClient


@pytest.fixture
def mock_spotify_client():
    """Create a mocked SpotifyClient."""
    client = Mock(spec=SpotifyClient)
    client.get_track = Mock()
    client.get_album = Mock()
    return client


@pytest.fixture
def sample_config():
    """Create a sample DownloadSettings."""
    return DownloadSettings(
        client_id="test_id",
        client_secret="test_secret",
        overwrite="skip",
        output="{artist}/{album}/{title}.{output-ext}",
        format="mp3",
    )


@pytest.fixture
def sample_plan():
    """Create a sample DownloadPlan with duplicate items."""
    plan = DownloadPlan()
    
    # Add duplicate tracks
    track1 = PlanItem(
        item_id="track:1",
        item_type=PlanItemType.TRACK,
        spotify_id="123",
        name="Track 1",
    )
    track2 = PlanItem(
        item_id="track:2",
        item_type=PlanItemType.TRACK,
        spotify_id="123",  # Duplicate Spotify ID
        name="Track 1 Duplicate",
    )
    track3 = PlanItem(
        item_id="track:3",
        item_type=PlanItemType.TRACK,
        spotify_id="456",
        name="Track 2",
    )
    
    plan.items = [track1, track2, track3]
    return plan


class TestPlanOptimizer:
    """Test PlanOptimizer."""

    def test_optimize_removes_duplicates(self, mock_spotify_client, sample_config, sample_plan):
        """Test that optimizer removes duplicate items."""
        optimizer = PlanOptimizer(sample_config, mock_spotify_client)
        optimized_plan = optimizer.optimize(sample_plan)

        # Should have 2 tracks (one duplicate removed)
        track_items = [item for item in optimized_plan.items if item.item_type == PlanItemType.TRACK]
        assert len(track_items) == 2

        # Check that duplicate was removed (keep first occurrence)
        spotify_ids = [item.spotify_id for item in track_items]
        assert spotify_ids.count("123") == 1
        assert "456" in spotify_ids

    def test_optimize_sorts_items(self, mock_spotify_client, sample_config):
        """Test that optimizer sorts items correctly."""
        plan = DownloadPlan()
        
        # Add items in random order
        m3u = PlanItem(
            item_id="m3u:1",
            item_type=PlanItemType.M3U,
            name="playlist.m3u",
        )
        album = PlanItem(
            item_id="album:1",
            item_type=PlanItemType.ALBUM,
            name="Album 1",
        )
        track = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            name="Track 1",
        )
        
        plan.items = [m3u, album, track]
        
        optimizer = PlanOptimizer(sample_config, mock_spotify_client, check_file_existence=False)
        optimized_plan = optimizer.optimize(plan)

        # Should be sorted: tracks first, then albums, then M3U
        assert optimized_plan.items[0].item_type == PlanItemType.TRACK
        assert optimized_plan.items[1].item_type == PlanItemType.ALBUM
        assert optimized_plan.items[2].item_type == PlanItemType.M3U

    def test_optimize_checks_file_existence(self, mock_spotify_client, sample_config):
        """Test that optimizer checks file existence and marks as skipped."""
        plan = DownloadPlan()
        
        track = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            spotify_id="123",
            name="Track 1",
        )
        plan.items = [track]

        # Mock track and album data
        mock_spotify_client.get_track.return_value = {
            "id": "123",
            "name": "Track 1",
            "artists": [{"name": "Artist 1"}],
            "album": {"id": "album1"},
            "external_urls": {"spotify": "https://open.spotify.com/track/123"},
        }

        mock_spotify_client.get_album.return_value = {
            "id": "album1",
            "name": "Album 1",
            "artists": [{"name": "Artist 1"}],
            "release_date": "2023-01-01",
            "tracks": {"items": []},
        }

        # Mock file exists
        with patch("pathlib.Path.exists", return_value=True):
            optimizer = PlanOptimizer(sample_config, mock_spotify_client, check_file_existence=True)
            optimized_plan = optimizer.optimize(plan)

            # Track should be marked as skipped
            assert optimized_plan.items[0].status == PlanItemStatus.SKIPPED

    def test_optimize_skips_file_check_when_disabled(self, mock_spotify_client, sample_config):
        """Test that optimizer skips file existence check when disabled."""
        plan = DownloadPlan()
        
        track = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            spotify_id="123",
            name="Track 1",
        )
        plan.items = [track]

        optimizer = PlanOptimizer(sample_config, mock_spotify_client, check_file_existence=False)
        optimized_plan = optimizer.optimize(plan)

        # Track should still be pending (not checked)
        assert optimized_plan.items[0].status == PlanItemStatus.PENDING

    def test_optimize_handles_overwrite_mode(self, mock_spotify_client):
        """Test that optimizer respects overwrite mode."""
        config = DownloadSettings(
            client_id="test_id",
            client_secret="test_secret",
            overwrite="overwrite",  # Don't skip existing files
        )

        plan = DownloadPlan()
        track = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            spotify_id="123",
            name="Track 1",
        )
        plan.items = [track]

        optimizer = PlanOptimizer(config, mock_spotify_client, check_file_existence=True)
        optimized_plan = optimizer.optimize(plan)

        # Should not check file existence when overwrite != "skip" or "metadata"
        assert optimized_plan.items[0].status == PlanItemStatus.PENDING

    def test_optimize_handles_metadata_mode(self, mock_spotify_client):
        """Test that optimizer keeps items PENDING when overwrite=metadata and file exists."""
        config = DownloadSettings(
            client_id="test_id",
            client_secret="test_secret",
            overwrite="metadata",  # Update metadata for existing files
            output="{artist}/{album}/{title}.{output-ext}",
            format="mp3",
        )

        plan = DownloadPlan()
        track = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            spotify_id="123",
            name="Track 1",
        )
        plan.items = [track]

        # Mock track and album data
        mock_spotify_client.get_track.return_value = {
            "id": "123",
            "name": "Track 1",
            "artists": [{"name": "Artist 1"}],
            "album": {"id": "album1"},
            "external_urls": {"spotify": "https://open.spotify.com/track/123"},
        }

        mock_spotify_client.get_album.return_value = {
            "id": "album1",
            "name": "Album 1",
            "artists": [{"name": "Artist 1"}],
            "release_date": "2023-01-01",
            "tracks": {"items": []},
        }

        # Mock file exists
        with patch("pathlib.Path.exists", return_value=True):
            optimizer = PlanOptimizer(config, mock_spotify_client, check_file_existence=True)
            optimized_plan = optimizer.optimize(plan)

            # Track should remain PENDING (not SKIPPED) for metadata update
            assert optimized_plan.items[0].status == PlanItemStatus.PENDING
            assert optimized_plan.items[0].file_path is not None

    def test_optimize_handles_missing_spotify_id(self, mock_spotify_client, sample_config):
        """Test that optimizer handles items without Spotify ID."""
        plan = DownloadPlan()
        
        track1 = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            spotify_id="123",
            name="Track 1",
        )
        track2 = PlanItem(
            item_id="track:2",
            item_type=PlanItemType.TRACK,
            spotify_id=None,  # No Spotify ID
            name="Track 2",
        )
        
        plan.items = [track1, track2]
        
        optimizer = PlanOptimizer(sample_config, mock_spotify_client, check_file_existence=False)
        optimized_plan = optimizer.optimize(plan)

        # Both items should remain (can't deduplicate without Spotify ID)
        assert len(optimized_plan.items) == 2

    def test_optimize_updates_parent_child_relationships(self, mock_spotify_client, sample_config):
        """Test that optimizer updates parent-child relationships when removing duplicates."""
        plan = DownloadPlan()
        
        album = PlanItem(
            item_id="album:1",
            item_type=PlanItemType.ALBUM,
            spotify_id="album1",
            name="Album 1",
        )
        
        track1 = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            spotify_id="123",
            parent_id="album:1",
            name="Track 1",
        )
        
        track2 = PlanItem(
            item_id="track:2",
            item_type=PlanItemType.TRACK,
            spotify_id="123",  # Duplicate
            parent_id="album:1",
            name="Track 1 Duplicate",
        )
        
        album.child_ids = [track1.item_id, track2.item_id]
        plan.items = [album, track1, track2]
        
        optimizer = PlanOptimizer(sample_config, mock_spotify_client, check_file_existence=False)
        optimized_plan = optimizer.optimize(plan)

        # Find album item
        album_item = next(
            (item for item in optimized_plan.items if item.item_type == PlanItemType.ALBUM),
            None,
        )
        assert album_item is not None
        
        # Should only have one child (duplicate removed)
        assert len(album_item.child_ids) == 1

    def test_optimize_handles_cross_parent_duplicates(self, mock_spotify_client, sample_config):
        """Test that optimizer maintains parent references when duplicates have different parents."""
        plan = DownloadPlan()
        
        album1 = PlanItem(
            item_id="album:1",
            item_type=PlanItemType.ALBUM,
            spotify_id="album1",
            name="Album 1",
        )
        
        album2 = PlanItem(
            item_id="album:2",
            item_type=PlanItemType.ALBUM,
            spotify_id="album2",
            name="Album 2",
        )
        
        # Track 1 in Album 1
        track1 = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            spotify_id="123",
            parent_id="album:1",
            name="Track 1",
        )
        
        # Track 2 in Album 2 (same spotify_id, different parent)
        track2 = PlanItem(
            item_id="track:2",
            item_type=PlanItemType.TRACK,
            spotify_id="123",  # Duplicate of track:1
            parent_id="album:2",
            name="Track 1 Duplicate",
        )
        
        album1.child_ids = [track1.item_id]
        album2.child_ids = [track2.item_id]
        plan.items = [album1, album2, track1, track2]
        
        optimizer = PlanOptimizer(sample_config, mock_spotify_client, check_file_existence=False)
        optimized_plan = optimizer.optimize(plan)
        
        # Find album items
        album1_item = optimized_plan.get_item("album:1")
        album2_item = optimized_plan.get_item("album:2")
        assert album1_item is not None
        assert album2_item is not None
        
        # Album 1 should have track:1 (original)
        assert track1.item_id in album1_item.child_ids
        assert len(album1_item.child_ids) == 1
        
        # Album 2 should have track:1 (original) instead of track:2 (duplicate)
        # This ensures M3U generation and progress tracking work correctly
        assert track1.item_id in album2_item.child_ids
        assert track2.item_id not in album2_item.child_ids
        assert len(album2_item.child_ids) == 1
        
        # track:2 should be removed from plan
        assert optimized_plan.get_item("track:2") is None
        # track:1 should still exist
        assert optimized_plan.get_item("track:1") is not None

