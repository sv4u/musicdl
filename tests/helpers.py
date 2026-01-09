"""
Test helper functions and utilities.
"""
import json
import time
from datetime import datetime
from pathlib import Path
from typing import Dict, Any, List, Optional
from unittest.mock import Mock, MagicMock

from core.models import Song
from core.plan import DownloadPlan, PlanItem, PlanItemStatus, PlanItemType


def create_mock_spotify_response(response_type: str, **kwargs) -> Dict[str, Any]:
    """
    Create mock Spotify API response.
    
    Args:
        response_type: Type of response (track, album, playlist, artist_albums)
        **kwargs: Override values for the response
    
    Returns:
        Mock response dictionary
    """
    base_responses = {
        "track": {
            "id": "1RKbVxcm267VdsIzqY7msi",
            "name": "YYZ",
            "artists": [{"name": "Rush"}],
            "album": {"id": "77CZUF57sYqgtznUe3OikQ", "name": "Moving Pictures (40th Anniversary Super Deluxe)"},
            "duration_ms": 266000,
            "track_number": 3,
            "disc_number": 1,
            "external_urls": {"spotify": "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi"},
        },
        "album": {
            "id": "77CZUF57sYqgtznUe3OikQ",
            "name": "I Love My Computer",
            "artists": [{"name": "Ninajirachi"}],
            "total_tracks": 12,
            "tracks": {"items": [], "next": None},
        },
        "playlist": {
            "id": "5Xrt7Y1mwD4q107Ty56xnn",
            "name": "planet namek",
            "tracks": {"items": [], "next": None},
        },
        "artist_albums": [],
    }
    
    response = base_responses.get(response_type, {}).copy()
    response.update(kwargs)
    return response


def create_sample_song(**kwargs) -> Song:
    """Create sample Song with optional overrides."""
    defaults = {
        "title": "YYZ",
        "artist": "Rush",
        "album": "Moving Pictures (40th Anniversary Super Deluxe)",
        "track_number": 3,
        "duration": 266,
        "spotify_url": "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi",
    }
    defaults.update(kwargs)
    return Song(**defaults)


def assert_song_metadata(file_path: Path, expected_song: Song):
    """
    Assert that audio file has correct metadata.
    
    Args:
        file_path: Path to audio file
        expected_song: Expected song metadata
    """
    from mutagen import File
    
    audio_file = File(str(file_path))
    assert audio_file is not None
    
    # Verify basic tags based on format
    if file_path.suffix.lower() == ".mp3":
        assert audio_file.get("TIT2")[0].text[0] == expected_song.title
        assert audio_file.get("TPE1")[0].text[0] == expected_song.artist
    elif file_path.suffix.lower() in [".flac", ".ogg", ".opus"]:
        assert audio_file.get("title")[0] == expected_song.title
        assert audio_file.get("artist")[0] == expected_song.artist
    elif file_path.suffix.lower() == ".m4a":
        assert audio_file.get("\xa9nam")[0] == expected_song.title
        assert audio_file.get("\xa9ART")[0] == expected_song.artist


def create_test_audio_file(path: Path, format: str = "mp3") -> Path:
    """
    Create a minimal test audio file.
    
    Args:
        path: Path where file should be created
        format: Audio format (mp3, flac, m4a)
    
    Returns:
        Path to created file
    """
    # Create minimal valid audio file content
    # In real tests, use actual audio file bytes or generate minimal valid files
    test_content = b"fake audio content"
    file_path = path.with_suffix(f".{format}")
    file_path.write_bytes(test_content)
    return file_path


def verify_file_structure(output_dir: Path, expected_files: List[str]):
    """
    Verify that expected files exist in output directory.
    
    Args:
        output_dir: Output directory to check
        expected_files: List of expected file paths (relative to output_dir)
    """
    for file_path in expected_files:
        full_path = output_dir / file_path
        assert full_path.exists(), f"Expected file not found: {file_path}"
        assert full_path.is_file(), f"Expected path is not a file: {file_path}"


def extract_spotify_id(url_or_id: str) -> str:
    """Extract Spotify ID from URL or return ID as-is."""
    import re
    match = re.search(r"spotify\.com/\w+/([a-zA-Z0-9]+)", url_or_id)
    return match.group(1) if match else url_or_id


# Healthcheck Server Test Helpers

def create_log_entry(
    timestamp: Optional[float] = None,
    logger_name: str = "test.logger",
    level: str = "INFO",
    message: str = "Test log message",
) -> str:
    """
    Create a log entry string in the format used by the application.
    
    Format: "YYYY-MM-DD HH:MM:SS - logger.name - LEVEL - message"
    
    Args:
        timestamp: Unix timestamp (defaults to current time)
        logger_name: Logger name
        level: Log level (DEBUG, INFO, WARNING, ERROR, CRITICAL)
        message: Log message
    
    Returns:
        Formatted log entry string
    """
    if timestamp is None:
        timestamp = time.time()
    
    dt = datetime.fromtimestamp(timestamp)
    timestamp_str = dt.strftime("%Y-%m-%d %H:%M:%S")
    
    return f"{timestamp_str} - {logger_name} - {level} - {message}"


def create_log_file(
    log_path: Path,
    entries: List[Dict[str, Any]],
    append: bool = False,
) -> None:
    """
    Create a log file with the specified entries.
    
    Args:
        log_path: Path to log file
        entries: List of log entry dictionaries with keys: timestamp, logger, level, message
        append: If True, append to existing file; if False, overwrite
    """
    mode = "a" if append else "w"
    with open(log_path, mode, encoding="utf-8") as f:
        for entry in entries:
            log_line = create_log_entry(
                timestamp=entry.get("timestamp"),
                logger_name=entry.get("logger", "test.logger"),
                level=entry.get("level", "INFO"),
                message=entry.get("message", ""),
            )
            f.write(log_line + "\n")


def create_sample_log_entries(
    count: int = 10,
    start_timestamp: Optional[float] = None,
    interval_seconds: int = 1,
    levels: Optional[List[str]] = None,
) -> List[Dict[str, Any]]:
    """
    Create sample log entries for testing.
    
    Args:
        count: Number of log entries to create
        start_timestamp: Starting timestamp (defaults to current time - count * interval)
        interval_seconds: Seconds between each log entry
        levels: List of log levels to cycle through (defaults to all levels)
    
    Returns:
        List of log entry dictionaries
    """
    if levels is None:
        levels = ["DEBUG", "INFO", "WARNING", "ERROR", "CRITICAL"]
    
    if start_timestamp is None:
        start_timestamp = time.time() - (count * interval_seconds)
    
    entries = []
    loggers = ["core.downloader", "core.spotify_client", "spotipy.util", "core.plan_executor"]
    messages = [
        "Starting download",
        "Track downloaded successfully",
        "Rate limit warning",
        "Error occurred",
        "Plan generation complete",
        "Processing track",
    ]
    
    for i in range(count):
        timestamp = start_timestamp + (i * interval_seconds)
        level = levels[i % len(levels)]
        logger_name = loggers[i % len(loggers)]
        message = messages[i % len(messages)] + f" (entry {i+1})"
        
        entries.append({
            "timestamp": timestamp,
            "logger": logger_name,
            "level": level,
            "message": message,
        })
    
    return entries


def create_plan_file_with_rate_limit(
    plan_path: Path,
    items: Optional[List[PlanItem]] = None,
    retry_after_seconds: int = 3600,
    detected_at: Optional[float] = None,
) -> None:
    """
    Create a plan file with rate limit metadata.
    
    Args:
        plan_path: Path to plan file
        items: List of plan items (defaults to empty list)
        retry_after_seconds: Seconds until rate limit expires
        detected_at: Timestamp when rate limit was detected (defaults to current time)
    """
    if items is None:
        items = []
    
    if detected_at is None:
        detected_at = time.time()
    
    # Handle negative retry_after_seconds (for expired rate limits)
    # The retry_after_seconds field should always be positive (absolute value)
    # but retry_after_timestamp will be in the past if retry_after_seconds is negative
    retry_after_timestamp = detected_at + retry_after_seconds
    retry_after_seconds_abs = abs(retry_after_seconds)
    
    plan = DownloadPlan(
        items=items,
        created_at=time.time(),
        metadata={
            "rate_limit": {
                "active": True,
                "retry_after_seconds": retry_after_seconds_abs,
                "retry_after_timestamp": retry_after_timestamp,
                "detected_at": detected_at,
            },
        },
    )
    
    plan.save(plan_path)


def create_plan_file_with_phase(
    plan_path: Path,
    items: Optional[List[PlanItem]] = None,
    phase: str = "executing",
    phase_updated_at: Optional[float] = None,
) -> None:
    """
    Create a plan file with phase metadata.
    
    Args:
        plan_path: Path to plan file
        items: List of plan items (defaults to empty list)
        phase: Current phase (generating, optimizing, executing)
        phase_updated_at: Timestamp when phase was updated (defaults to current time)
    """
    if items is None:
        items = []
    
    if phase_updated_at is None:
        phase_updated_at = time.time()
    
    plan = DownloadPlan(
        items=items,
        created_at=time.time(),
        metadata={
            "phase": phase,
            "phase_updated_at": phase_updated_at,
        },
    )
    
    plan.save(plan_path)


def create_plan_file_with_items(
    plan_path: Path,
    item_count: int = 5,
    statuses: Optional[List[PlanItemStatus]] = None,
) -> DownloadPlan:
    """
    Create a plan file with multiple items in various statuses.
    
    Args:
        plan_path: Path to plan file
        item_count: Number of items to create
        statuses: List of statuses to cycle through (defaults to all statuses)
    
    Returns:
        Created DownloadPlan instance
    """
    if statuses is None:
        statuses = [
            PlanItemStatus.PENDING,
            PlanItemStatus.IN_PROGRESS,
            PlanItemStatus.COMPLETED,
            PlanItemStatus.FAILED,
            PlanItemStatus.SKIPPED,
        ]
    
    items = []
    item_types = [PlanItemType.TRACK, PlanItemType.ALBUM, PlanItemType.PLAYLIST, PlanItemType.ARTIST]
    
    for i in range(item_count):
        item = PlanItem(
            item_id=f"item_{i}",
            item_type=item_types[i % len(item_types)],
            name=f"Item {i}",
        )
        
        status = statuses[i % len(statuses)]
        if status == PlanItemStatus.COMPLETED:
            item.mark_completed()
        elif status == PlanItemStatus.FAILED:
            item.mark_failed(f"Error {i}")
        elif status == PlanItemStatus.SKIPPED:
            item.mark_skipped(f"Skipped {i}")
        elif status == PlanItemStatus.IN_PROGRESS:
            item.mark_started()
        # PENDING is the default, no action needed
        
        items.append(item)
    
    plan = DownloadPlan(items=items, created_at=time.time(), metadata={})
    plan.save(plan_path)
    return plan

