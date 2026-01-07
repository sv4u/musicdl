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
from core.utils import get_plan_path

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
    Orchestrate all downloads using plan-based architecture.

    Args:
        config: MusicDLConfig instance

    Returns:
        Dictionary with download statistics
    """
    return _process_downloads_plan(config)


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
            plan_path = get_plan_path() / "download_plan.json"
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
            plan_path = get_plan_path() / "download_plan_optimized.json"
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
        # Include both completed and skipped as success
        results = {
            "songs": {
                "success": stats["completed"] + stats["skipped"],
                "failed": stats["failed"],
            },
            "artists": {"success": 0, "failed": 0},
            "playlists": {"success": 0, "failed": 0},
            "albums": {"success": 0, "failed": 0},
        }

        # Count by type for better reporting
        plan_stats = plan.get_statistics()
        logger.info(f"Plan execution complete: {plan_stats}")

        # Print cache statistics
        print_cache_stats(downloader, spotify_client)

        return results
    else:
        # Plan generation/execution disabled, return empty results
        logger.warning("Plan execution disabled, no downloads performed")
        return {
            "songs": {"success": 0, "failed": 0},
            "artists": {"success": 0, "failed": 0},
            "playlists": {"success": 0, "failed": 0},
            "albums": {"success": 0, "failed": 0},
        }




def print_cache_stats(downloader: Downloader, spotify_client: SpotifyClient = None) -> None:
    """
    Print cache statistics for all caches.

    Args:
        downloader: Downloader instance with caches
        spotify_client: Optional SpotifyClient instance (for plan-based architecture)
    """
    logger.info("")
    logger.info("=" * 80)
    logger.info("CACHE STATISTICS")
    logger.info("=" * 80)

    # Spotify API cache
    # Use the separate spotify_client (created for plan generation)
    spotify_cache = None
    if spotify_client and hasattr(spotify_client, "cache"):
        spotify_cache = spotify_client.cache
    elif hasattr(downloader.spotify, "cache"):
        spotify_cache = downloader.spotify.cache

    if spotify_cache:
        spotify_stats = spotify_cache.stats()
        logger.info("Spotify API Cache:")
        logger.info(f"  Size: {spotify_stats['size']}/{spotify_stats['max_size']} entries")
        logger.info(f"  TTL: {spotify_stats['ttl_seconds']}s ({spotify_stats['ttl_seconds'] // 3600}h)")
        logger.info(f"  Hits: {spotify_stats['hits']}, Misses: {spotify_stats['misses']}")
        logger.info(f"  Hit Rate: {spotify_stats['hit_rate']}")

    # Audio search cache
    if hasattr(downloader.audio, "search_cache"):
        audio_stats = downloader.audio.search_cache.stats()
        logger.info("Audio Search Cache:")
        logger.info(f"  Size: {audio_stats['size']}/{audio_stats['max_size']} entries")
        logger.info(f"  TTL: {audio_stats['ttl_seconds']}s ({audio_stats['ttl_seconds'] // 3600}h)")
        logger.info(f"  Hits: {audio_stats['hits']}, Misses: {audio_stats['misses']}")
        logger.info(f"  Hit Rate: {audio_stats['hit_rate']}")

    # File existence cache
    if hasattr(downloader, "file_existence_cache"):
        file_stats = downloader.file_existence_cache.stats()
        logger.info("File Existence Cache:")
        logger.info(f"  Size: {file_stats['size']}/{file_stats['max_size']} entries")
        logger.info(f"  TTL: {file_stats['ttl_seconds']}s ({file_stats['ttl_seconds'] // 3600}h)")
        logger.info(f"  Hits: {file_stats['hits']}, Misses: {file_stats['misses']}")
        logger.info(f"  Hit Rate: {file_stats['hit_rate']}")

    logger.info("=" * 80)
    logger.info("")


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

    # Add rate limiting arguments
    parser.add_argument(
        "--download-rate-limit-enabled",
        type=lambda x: x.lower() in ("true", "1", "yes"),
        default=None,
        help="Enable download rate limiting (overrides config file)",
    )
    parser.add_argument(
        "--download-rate-limit-requests",
        type=int,
        default=None,
        help="Maximum requests per window (overrides config file)",
    )
    parser.add_argument(
        "--download-rate-limit-window",
        type=float,
        default=None,
        help="Window size in seconds (overrides config file)",
    )
    parser.add_argument(
        "--download-bandwidth-limit",
        type=int,
        default=None,
        help="Bandwidth limit in bytes per second (overrides config file, 0 = unlimited)",
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

    # Apply CLI overrides for rate limiting
    if args.download_rate_limit_enabled is not None:
        config.download.download_rate_limit_enabled = args.download_rate_limit_enabled
    if args.download_rate_limit_requests is not None:
        config.download.download_rate_limit_requests = args.download_rate_limit_requests
    if args.download_rate_limit_window is not None:
        config.download.download_rate_limit_window = args.download_rate_limit_window
    if args.download_bandwidth_limit is not None:
        # 0 means unlimited (None)
        config.download.download_bandwidth_limit = (
            None if args.download_bandwidth_limit == 0 else args.download_bandwidth_limit
        )

    # Setup logging based on config (if log_level is available)
    if hasattr(config.download, "log_level"):
        setup_logging(config.download.log_level)

    logger.info("Starting download process...")
    logger.info("Architecture: Plan-based (parallel execution)")
    logger.info(f"Threads: {config.download.threads}")
    logger.info(f"Max retries: {config.download.max_retries}")
    logger.info(f"Format: {config.download.format}")
    logger.info(f"Bitrate: {config.download.bitrate}")
    if config.download.download_rate_limit_enabled:
        logger.info(
            f"Download rate limiting: {config.download.download_rate_limit_requests} "
            f"requests per {config.download.download_rate_limit_window}s"
        )
        if config.download.download_bandwidth_limit:
            bandwidth_mb = config.download.download_bandwidth_limit / (1024 * 1024)
            logger.info(f"Bandwidth limit: {bandwidth_mb:.2f} MB/s")
        else:
            logger.info("Bandwidth limit: unlimited")
    else:
        logger.info("Download rate limiting: disabled")

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
        # Note: The executor's graceful shutdown handler will automatically
        # save plan progress to download_plan_progress.json
        sys.exit(130)
    except Exception as e:
        logger.error(f"Unexpected error: {e}", exc_info=True)
        sys.exit(1)


if __name__ == "__main__":
    main()
