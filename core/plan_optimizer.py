"""
Plan optimizer for removing duplicates and optimizing download order.

This module optimizes the download plan by:
- Removing duplicate items (by Spotify ID)
- Checking file existence and marking as skipped
- Sorting items for optimal download order
"""

import logging
from collections import defaultdict
from pathlib import Path
from typing import Dict, Optional, Set

from core.downloader import format_filename, spotify_track_to_song
from core.models import Song
from core.plan import DownloadPlan, PlanItem, PlanItemStatus, PlanItemType
from core.spotify_client import SpotifyClient

logger = logging.getLogger(__name__)


class PlanOptimizer:
    """
    Optimizes download plan by removing duplicates and sorting items.

    The optimizer ensures:
    - No duplicate tracks (by Spotify ID)
    - Files that already exist are marked as skipped (if overwrite == "skip")
    - Items are sorted for optimal download order
    """

    def __init__(
        self,
        config,
        spotify_client: SpotifyClient,
        check_file_existence: bool = True,
    ):
        """
        Initialize plan optimizer.

        Args:
            config: DownloadSettings configuration
            spotify_client: SpotifyClient for fetching metadata
            check_file_existence: Whether to check if files already exist
        """
        self.config = config
        self.spotify = spotify_client
        self.check_file_existence = check_file_existence

    def optimize(self, plan: DownloadPlan) -> DownloadPlan:
        """
        Optimize download plan.

        Args:
            plan: DownloadPlan to optimize

        Returns:
            Optimized DownloadPlan (modifies plan in place, also returns it)

        Raises:
            ValueError: If plan is None
        """
        if plan is None:
            raise ValueError("Plan cannot be None")
        
        if not plan.items:
            logger.warning("Optimizing empty plan")
            return plan
        
        logger.info("Optimizing download plan...")

        # Step 1: Remove duplicates
        self._remove_duplicates(plan)

        # Step 2: Check file existence (if enabled)
        if self.check_file_existence:
            self._check_existing_files(plan)

        # Step 3: Sort items for optimal order
        self._sort_items(plan)

        stats = plan.get_statistics()
        logger.info(
            f"Plan optimization complete: {stats['total_items']} items "
            f"({stats['by_status']['pending']} pending, "
            f"{stats['by_status']['skipped']} skipped, "
            f"{stats['by_status']['failed']} failed)"
        )

        return plan

    def _remove_duplicates(self, plan: DownloadPlan) -> None:
        """
        Remove duplicate items by Spotify ID, keeping first occurrence.

        Args:
            plan: DownloadPlan to deduplicate
        """
        # Map (item_type, spotify_id) -> item_id of first occurrence
        seen_ids: Dict[PlanItemType, Dict[str, str]] = defaultdict(dict)
        items_to_remove: Set[str] = set()

        for item in plan.items:
            if not item.spotify_id:
                continue  # Skip items without Spotify ID

            # Track items by type
            if item.spotify_id in seen_ids[item.item_type]:
                # Duplicate found - mark for removal
                items_to_remove.add(item.item_id)
                original_item_id = seen_ids[item.item_type][item.spotify_id]
                logger.debug(f"Removing duplicate {item.item_type.value}: {item.name} ({item.spotify_id})")

                # Update parent's child_ids if this item had a parent
                if item.parent_id:
                    parent = plan.get_item(item.parent_id)
                    if parent and item.item_id in parent.child_ids:
                        parent.child_ids.remove(item.item_id)
                        # Add reference to original item so parent can track it
                        # (important for M3U generation and progress tracking)
                        if original_item_id not in parent.child_ids:
                            parent.child_ids.append(original_item_id)
            else:
                # First occurrence - store item_id for later reference
                seen_ids[item.item_type][item.spotify_id] = item.item_id

        # Remove duplicate items
        plan.items = [item for item in plan.items if item.item_id not in items_to_remove]

        if items_to_remove:
            logger.info(f"Removed {len(items_to_remove)} duplicate items")

    def _check_existing_files(self, plan: DownloadPlan) -> None:
        """
        Check if files already exist and mark as skipped if overwrite == "skip".
        If overwrite == "metadata", leave items as PENDING so they can be processed for metadata updates.

        Args:
            plan: DownloadPlan to check
        """
        if self.config.overwrite not in ["skip", "metadata"]:
            return  # Only check if we're skipping or updating metadata for existing files

        logger.info("Checking for existing files...")
        skipped_count = 0
        metadata_update_count = 0

        # Only check track items (they're the ones that create files)
        track_items = plan.get_items_by_type(PlanItemType.TRACK)

        for item in track_items:
            if item.status != PlanItemStatus.PENDING:
                continue  # Skip items that are already processed

            try:
                # Fetch track and album metadata to build file path
                if not item.spotify_id:
                    continue

                track_data = self.spotify.get_track(item.spotify_id)
                album_id = track_data.get("album", {}).get("id")
                if not album_id:
                    continue

                album_data = self.spotify.get_album(album_id)
                song = spotify_track_to_song(track_data, album_data)

                # Build file path
                filename = format_filename(
                    self.config.output, song, self.config.format
                )
                file_path = Path(filename)

                # Check if file exists
                if file_path.exists():
                    if self.config.overwrite == "skip":
                        # Mark as skipped - no processing needed
                        item.mark_skipped("File already exists")
                        item.file_path = file_path
                        skipped_count += 1
                        logger.debug(f"File exists, skipping: {file_path}")
                    elif self.config.overwrite == "metadata":
                        # Leave as PENDING - will be processed for metadata update only
                        item.file_path = file_path
                        metadata_update_count += 1
                        logger.debug(f"File exists, will update metadata: {file_path}")

            except Exception as e:
                # If we can't check, leave item as pending
                logger.debug(f"Could not check file existence for {item.name}: {e}")

        if skipped_count > 0:
            logger.info(f"Marked {skipped_count} items as skipped (files already exist)")
        if metadata_update_count > 0:
            logger.info(f"Found {metadata_update_count} items that need metadata updates")

    def _sort_items(self, plan: DownloadPlan) -> None:
        """
        Sort items for optimal download order.

        Order:
        1. Tracks (leaf nodes, can be downloaded immediately)
        2. Albums (containers, processed after tracks)
        3. Artists (containers, processed after albums)
        4. Playlists (containers, processed after tracks)
        5. M3U files (created after all playlist tracks complete)

        Args:
            plan: DownloadPlan to sort
        """
        # Define sort order by type
        type_order = {
            PlanItemType.TRACK: 0,
            PlanItemType.ALBUM: 1,
            PlanItemType.ARTIST: 2,
            PlanItemType.PLAYLIST: 3,
            PlanItemType.M3U: 4,
        }

        def sort_key(item: PlanItem) -> tuple:
            """
            Sort key for items.

            Returns:
                Tuple for sorting: (type_order, name)
            """
            return (type_order.get(item.item_type, 99), item.name)

        plan.items.sort(key=sort_key)
        logger.debug("Sorted plan items for optimal download order")

