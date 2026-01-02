"""
Plan-based architecture data models for musicdl.

This module defines the data structures used for the plan-based download architecture,
which enables better optimization, parallelization, and progress tracking.
"""

import json
import threading
import time
from dataclasses import asdict, dataclass, field
from enum import Enum
from pathlib import Path
from typing import Any, Dict, List, Optional


class PlanItemType(str, Enum):
    """Type of plan item."""

    TRACK = "track"  # Individual track download
    ALBUM = "album"  # Album container (parent of tracks)
    ARTIST = "artist"  # Artist container (parent of albums)
    PLAYLIST = "playlist"  # Playlist container (parent of tracks)
    M3U = "m3u"  # M3U playlist file creation (child of playlist)


class PlanItemStatus(str, Enum):
    """Status of a plan item."""

    PENDING = "pending"  # Not yet started
    IN_PROGRESS = "in_progress"  # Currently being processed
    COMPLETED = "completed"  # Successfully completed
    FAILED = "failed"  # Failed with error
    SKIPPED = "skipped"  # Skipped (e.g., file already exists)


@dataclass
class PlanItem:
    """
    Represents a single item in the download plan.

    Each item can be a track, album, artist, playlist, or M3U file.
    Items form a hierarchy where containers (albums, artists, playlists)
    have child items (tracks).
    """

    # Identification
    item_id: str  # Unique identifier (Spotify ID or generated ID)
    item_type: PlanItemType  # Type of item
    spotify_id: Optional[str] = None  # Spotify ID if applicable
    spotify_url: Optional[str] = None  # Spotify URL if applicable

    # Hierarchy
    parent_id: Optional[str] = None  # Parent item ID (e.g., album ID for track)
    child_ids: List[str] = field(default_factory=list)  # Child item IDs

    # Metadata
    name: str = ""  # Display name (e.g., track name, album name)
    metadata: Dict[str, Any] = field(default_factory=dict)  # Additional metadata

    # Status tracking
    status: PlanItemStatus = PlanItemStatus.PENDING
    error: Optional[str] = None  # Error message if failed
    file_path: Optional[Path] = None  # Output file path (if applicable)

    # Timestamps
    created_at: float = field(default_factory=time.time)
    started_at: Optional[float] = None
    completed_at: Optional[float] = None

    # Progress tracking
    progress: float = 0.0  # Progress percentage (0.0 to 1.0)
    
    # Thread safety lock (not serialized)
    _lock: threading.Lock = field(default_factory=threading.Lock, init=False, compare=False, repr=False)

    def to_dict(self) -> Dict[str, Any]:
        """
        Convert plan item to dictionary for serialization.

        Returns:
            Dictionary representation
        """
        # Manually build dict to exclude _lock (cannot be pickled)
        data = {
            "item_id": self.item_id,
            "item_type": self.item_type.value,
            "spotify_id": self.spotify_id,
            "spotify_url": self.spotify_url,
            "parent_id": self.parent_id,
            "name": self.name,
            "metadata": self.metadata.copy() if self.metadata else {},
            "file_path": str(self.file_path) if self.file_path else None,
            "status": self.status.value,
            "error": self.error,
            "child_ids": self.child_ids.copy(),
            "created_at": self.created_at,
            "started_at": self.started_at,
            "completed_at": self.completed_at,
            "progress": self.progress,
        }
        return data

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "PlanItem":
        """
        Create plan item from dictionary.

        Args:
            data: Dictionary representation

        Returns:
            PlanItem instance

        Raises:
            ValueError: If required fields are missing or invalid
        """
        # Validate required fields
        required_fields = ["item_id", "item_type"]
        for field in required_fields:
            if field not in data:
                raise ValueError(f"Missing required field: {field}")
        
        # Validate and convert enum values with better error messages
        try:
            item_type = PlanItemType(data["item_type"])
        except ValueError:
            valid_types = [e.value for e in PlanItemType]
            raise ValueError(
                f"Invalid item_type: {data['item_type']}. "
                f"Must be one of: {valid_types}"
            ) from None
        
        # Convert string paths back to Path objects
        if data.get("file_path"):
            data["file_path"] = Path(data["file_path"])
        
        # Convert enum strings back to enums
        data["item_type"] = item_type
        
        # Use default PENDING status if status is not provided
        if "status" in data:
            try:
                data["status"] = PlanItemStatus(data["status"])
            except ValueError:
                valid_statuses = [e.value for e in PlanItemStatus]
                raise ValueError(
                    f"Invalid status: {data['status']}. "
                    f"Must be one of: {valid_statuses}"
                ) from None
        
        return cls(**data)

    def mark_started(self) -> None:
        """Mark item as started."""
        with self._lock:
            self.status = PlanItemStatus.IN_PROGRESS
            self.started_at = time.time()

    def mark_completed(self, file_path: Optional[Path] = None, progress: Optional[float] = None) -> None:
        """
        Mark item as completed.

        Args:
            file_path: Optional file path where item was saved
            progress: Optional progress value (0.0 to 1.0). Defaults to 1.0 for full completion.
        """
        with self._lock:
            self.status = PlanItemStatus.COMPLETED
            self.completed_at = time.time()
            self.progress = progress if progress is not None else 1.0
            if file_path:
                self.file_path = file_path

    def mark_failed(self, error: str) -> None:
        """
        Mark item as failed.

        Args:
            error: Error message
        """
        with self._lock:
            self.status = PlanItemStatus.FAILED
            self.error = error
            self.completed_at = time.time()

    def mark_skipped(self, reason: Optional[str] = None) -> None:
        """
        Mark item as skipped.

        Args:
            reason: Optional reason for skipping
        """
        with self._lock:
            self.status = PlanItemStatus.SKIPPED
            self.completed_at = time.time()
            if reason:
                self.error = reason


@dataclass
class DownloadPlan:
    """
    Complete download plan containing all items to process.

    The plan is generated from configuration, optimized (deduplicated, sorted),
    and then executed.
    """

    items: List[PlanItem] = field(default_factory=list)
    created_at: float = field(default_factory=time.time)
    metadata: Dict[str, Any] = field(default_factory=dict)

    def get_item(self, item_id: str) -> Optional[PlanItem]:
        """
        Get item by ID.

        Args:
            item_id: Item identifier

        Returns:
            PlanItem or None if not found
        """
        for item in self.items:
            if item.item_id == item_id:
                return item
        return None

    def get_items_by_type(self, item_type: PlanItemType) -> List[PlanItem]:
        """
        Get all items of a specific type.

        Args:
            item_type: Type to filter by

        Returns:
            List of matching items
        """
        return [item for item in self.items if item.item_type == item_type]

    def get_items_by_status(self, status: PlanItemStatus) -> List[PlanItem]:
        """
        Get all items with a specific status.

        Args:
            status: Status to filter by

        Returns:
            List of matching items
        """
        return [item for item in self.items if item.status == status]

    def get_statistics(self) -> Dict[str, Any]:
        """
        Get plan statistics.

        Returns:
            Dictionary with statistics
        """
        total = len(self.items)
        by_status = {
            status.value: len(self.get_items_by_status(status))
            for status in PlanItemStatus
        }
        by_type = {
            item_type.value: len(self.get_items_by_type(item_type))
            for item_type in PlanItemType
        }

        return {
            "total_items": total,
            "by_status": by_status,
            "by_type": by_type,
            "created_at": self.created_at,
        }

    def to_dict(self) -> Dict[str, Any]:
        """
        Convert plan to dictionary for serialization.

        Returns:
            Dictionary representation
        """
        return {
            "items": [item.to_dict() for item in self.items],
            "created_at": self.created_at,
            "metadata": self.metadata,
        }

    @classmethod
    def from_dict(cls, data: Dict[str, Any]) -> "DownloadPlan":
        """
        Create plan from dictionary.

        Args:
            data: Dictionary representation

        Returns:
            DownloadPlan instance
        """
        items = [PlanItem.from_dict(item_data) for item_data in data.get("items", [])]
        return cls(
            items=items,
            created_at=data.get("created_at", time.time()),
            metadata=data.get("metadata", {}),
        )

    def save(self, file_path: Path) -> None:
        """
        Save plan to JSON file.

        Args:
            file_path: Path to save plan
        """
        with open(file_path, "w", encoding="utf-8") as f:
            json.dump(self.to_dict(), f, indent=2)

    @classmethod
    def load(cls, file_path: Path) -> "DownloadPlan":
        """
        Load plan from JSON file.

        Args:
            file_path: Path to load plan from

        Returns:
            DownloadPlan instance

        Raises:
            FileNotFoundError: If file doesn't exist
            ValueError: If file is invalid JSON
            IOError: If file cannot be read
        """
        try:
            with open(file_path, "r", encoding="utf-8") as f:
                data = json.load(f)
            return cls.from_dict(data)
        except FileNotFoundError:
            raise FileNotFoundError(f"Plan file not found: {file_path}")
        except json.JSONDecodeError as e:
            raise ValueError(f"Invalid JSON in plan file {file_path}: {e}")
        except IOError as e:
            raise IOError(f"Error reading plan file {file_path}: {e}")

