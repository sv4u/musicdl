#!/usr/bin/env python3
"""
Download music from Spotify using configuration file.

USAGE:
    python3 download.py [CONFIG]

SYNOPSIS:
    Reads a YAML configuration file for information on Spotify links
    and downloads music using the configured settings.

COMMAND LINE ARGUMENT:
    [CONFIG]      musicdl YAML configuration file containing information on
                  artists and playlists to download
"""

import argparse
import logging
import sys
from pathlib import Path
from typing import Dict

from core.config import ConfigError, load_config
from core.downloader import Downloader
from core.exceptions import DownloadError, SpotifyError
from core.plan import DownloadPlan, PlanItemStatus
from core.plan_executor import PlanExecutor
from core.plan_generator import PlanGenerator
from core.plan_optimizer import PlanOptimizer
from core.spotify_client import SpotifyClient

# Configure logging
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
)
logger = logging.getLogger(__name__)


def setup_logging(log_level: str) -> None:
    """Configure structured logging based on config."""
    level = getattr(logging, log_level.upper(), logging.INFO)
    logging.getLogger().setLevel(level)
    logger.setLevel(level)


def process_downloads(config) -> Dict[str, Dict[str, int]]:
    """
    Orchestrate all downloads.

    Uses plan-based architecture if enabled, otherwise uses sequential processing.

    Args:
        config: MusicDLConfig instance

    Returns:
        Dictionary with download statistics
    """
    if config.download.use_plan_architecture:
        return _process_downloads_plan(config)
    else:
        return _process_downloads_sequential(config)


def _process_downloads_plan(config) -> Dict[str, Dict[str, int]]:
    """
    Process downloads using plan-based architecture.

    Args:
        config: MusicDLConfig instance

    Returns:
        Dictionary with download statistics
    """
    logger.info("Using plan-based architecture")
    downloader = Downloader(config.download)

    # Create Spotify client for plan generation
    spotify_client = SpotifyClient(
        config.download.client_id,
        config.download.client_secret,
        cache_max_size=config.download.cache_max_size,
        cache_ttl=config.download.cache_ttl,
        max_retries=config.download.spotify_max_retries,
        retry_base_delay=config.download.spotify_retry_base_delay,
        retry_max_delay=config.download.spotify_retry_max_delay,
        rate_limit_enabled=config.download.spotify_rate_limit_enabled,
        rate_limit_requests=config.download.spotify_rate_limit_requests,
        rate_limit_window=config.download.spotify_rate_limit_window,
    )

    # Generate plan
    plan = None
    if config.download.plan_generation_enabled:
        generator = PlanGenerator(config, spotify_client)
        plan = generator.generate_plan()

        # Save plan if persistence enabled
        if config.download.plan_persistence_enabled:
            plan_path = Path("download_plan.json")
            try:
                plan.save(plan_path)
                logger.info(f"Saved plan to {plan_path}")
            except Exception as e:
                logger.error(f"Failed to save plan to {plan_path}: {e}")
                raise RuntimeError(f"Plan persistence failed: {e}") from e

    # Optimize plan
    if plan and config.download.plan_optimization_enabled:
        optimizer = PlanOptimizer(
            config.download,
            spotify_client,
            check_file_existence=True,
        )
        plan = optimizer.optimize(plan)

        # Save optimized plan if persistence enabled
        if config.download.plan_persistence_enabled:
            plan_path = Path("download_plan_optimized.json")
            try:
                plan.save(plan_path)
                logger.info(f"Saved optimized plan to {plan_path}")
            except Exception as e:
                logger.error(f"Failed to save optimized plan to {plan_path}: {e}")
                raise RuntimeError(f"Plan persistence failed: {e}") from e

    # Execute plan
    if plan and config.download.plan_execution_enabled:
        executor = PlanExecutor(downloader, max_workers=config.download.threads)

        # Progress callback for detailed tracking
        def progress_callback(item):
            """Progress callback for detailed tracking."""
            if item.item_type.value == "track":
                status_emoji = {
                    "pending": "⏳",
                    "in_progress": "🔄",
                    "completed": "✅",
                    "failed": "❌",
                    "skipped": "⏭️",
                }.get(item.status.value, "❓")
                logger.info(
                    f"{status_emoji} {item.name} - {item.status.value}"
                )

        stats = executor.execute(plan, progress_callback=progress_callback)

        # Convert plan stats to legacy format
        # Include both completed and skipped as success (matches sequential mode behavior
        # where existing files return (True, path) and are counted as success)
        results = {
            "songs": {
                "success": stats["completed"] + stats["skipped"],
                "failed": stats["failed"],
            },
            "artists": {"success": 0, "failed": 0},
            "playlists": {"success": 0, "failed": 0},
        }

        # Count by type for better reporting
        plan_stats = plan.get_statistics()
        logger.info(f"Plan execution complete: {plan_stats}")

        return results
    else:
        # Plan generation/execution disabled, return empty results
        logger.warning("Plan execution disabled, no downloads performed")
        return {
            "songs": {"success": 0, "failed": 0},
            "artists": {"success": 0, "failed": 0},
            "playlists": {"success": 0, "failed": 0},
        }


def _process_downloads_sequential(config) -> Dict[str, Dict[str, int]]:
    """
    Process downloads using sequential architecture (legacy).

    Args:
        config: MusicDLConfig instance

    Returns:
        Dictionary with download statistics
    """
    logger.info("Using sequential architecture (legacy)")
    downloader = Downloader(config.download)
    results = {
        "songs": {"success": 0, "failed": 0},
        "artists": {"success": 0, "failed": 0},
        "playlists": {"success": 0, "failed": 0},
    }

    # Process songs
    logger.info(f"Processing {len(config.songs)} songs...")
    for song in config.songs:
        logger.info(f"Downloading song: {song.name}")
        try:
            success, path = downloader.download_track(song.url)
            if success:
                results["songs"]["success"] += 1
                logger.info(f"Successfully downloaded: {song.name}")
            else:
                results["songs"]["failed"] += 1
                logger.error(f"Failed to download: {song.name}")
        except Exception as e:
            results["songs"]["failed"] += 1
            logger.error(f"Error downloading {song.name}: {e}")

    # Process artists
    logger.info(f"Processing {len(config.artists)} artists...")
    for artist in config.artists:
        logger.info(f"Downloading artist: {artist.name}")
        try:
            tracks = downloader.download_artist(artist.url)
            success_count = sum(1 for success, _ in tracks if success)
            failed_count = len(tracks) - success_count
            results["artists"]["success"] += success_count
            results["artists"]["failed"] += failed_count
            logger.info(
                f"Artist {artist.name}: {success_count} successful, {failed_count} failed"
            )
        except Exception as e:
            logger.error(f"Error downloading artist {artist.name}: {e}")

    # Process playlists
    logger.info(f"Processing {len(config.playlists)} playlists...")
    for playlist in config.playlists:
        logger.info(f"Downloading playlist: {playlist.name}")
        try:
            tracks = downloader.download_playlist(playlist.url, create_m3u=True)
            success_count = sum(1 for success, _ in tracks if success)
            failed_count = len(tracks) - success_count
            results["playlists"]["success"] += success_count
            results["playlists"]["failed"] += failed_count
            logger.info(
                f"Playlist {playlist.name}: {success_count} successful, {failed_count} failed"
            )
        except Exception as e:
            logger.error(f"Error downloading playlist {playlist.name}: {e}")

    return results


def print_summary(results: Dict[str, Dict[str, int]]) -> None:
    """Print download summary."""
    print("\n" + "=" * 80)
    print("DOWNLOAD SUMMARY")
    print("=" * 80)

    total_success = 0
    total_failed = 0

    for category, stats in results.items():
        success = stats["success"]
        failed = stats["failed"]
        total_success += success
        total_failed += failed
        print(f"{category.capitalize()}: {success} successful, {failed} failed")

    print("-" * 80)
    print(f"Total: {total_success} successful, {total_failed} failed")
    print("=" * 80)


def main() -> None:
    """Main entry point."""
    # Create argument parser
    parser = argparse.ArgumentParser(
        prog="download.py",
        description="Download music using a YAML configuration file.",
    )

    # Add config argument
    parser.add_argument(
        "config",
        type=str,
        help="Path to the YAML configuration file.",
    )

    # Parse arguments
    args = parser.parse_args()

    # Load configuration
    try:
        config = load_config(args.config)
        logger.info(f"Loaded configuration version {config.version}")
    except ConfigError as e:
        logger.error(f"Configuration error: {e}")
        sys.exit(1)
    except Exception as e:
        logger.error(f"Unexpected error loading config: {e}")
        sys.exit(1)

    # Setup logging based on config (if log_level is available)
    if hasattr(config.download, "log_level"):
        setup_logging(config.download.log_level)

    logger.info("Starting download process...")
    logger.info(f"Architecture: {'Plan-based' if config.download.use_plan_architecture else 'Sequential (legacy)'}")
    logger.info(f"Threads: {config.download.threads}")
    logger.info(f"Max retries: {config.download.max_retries}")
    logger.info(f"Format: {config.download.format}")
    logger.info(f"Bitrate: {config.download.bitrate}")

    # Process downloads
    try:
        results = process_downloads(config)
        print_summary(results)

        # Exit with error code if any downloads failed
        total_failed = sum(stats["failed"] for stats in results.values())
        if total_failed > 0:
            sys.exit(1)

    except KeyboardInterrupt:
        logger.warning("Download interrupted by user")
        # Note: If using plan architecture, the executor's graceful shutdown handler
        # will automatically save plan progress to download_plan_progress.json
        sys.exit(130)
    except Exception as e:
        logger.error(f"Unexpected error: {e}", exc_info=True)
        sys.exit(1)


if __name__ == "__main__":
    main()
