"""
Plan executor for executing download plan with parallel processing.

This module executes the download plan, handling parallel downloads,
progress tracking, and error recovery.
"""

import logging
import signal
import threading
import time
from concurrent.futures import ThreadPoolExecutor, as_completed
from pathlib import Path
from typing import Callable, Dict, List, Optional

from core.downloader import Downloader, format_filename, spotify_track_to_song
from core.plan import DownloadPlan, PlanItem, PlanItemStatus, PlanItemType
from core.utils import get_plan_path

logger = logging.getLogger(__name__)


class PlanExecutor:
    """
    Executes download plan with optimized parallel processing and progress tracking.

    Parallelization Strategy:
    - Uses ThreadPoolExecutor for concurrent track downloads
    - All pending tracks are submitted to the thread pool simultaneously
    - Thread-safe status updates using locks to prevent race conditions
    - Graceful shutdown handling with progress saving on interruption
    - Container items (albums, artists, playlists) are processed sequentially
      after all tracks complete (they depend on track completion status)

    Performance Optimizations:
    - Parallel execution of all track downloads (limited by max_workers)
    - Thread-safe caching (file existence, audio search, Spotify API) reduces
      redundant I/O operations across threads
    - Atomic status updates prevent race conditions in multi-threaded environment
    - Efficient progress tracking with minimal locking overhead

    The executor:
    - Processes tracks in parallel using ThreadPoolExecutor
    - Updates item status atomically (thread-safe)
    - Handles errors gracefully (per-item, doesn't stop entire plan)
    - Provides detailed progress tracking
    - Creates M3U files after playlist tracks complete
    - Supports graceful shutdown with progress persistence
    """

    def __init__(self, downloader: Downloader, max_workers: Optional[int] = None):
        """
        Initialize plan executor.

        Args:
            downloader: Downloader instance for actual downloads
            max_workers: Maximum number of parallel workers (default: config.threads)
        """
        self.downloader = downloader
        self.max_workers = max_workers or downloader.config.threads
        self.lock = threading.Lock()  # For thread-safe status updates
        self.progress_callback: Optional[Callable[[PlanItem], None]] = None
        self._shutdown_requested = threading.Event()  # Flag for graceful shutdown
        self._current_plan: Optional[DownloadPlan] = None  # Track current plan for saving
        self._signal_handlers_registered = False  # Track if signal handlers are registered
        self._original_sigint = None  # Store original SIGINT handler
        self._original_sigterm = None  # Store original SIGTERM handler

    def execute(
        self,
        plan: DownloadPlan,
        progress_callback: Optional[Callable[[PlanItem], None]] = None,
    ) -> Dict[str, int]:
        """
        Execute download plan.

        Args:
            plan: DownloadPlan to execute
            progress_callback: Optional callback for progress updates

        Returns:
            Dictionary with execution statistics
        """
        self.progress_callback = progress_callback
        self._current_plan = plan
        self._shutdown_requested.clear()

        # Register signal handlers for graceful shutdown (only once, and only in main thread)
        if not self._signal_handlers_registered:
            # Signal handlers can only be registered in the main thread
            if threading.current_thread() == threading.main_thread():
                def signal_handler(signum, frame):
                    """Handle shutdown signals."""
                    logger.warning(f"Received signal {signum}, initiating graceful shutdown...")
                    self._shutdown_requested.set()

                try:
                    # Store original handlers for cleanup
                    self._original_sigint = signal.signal(signal.SIGINT, signal_handler)
                    self._original_sigterm = signal.signal(signal.SIGTERM, signal_handler)
                    self._signal_handlers_registered = True
                    logger.debug("Registered signal handlers for graceful shutdown")
                except ValueError as e:
                    # Should not happen if we checked main thread, but handle gracefully
                    logger.warning(f"Cannot register signal handlers: {e}")
            else:
                logger.debug(
                    "Skipping signal handler registration (not in main thread). "
                    "Graceful shutdown via signals will not be available."
                )

        logger.info(f"Starting plan execution with {self.max_workers} workers...")
        start_time = time.time()

        # Get items to execute (pending tracks only - containers processed after)
        track_items = [
            item
            for item in plan.items
            if item.item_type == PlanItemType.TRACK
            and item.status == PlanItemStatus.PENDING
        ]

        logger.info(f"Executing {len(track_items)} tracks with {self.max_workers} parallel workers")

        # Execute tracks in parallel using ThreadPoolExecutor
        # All tracks are submitted simultaneously, with execution limited by max_workers
        # This maximizes parallelism while respecting resource constraints
        try:
            with ThreadPoolExecutor(max_workers=self.max_workers) as executor:
                # Submit all track downloads to the thread pool
                # Each track download benefits from shared caches (thread-safe):
                # - Spotify API cache (reduces redundant API calls)
                # - Audio search cache (reduces redundant YouTube searches)
                # - File existence cache (reduces redundant filesystem checks)
                futures = {
                    executor.submit(self._execute_track, item, plan): item
                    for item in track_items
                }

                # Process completed futures
                for future in as_completed(futures):
                    # Check for shutdown request
                    if self._shutdown_requested.is_set():
                        logger.warning("Shutdown requested, cancelling remaining tasks...")
                        # Cancel pending futures
                        for f in futures:
                            f.cancel()
                        break
                    
                    item = futures[future]
                    try:
                        future.result()  # Will raise if there was an exception
                    except Exception as e:
                        logger.error(f"Unexpected error executing {item.name}: {e}")
        except KeyboardInterrupt:
            logger.warning("Interrupted by user, initiating graceful shutdown...")
            self._shutdown_requested.set()

        # Process containers and M3U files after tracks complete (if not shutting down)
        # Note: Containers are processed sequentially after parallel track execution
        # because they depend on the final status of their child tracks
        if not self._shutdown_requested.is_set():
            self._process_containers(plan)
            self._process_m3u_files(plan)
            # Update containers again after M3U processing to reflect final M3U status
            # (needed for playlists with no tracks, where M3U is the only child)
            self._process_containers(plan)
        else:
            logger.warning("Skipping container and M3U processing due to shutdown")

        elapsed = time.time() - start_time
        stats = self._get_execution_stats(plan)

        if self._shutdown_requested.is_set():
            logger.warning(
                f"Plan execution interrupted after {elapsed:.1f}s: "
                f"{stats['completed']} completed, "
                f"{stats['failed']} failed, "
                f"{stats['pending']} pending"
            )
            # Save plan progress on shutdown
            try:
                plan_path = get_plan_path() / "download_plan_progress.json"
                plan.save(plan_path)
                logger.info(f"Saved plan progress to {plan_path}")
            except Exception as e:
                logger.error(f"Failed to save plan progress: {e}")
        else:
            logger.info(
                f"Plan execution complete in {elapsed:.1f}s: "
                f"{stats['completed']} completed, "
                f"{stats['failed']} failed"
            )

        return stats

    def _execute_track(self, item: PlanItem, plan: DownloadPlan) -> None:
        """
        Execute a single track item.

        Args:
            item: Track item to execute
            plan: DownloadPlan (for accessing parent items if needed)
        """
        item.mark_started()
        self._notify_progress(item)

        try:
            if not item.spotify_url:
                raise ValueError("Missing Spotify URL for track")

            # Download track
            success, file_path = self.downloader.download_track(item.spotify_url)

            if success and file_path:
                item.mark_completed(file_path)
                logger.debug(f"Completed: {item.name}")
            else:
                item.mark_failed("Download returned failure")
                logger.warning(f"Failed: {item.name}")

        except Exception as e:
            item.mark_failed(str(e))
            logger.error(f"Error downloading {item.name}: {e}")

        self._notify_progress(item)

    def _process_containers(self, plan: DownloadPlan) -> None:
        """
        Process container items (albums, artists, playlists).

        Updates container status based on child items.

        Args:
            plan: DownloadPlan to process
        """
        # Process albums
        album_items = plan.get_items_by_type(PlanItemType.ALBUM)
        for album_item in album_items:
            self._update_container_status(album_item, plan)

        # Process artists
        artist_items = plan.get_items_by_type(PlanItemType.ARTIST)
        for artist_item in artist_items:
            self._update_container_status(artist_item, plan)

        # Process playlists
        playlist_items = plan.get_items_by_type(PlanItemType.PLAYLIST)
        for playlist_item in playlist_items:
            self._update_container_status(playlist_item, plan)

    def _update_container_status(self, container_item: PlanItem, plan: DownloadPlan) -> None:
        """
        Update container item status based on child items.

        Args:
            container_item: Container item to update
            plan: DownloadPlan for accessing child items
        """
        if not container_item.child_ids:
            # Empty containers are marked as failed
            container_item.mark_failed("Container has no child items")
            return

        # Snapshot child items AND their statuses atomically to prevent race conditions
        with self.lock:
            child_items = [
                plan.get_item(child_id) for child_id in container_item.child_ids
            ]
            child_items = [item for item in child_items if item]  # Remove None
            # Snapshot statuses while holding lock to prevent race conditions
            child_statuses = {
                item.item_id: item.status for item in child_items
            }

        if not child_items:
            # All child references are invalid - mark as failed
            container_item.mark_failed("All child item references are invalid")
            return

        # Filter to only TRACK items for status calculation
        # (M3U items are processed separately and shouldn't affect container status)
        track_items = [
            item for item in child_items if item.item_type == PlanItemType.TRACK
        ]

        if not track_items:
            # No track items - check status of all children (e.g., M3U items)
            # Use snapshot statuses to avoid race conditions
            completed_count = sum(
                1 for item in child_items 
                if child_statuses.get(item.item_id) == PlanItemStatus.COMPLETED
            )
            failed_count = sum(
                1 for item in child_items 
                if child_statuses.get(item.item_id) == PlanItemStatus.FAILED
            )
            
            if completed_count == len(child_items):
                container_item.mark_completed()
            elif failed_count > 0:
                # If any child failed and none are pending, mark container as failed
                pending_count = sum(
                    1 for item in child_items 
                    if child_statuses.get(item.item_id) == PlanItemStatus.PENDING
                )
                if pending_count == 0:
                    container_item.mark_failed(
                        f"{failed_count} of {len(child_items)} child items failed"
                    )
            # If there are pending children, leave container as pending
            # (will be updated after M3U processing completes)
            return

        # Count track statuses using snapshot to avoid race conditions
        with self.lock:
            # Re-snapshot statuses in case they changed
            track_statuses = {
                item.item_id: item.status for item in track_items
            }
            
            completed = sum(
                1
                for item in track_items
                if track_statuses.get(item.item_id) == PlanItemStatus.COMPLETED
            )
            failed = sum(
                1 for item in track_items 
                if track_statuses.get(item.item_id) == PlanItemStatus.FAILED
            )
            skipped = sum(
                1 for item in track_items 
                if track_statuses.get(item.item_id) == PlanItemStatus.SKIPPED
            )
            total = len(track_items)

            # Update container status atomically
            if total == 0:
                container_item.mark_completed()
            elif completed == total:
                container_item.mark_completed()
            elif completed + skipped == total:
                # All completed or skipped (no failures)
                container_item.mark_completed()
            elif failed > 0:
                # Some child items failed - mark container as failed
                # (even if some completed/skipped, failures indicate partial failure)
                container_item.mark_failed(
                    f"{failed} of {total} child items failed "
                    f"({completed} completed, {skipped} skipped)"
                )
            elif completed > 0 or skipped > 0:
                # Partial completion but no failures
                # Check if there are pending/in_progress children
                pending_count = sum(
                    1 for item in track_items
                    if track_statuses.get(item.item_id) in [
                        PlanItemStatus.PENDING,
                        PlanItemStatus.IN_PROGRESS,
                    ]
                )
                if pending_count > 0:
                    # Some children are still pending - don't mark as completed
                    # Update progress and status to IN_PROGRESS if currently PENDING
                    progress_value = (completed + skipped) / total if total > 0 else 0.0
                    with container_item._lock:
                        container_item.progress = progress_value
                        # Mark as IN_PROGRESS if currently PENDING (indicates work in progress)
                        if container_item.status == PlanItemStatus.PENDING:
                            container_item.status = PlanItemStatus.IN_PROGRESS
                            container_item.started_at = container_item.started_at or time.time()
                    logger.debug(
                        f"Container {container_item.name} has {pending_count} pending children, "
                        f"progress: {progress_value:.2%}"
                    )
                else:
                    # All items are processed (completed/skipped) - mark as completed
                    # This shouldn't happen if logic is correct, but handle gracefully
                    progress_value = (completed + skipped) / total if total > 0 else 0.0
                    container_item.mark_completed(progress=progress_value)
            else:
                # All items are PENDING or IN_PROGRESS (not yet processed)
                # Don't mark as failed - leave status as PENDING or IN_PROGRESS
                # This can occur when loading a plan with unprocessed items or during execution
                pending_count = sum(
                    1 for item in track_items
                    if track_statuses.get(item.item_id) in [
                        PlanItemStatus.PENDING,
                        PlanItemStatus.IN_PROGRESS,
                    ]
                )
                if pending_count > 0:
                    # Items are still pending - leave container status unchanged
                    # (will be updated once items are processed)
                    logger.debug(
                        f"Container {container_item.name} has {pending_count} pending child items, "
                        "leaving status unchanged"
                    )
                else:
                    # Unexpected state - all items are in an unknown status
                    # This shouldn't happen, but handle gracefully
                    logger.warning(
                        f"Container {container_item.name} has {total} child items "
                        "in unexpected state (not completed, failed, skipped, pending, or in_progress)"
                    )
                    # Leave status unchanged rather than incorrectly marking as failed

    def _process_m3u_files(self, plan: DownloadPlan) -> None:
        """
        Create M3U files for playlists and albums after all tracks are processed.

        Args:
            plan: DownloadPlan to process
        """
        m3u_items = plan.get_items_by_type(PlanItemType.M3U)

        for m3u_item in m3u_items:
            if m3u_item.status != PlanItemStatus.PENDING:
                continue  # Already processed

            m3u_item.mark_started()
            self._notify_progress(m3u_item)

            try:
                # Get parent container (playlist or album)
                if not m3u_item.parent_id:
                    raise ValueError("M3U item missing parent container ID")

                container_item = plan.get_item(m3u_item.parent_id)
                if not container_item:
                    raise ValueError(f"Parent container not found: {m3u_item.parent_id}")

                # Get all track items for this container
                track_items = []
                for child_id in container_item.child_ids:
                    item = plan.get_item(child_id)
                    if item and item.item_type == PlanItemType.TRACK:
                        track_items.append(item)

                # Filter to completed or skipped tracks with file paths
                # (Skipped tracks have valid file paths from previous downloads)
                available_tracks = [
                    (item, item.file_path)
                    for item in track_items
                    if item.status in [PlanItemStatus.COMPLETED, PlanItemStatus.SKIPPED]
                    and item.file_path
                    and item.file_path.exists()
                ]

                if not available_tracks:
                    m3u_item.mark_failed("No available tracks to include in M3U")
                    self._notify_progress(m3u_item)
                    continue

                # Create M3U file
                # For playlists, use playlist_name from metadata; for albums, use album_name
                container_name = (
                    m3u_item.metadata.get("playlist_name")
                    or m3u_item.metadata.get("album_name")
                    or container_item.name
                )
                m3u_path = self._create_m3u_file(container_name, available_tracks)

                m3u_item.mark_completed(m3u_path)
                logger.info(f"Created M3U file: {m3u_path}")

            except Exception as e:
                m3u_item.mark_failed(str(e))
                logger.error(f"Error creating M3U file for {m3u_item.name}: {e}")

            self._notify_progress(m3u_item)

    def _get_base_output_directory(self) -> Path:
        """
        Get base output directory from config template.

        Returns:
            Base output directory Path

        Raises:
            ValueError: If output template is empty or invalid
            IOError: If directory cannot be created
        """
        output_template = self.downloader.config.output
        
        # Validate output template
        if not output_template or not output_template.strip():
            raise ValueError("Output template cannot be empty")
        
        # Extract base directory from template (everything before first {)
        if "{" in output_template:
            base_dir_str = output_template.split("{", 1)[0].rstrip("/")
        else:
            # No placeholders, use current directory
            base_dir_str = "."
        
        try:
            base_dir = Path(base_dir_str).resolve()
            base_dir.mkdir(parents=True, exist_ok=True)
            return base_dir
        except (PermissionError, OSError) as e:
            raise IOError(f"Cannot create base output directory '{base_dir_str}': {e}") from e

    def _create_m3u_file(
        self, playlist_name: str, tracks: List[tuple]
    ) -> Path:
        """
        Create M3U playlist file in base output directory.

        Args:
            playlist_name: Name of the playlist
            tracks: List of (item, file_path) tuples

        Returns:
            Path to created M3U file

        Raises:
            IOError: If file cannot be written
        """
        from core.downloader import _sanitize
        import uuid

        playlist_name_safe = _sanitize(playlist_name)
        
        # Get base output directory (may raise IOError)
        try:
            base_dir = self._get_base_output_directory()
        except (ValueError, IOError) as e:
            raise IOError(f"Cannot create base output directory for M3U file: {e}") from e
        
        m3u_path = base_dir / f"{playlist_name_safe}.m3u"
        
        # Handle name collisions by appending number
        counter = 1
        while m3u_path.exists():
            m3u_path = base_dir / f"{playlist_name_safe}_{counter}.m3u"
            counter += 1
            # Safety check to prevent excessive iterations
            if counter > 100:
                logger.warning(
                    f"Too many M3U file collisions for {playlist_name}, using UUID"
                )
                m3u_path = base_dir / f"{playlist_name_safe}_{uuid.uuid4().hex[:8]}.m3u"
                break

        try:
            with open(m3u_path, "w", encoding="utf-8") as f:
                f.write("#EXTM3U\n")
                for item, file_path in tracks:
                    # Extract title from filename
                    title = file_path.stem
                    f.write(f"#EXTINF:-1,{title}\n")
                    f.write(f"{file_path.absolute()}\n")
        except (PermissionError, OSError) as e:
            raise IOError(f"Cannot write M3U file '{m3u_path}': {e}") from e

        return m3u_path

    def _notify_progress(self, item: PlanItem) -> None:
        """
        Notify progress callback if set.

        Args:
            item: Item that was updated
        """
        if self.progress_callback:
            try:
                self.progress_callback(item)
            except Exception as e:
                logger.debug(f"Error in progress callback: {e}")

    def _get_execution_stats(self, plan: DownloadPlan) -> Dict[str, int]:
        """
        Get execution statistics.

        Excludes SKIPPED items from statistics since they require no updates.

        Args:
            plan: DownloadPlan to analyze

        Returns:
            Dictionary with statistics for TRACK items only (excluding SKIPPED)
        """
        # Get statistics for TRACK items only (exclude containers and M3U)
        # Filter out SKIPPED items - they don't need updates and shouldn't be counted
        track_items = [
            item for item in plan.get_items_by_type(PlanItemType.TRACK)
            if item.status != PlanItemStatus.SKIPPED
        ]
        
        completed = sum(1 for item in track_items if item.status == PlanItemStatus.COMPLETED)
        failed = sum(1 for item in track_items if item.status == PlanItemStatus.FAILED)
        pending = sum(1 for item in track_items if item.status == PlanItemStatus.PENDING)
        in_progress = sum(1 for item in track_items if item.status == PlanItemStatus.IN_PROGRESS)
        
        return {
            "completed": completed,
            "failed": failed,
            "pending": pending,
            "in_progress": in_progress,
            "total": len(track_items),
        }

    def cleanup(self) -> None:
        """
        Clean up signal handlers and restore original handlers.
        
        Should be called when executor is no longer needed, especially
        in library contexts where signal handling should be restored.
        
        Note: Signal handler restoration can only be done from the main thread.
        If called from a worker thread, the restoration will be skipped.
        """
        if self._signal_handlers_registered:
            # Signal handlers can only be restored in the main thread
            if threading.current_thread() == threading.main_thread():
                try:
                    if self._original_sigint is not None:
                        signal.signal(signal.SIGINT, self._original_sigint)
                    if self._original_sigterm is not None:
                        signal.signal(signal.SIGTERM, self._original_sigterm)
                    self._signal_handlers_registered = False
                    logger.debug("Restored original signal handlers")
                except (ValueError, Exception) as e:
                    logger.warning(f"Error restoring signal handlers: {e}")
            else:
                logger.debug(
                    "Skipping signal handler restoration (not in main thread). "
                    "Handlers will remain registered until main thread cleanup."
                )
                # Mark as not registered to prevent repeated warnings
                # (but don't actually restore since we can't from this thread)
                self._signal_handlers_registered = False

    def __del__(self) -> None:
        """Cleanup on destruction."""
        try:
            self.cleanup()
        except Exception:
            pass  # Ignore errors during cleanup in destructor

