"""
Unit tests for plan data models.
"""
import json
import time
import pytest
from pathlib import Path
from tempfile import TemporaryDirectory

from core.plan import (
    DownloadPlan,
    PlanItem,
    PlanItemStatus,
    PlanItemType,
)


class TestPlanItem:
    """Test PlanItem dataclass."""

    def test_plan_item_creation(self):
        """Test creating a plan item."""
        item = PlanItem(
            item_id="track:123",
            item_type=PlanItemType.TRACK,
            spotify_id="123",
            spotify_url="https://open.spotify.com/track/123",
            name="Test Track",
        )
        assert item.item_id == "track:123"
        assert item.item_type == PlanItemType.TRACK
        assert item.spotify_id == "123"
        assert item.status == PlanItemStatus.PENDING
        assert item.progress == 0.0

    def test_plan_item_mark_started(self):
        """Test marking item as started."""
        item = PlanItem(
            item_id="track:123",
            item_type=PlanItemType.TRACK,
            name="Test Track",
        )
        item.mark_started()
        assert item.status == PlanItemStatus.IN_PROGRESS
        assert item.started_at is not None
        assert item.started_at <= time.time()

    def test_plan_item_mark_completed(self):
        """Test marking item as completed."""
        item = PlanItem(
            item_id="track:123",
            item_type=PlanItemType.TRACK,
            name="Test Track",
        )
        file_path = Path("test.mp3")
        item.mark_completed(file_path)
        assert item.status == PlanItemStatus.COMPLETED
        assert item.completed_at is not None
        assert item.progress == 1.0
        assert item.file_path == file_path

    def test_plan_item_mark_failed(self):
        """Test marking item as failed."""
        item = PlanItem(
            item_id="track:123",
            item_type=PlanItemType.TRACK,
            name="Test Track",
        )
        error = "Download failed"
        item.mark_failed(error)
        assert item.status == PlanItemStatus.FAILED
        assert item.error == error
        assert item.completed_at is not None

    def test_plan_item_mark_skipped(self):
        """Test marking item as skipped."""
        item = PlanItem(
            item_id="track:123",
            item_type=PlanItemType.TRACK,
            name="Test Track",
        )
        item.mark_skipped("File already exists")
        assert item.status == PlanItemStatus.SKIPPED
        assert item.completed_at is not None

    def test_plan_item_to_dict(self):
        """Test converting plan item to dictionary."""
        item = PlanItem(
            item_id="track:123",
            item_type=PlanItemType.TRACK,
            spotify_id="123",
            name="Test Track",
            file_path=Path("test.mp3"),
        )
        data = item.to_dict()
        assert data["item_id"] == "track:123"
        assert data["item_type"] == "track"
        assert data["status"] == "pending"
        assert data["file_path"] == "test.mp3"

    def test_plan_item_from_dict(self):
        """Test creating plan item from dictionary."""
        data = {
            "item_id": "track:123",
            "item_type": "track",
            "spotify_id": "123",
            "name": "Test Track",
            "status": "pending",
            "file_path": "test.mp3",
            "parent_id": None,
            "child_ids": [],
            "metadata": {},
            "error": None,
            "created_at": time.time(),
            "started_at": None,
            "completed_at": None,
            "progress": 0.0,
        }
        item = PlanItem.from_dict(data)
        assert item.item_id == "track:123"
        assert item.item_type == PlanItemType.TRACK
        assert item.file_path == Path("test.mp3")

    def test_plan_item_from_dict_without_status(self):
        """Test creating plan item from dictionary without status field."""
        # Minimal data without status field (should use default PENDING)
        data = {
            "item_id": "track:456",
            "item_type": "track",
            "name": "Minimal Track",
        }
        item = PlanItem.from_dict(data)
        assert item.item_id == "track:456"
        assert item.item_type == PlanItemType.TRACK
        assert item.status == PlanItemStatus.PENDING  # Should use default
        assert item.name == "Minimal Track"


class TestDownloadPlan:
    """Test DownloadPlan dataclass."""

    def test_download_plan_creation(self):
        """Test creating a download plan."""
        plan = DownloadPlan()
        assert len(plan.items) == 0
        assert plan.created_at <= time.time()

    def test_download_plan_add_items(self):
        """Test adding items to plan."""
        plan = DownloadPlan()
        item1 = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            name="Track 1",
        )
        item2 = PlanItem(
            item_id="track:2",
            item_type=PlanItemType.TRACK,
            name="Track 2",
        )
        plan.items = [item1, item2]
        assert len(plan.items) == 2

    def test_download_plan_get_item(self):
        """Test getting item by ID."""
        plan = DownloadPlan()
        item = PlanItem(
            item_id="track:123",
            item_type=PlanItemType.TRACK,
            name="Test Track",
        )
        plan.items = [item]
        assert plan.get_item("track:123") == item
        assert plan.get_item("track:999") is None

    def test_download_plan_get_items_by_type(self):
        """Test getting items by type."""
        plan = DownloadPlan()
        track1 = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            name="Track 1",
        )
        track2 = PlanItem(
            item_id="track:2",
            item_type=PlanItemType.TRACK,
            name="Track 2",
        )
        album = PlanItem(
            item_id="album:1",
            item_type=PlanItemType.ALBUM,
            name="Album 1",
        )
        plan.items = [track1, track2, album]
        tracks = plan.get_items_by_type(PlanItemType.TRACK)
        assert len(tracks) == 2
        assert all(item.item_type == PlanItemType.TRACK for item in tracks)

    def test_download_plan_get_items_by_status(self):
        """Test getting items by status."""
        plan = DownloadPlan()
        item1 = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            name="Track 1",
        )
        item2 = PlanItem(
            item_id="track:2",
            item_type=PlanItemType.TRACK,
            name="Track 2",
        )
        item2.mark_completed()
        plan.items = [item1, item2]
        completed = plan.get_items_by_status(PlanItemStatus.COMPLETED)
        assert len(completed) == 1
        assert completed[0].item_id == "track:2"

    def test_download_plan_get_statistics(self):
        """Test getting plan statistics."""
        plan = DownloadPlan()
        item1 = PlanItem(
            item_id="track:1",
            item_type=PlanItemType.TRACK,
            name="Track 1",
        )
        item2 = PlanItem(
            item_id="track:2",
            item_type=PlanItemType.TRACK,
            name="Track 2",
        )
        item2.mark_completed()
        plan.items = [item1, item2]
        stats = plan.get_statistics()
        assert stats["total_items"] == 2
        assert stats["by_status"]["pending"] == 1
        assert stats["by_status"]["completed"] == 1
        assert stats["by_type"]["track"] == 2

    def test_download_plan_to_dict(self):
        """Test converting plan to dictionary."""
        plan = DownloadPlan()
        item = PlanItem(
            item_id="track:123",
            item_type=PlanItemType.TRACK,
            name="Test Track",
        )
        plan.items = [item]
        data = plan.to_dict()
        assert "items" in data
        assert len(data["items"]) == 1
        assert "created_at" in data
        assert "metadata" in data

    def test_download_plan_from_dict(self):
        """Test creating plan from dictionary."""
        data = {
            "items": [
                {
                    "item_id": "track:123",
                    "item_type": "track",
                    "spotify_id": "123",
                    "name": "Test Track",
                    "status": "pending",
                    "parent_id": None,
                    "child_ids": [],
                    "metadata": {},
                    "error": None,
                    "file_path": None,
                    "created_at": time.time(),
                    "started_at": None,
                    "completed_at": None,
                    "progress": 0.0,
                    "spotify_url": None,
                }
            ],
            "created_at": time.time(),
            "metadata": {},
        }
        plan = DownloadPlan.from_dict(data)
        assert len(plan.items) == 1
        assert plan.items[0].item_id == "track:123"

    def test_download_plan_save_and_load(self):
        """Test saving and loading plan from file."""
        plan = DownloadPlan()
        item = PlanItem(
            item_id="track:123",
            item_type=PlanItemType.TRACK,
            name="Test Track",
        )
        plan.items = [item]

        with TemporaryDirectory() as tmpdir:
            plan_path = Path(tmpdir) / "test_plan.json"
            plan.save(plan_path)
            assert plan_path.exists()

            loaded_plan = DownloadPlan.load(plan_path)
            assert len(loaded_plan.items) == 1
            assert loaded_plan.items[0].item_id == "track:123"
            assert loaded_plan.items[0].name == "Test Track"

