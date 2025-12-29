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

    Args:
        config: MusicDLConfig instance

    Returns:
        Dictionary with download statistics
    """
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
        sys.exit(130)
    except Exception as e:
        logger.error(f"Unexpected error: {e}", exc_info=True)
        sys.exit(1)


if __name__ == "__main__":
    main()
