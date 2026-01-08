"""
Unit tests for PlanExecutor.
"""
import pytest
from pathlib import Path
from unittest.mock import Mock, MagicMock, patch

from core.config import DownloadSettings
from core.downloader import Downloader
from core.plan import DownloadPlan, PlanItem, PlanItemType, PlanItemStatus
from core.plan_executor import PlanExecutor


@pytest.fixture
def sample_config():
    """Create a sample DownloadSettings."""
    return DownloadSettings(
        client_id="test_id",
        client_secret="test_secret",
        threads=2,
    )


@pytest.fixture
def mock_downloader(sample_config):
    """Create a mocked Downloader."""
    downloader = Mock(spec=Downloader)
    downloader.config = sample_config
    downloader.download_track = Mock(return_value=(True, Path("test.mp3")))
    return downloader


@pytest.fixture
def sample_plan():
    """Create a sample DownloadPlan."""
    plan = DownloadPlan()
    
    track1 = PlanItem(
        item_id="track:1",
        item_type=PlanItemType.TRACK,
        spotify_url="https://open.spotify.com/track/1",
        name="Track 1",
    )
    
    track2 = PlanItem(
        item_id="track:2",
        item_type=PlanItemType.TRACK,
        spotify_url="https://open.spotify.com/track/2",
        name="Track 2",
    )
    
    plan.items = [track1, track2]
    return plan


class TestPlanExecutor:
    """Test PlanExecutor."""

    def test_execute_processes_tracks(self, mock_downloader, sample_plan):
        """Test that executor processes track items."""
        executor = PlanExecutor(mock_downloader, max_workers=2)
        stats = executor.execute(sample_plan)

        # Should have called download_track for each track
        assert mock_downloader.download_track.call_count == 2
        assert stats["completed"] == 2

    def test_execute_updates_item_status(self, mock_downloader, sample_plan):
        """Test that executor updates item status during execution."""
        executor = PlanExecutor(mock_downloader, max_workers=2)
        executor.execute(sample_plan)

        # All tracks should be completed
        for item in sample_plan.items:
            if item.item_type == PlanItemType.TRACK:
                assert item.status == PlanItemStatus.COMPLETED
                assert item.file_path is not None

    def test_execute_handles_failures(self, mock_downloader, sample_plan):
        """Test that executor handles download failures."""
        # Make download_track fail for track:1, succeed for track:2
        # Use a function to return values based on URL to avoid race conditions
        def download_side_effect(url):
            if "track/1" in url or url.endswith("/1"):
                return (False, None)  # track:1 fails
            else:
                return (True, Path("test2.mp3"))  # track:2 succeeds
        
        mock_downloader.download_track.side_effect = download_side_effect

        executor = PlanExecutor(mock_downloader, max_workers=2)
        stats = executor.execute(sample_plan)

        # Should have 1 completed and 1 failed
        assert stats["completed"] == 1
        assert stats["failed"] == 1

        # Check item statuses (order-independent)
        track1 = sample_plan.get_item("track:1")
        track2 = sample_plan.get_item("track:2")
        assert track1.status == PlanItemStatus.FAILED
        assert track2.status == PlanItemStatus.COMPLETED

    def test_execute_handles_exceptions(self, mock_downloader, sample_plan):
        """Test that executor handles exceptions gracefully."""
        # Make download_track raise an exception
        mock_downloader.download_track.side_effect = Exception("Download error")

        executor = PlanExecutor(mock_downloader, max_workers=2)
        stats = executor.execute(sample_plan)

        # Should have failed items
        assert stats["failed"] == 2

        # Items should be marked as failed
        for item in sample_plan.items:
            if item.item_type == PlanItemType.TRACK:
                assert item.status == PlanItemStatus.FAILED
                assert item.error is not None

    def test_execute_calls_progress_callback(self, mock_downloader, sample_plan):
        """Test that executor calls progress callback."""
        callback_calls = []

        def progress_callback(item):
            callback_calls.append(item.item_id)

        executor = PlanExecutor(mock_downloader, max_workers=2)
        executor.execute(sample_plan, progress_callback=progress_callback)

        # Should have called callback for each item
        assert len(callback_calls) >= 2

    def test_execute_processes_containers(self, mock_downloader):
        """Test that executor processes container items."""
        plan = DownloadPlan()
        
        artist = PlanItem(
            item_id="artist:1",
            item_type=PlanItemType.ARTIST,
            name="Artist 1",
        )
        
        album = PlanItem(
            item_id="album:1",
            item_type=PlanItemType.ALBUM,
            parent_id="artist:1",
            name="Album 1",
        )
        
        track1 = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            spotify_url="https://open.spotify.com/track/1",
            parent_id="album:1",
            name="Track 1",
        )
        track1.mark_completed(Path("test1.mp3"))
        
        track2 = PlanItem(
            item_id="track:2",
            item_type=PlanItemType.TRACK,
            spotify_url="https://open.spotify.com/track/2",
            parent_id="album:1",
            name="Track 2",
        )
        track2.mark_completed(Path("test2.mp3"))
        
        artist.child_ids = [album.item_id]
        album.child_ids = [track1.item_id, track2.item_id]
        
        plan.items = [artist, album, track1, track2]
        
        executor = PlanExecutor(mock_downloader, max_workers=2)
        executor.execute(plan)

        # Container should be marked as completed
        artist_item = plan.get_item("artist:1")
        album_item = plan.get_item("album:1")
        assert artist_item.status == PlanItemStatus.COMPLETED
        assert album_item.status == PlanItemStatus.COMPLETED

    def test_execute_creates_m3u_files(self, mock_downloader):
        """Test that executor creates M3U files for playlists."""
        plan = DownloadPlan()
        
        playlist = PlanItem(
            item_id="playlist:1",
            item_type=PlanItemType.PLAYLIST,
            name="Test Playlist",
        )
        
        track1 = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            spotify_url="https://open.spotify.com/track/1",
            parent_id="playlist:1",
            name="Track 1",
        )
        track1.mark_completed(Path("test1.mp3"))
        
        track2 = PlanItem(
            item_id="track:2",
            item_type=PlanItemType.TRACK,
            spotify_url="https://open.spotify.com/track/2",
            parent_id="playlist:1",
            name="Track 2",
        )
        track2.mark_completed(Path("test2.mp3"))
        
        m3u = PlanItem(
            item_id="m3u:1",
            item_type=PlanItemType.M3U,
            parent_id="playlist:1",
            name="Test Playlist.m3u",
            metadata={"playlist_name": "Test Playlist"},
        )
        
        playlist.child_ids = [track1.item_id, track2.item_id, m3u.item_id]
        
        plan.items = [playlist, track1, track2, m3u]
        
        executor = PlanExecutor(mock_downloader, max_workers=2)
        
        with patch("pathlib.Path.exists", return_value=True):
            with patch("builtins.open", create=True) as mock_open:
                mock_file = MagicMock()
                mock_open.return_value.__enter__.return_value = mock_file
                
                executor.execute(plan)

                # M3U should be completed
                m3u_item = plan.get_item("m3u:1")
                assert m3u_item.status == PlanItemStatus.COMPLETED
                assert m3u_item.file_path is not None

    def test_execute_handles_m3u_with_no_tracks(self, mock_downloader):
        """Test that executor handles M3U creation when no tracks exist."""
        plan = DownloadPlan()
        
        playlist = PlanItem(
            item_id="playlist:1",
            item_type=PlanItemType.PLAYLIST,
            name="Test Playlist",
        )
        
        m3u = PlanItem(
            item_id="m3u:1",
            item_type=PlanItemType.M3U,
            parent_id="playlist:1",
            name="Test Playlist.m3u",
            metadata={"playlist_name": "Test Playlist"},
        )
        
        playlist.child_ids = [m3u.item_id]
        
        plan.items = [playlist, m3u]
        
        executor = PlanExecutor(mock_downloader, max_workers=2)
        executor.execute(plan)

        # M3U should be marked as failed (no tracks)
        m3u_item = plan.get_item("m3u:1")
        assert m3u_item.status == PlanItemStatus.FAILED
        
        # Playlist container should also be marked as failed since its only child failed
        playlist_item = plan.get_item("playlist:1")
        assert playlist_item.status == PlanItemStatus.FAILED

    def test_execute_creates_m3u_files_for_albums(self, mock_downloader):
        """Test that executor creates M3U files for albums when requested."""
        plan = DownloadPlan()
        
        album = PlanItem(
            item_id="album:1",
            item_type=PlanItemType.ALBUM,
            name="Test Album",
            metadata={"create_m3u": True},
        )
        
        track1 = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            spotify_url="https://open.spotify.com/track/1",
            parent_id="album:1",
            name="Track 1",
        )
        track1.mark_completed(Path("test1.mp3"))
        
        track2 = PlanItem(
            item_id="track:2",
            item_type=PlanItemType.TRACK,
            spotify_url="https://open.spotify.com/track/2",
            parent_id="album:1",
            name="Track 2",
        )
        track2.mark_completed(Path("test2.mp3"))
        
        m3u = PlanItem(
            item_id="m3u:album:1",
            item_type=PlanItemType.M3U,
            parent_id="album:1",
            name="Test Album.m3u",
            metadata={"album_name": "Test Album"},
        )
        
        album.child_ids = [track1.item_id, track2.item_id, m3u.item_id]
        
        plan.items = [album, track1, track2, m3u]
        
        executor = PlanExecutor(mock_downloader, max_workers=2)
        
        with patch("pathlib.Path.exists", return_value=True):
            with patch("builtins.open", create=True) as mock_open:
                mock_file = MagicMock()
                mock_open.return_value.__enter__.return_value = mock_file
                
                executor.execute(plan)

                # M3U should be completed
                m3u_item = plan.get_item("m3u:album:1")
                assert m3u_item.status == PlanItemStatus.COMPLETED
                assert m3u_item.file_path is not None

    def test_execute_respects_max_workers(self, mock_downloader, sample_plan):
        """Test that executor respects max_workers setting."""
        executor = PlanExecutor(mock_downloader, max_workers=1)
        executor.execute(sample_plan)

        # Should still process all items (just sequentially)
        assert mock_downloader.download_track.call_count == 2

    def test_execute_returns_statistics(self, mock_downloader, sample_plan):
        """Test that executor returns execution statistics."""
        executor = PlanExecutor(mock_downloader, max_workers=2)
        stats = executor.execute(sample_plan)

        assert "completed" in stats
        assert "failed" in stats
        assert "pending" in stats
        assert "in_progress" in stats
        assert "total" in stats
        # SKIPPED items are excluded from statistics
        assert "skipped" not in stats

    def test_execute_excludes_skipped_from_statistics(self, mock_downloader):
        """Test that executor excludes SKIPPED items from statistics."""
        plan = DownloadPlan()
        
        # Add a completed track
        track1 = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            spotify_url="https://open.spotify.com/track/1",
            name="Track 1",
        )
        track1.mark_completed(Path("test1.mp3"))
        
        # Add a skipped track (should be excluded from stats)
        track2 = PlanItem(
            item_id="track:2",
            item_type=PlanItemType.TRACK,
            spotify_url="https://open.spotify.com/track/2",
            name="Track 2",
        )
        track2.mark_skipped("File already exists")
        
        plan.items = [track1, track2]
        
        executor = PlanExecutor(mock_downloader, max_workers=2)
        stats = executor.execute(plan)
        
        # Should only count track1 (completed), not track2 (skipped)
        assert stats["total"] == 1
        assert stats["completed"] == 1
        assert stats["failed"] == 0
        assert stats["pending"] == 0

