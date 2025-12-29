"""
Data models for musicdl.
"""

from dataclasses import dataclass
from pathlib import Path
from typing import Optional


@dataclass
class Song:
    """Song metadata model."""

    title: str
    artist: str
    album: str
    track_number: int
    duration: int
    spotify_url: str
    cover_url: Optional[str] = None
    album_artist: Optional[str] = None
    year: Optional[int] = None
    date: Optional[str] = None
    disc_number: int = 1
    disc_count: int = 1
    tracks_count: int = 1
    genre: Optional[str] = None
    explicit: bool = False
    isrc: Optional[str] = None


@dataclass
class DownloadResult:
    """Download operation result."""

    success: bool
    file_path: Optional[Path] = None
    error: Optional[str] = None
    song: Optional[Song] = None

