"""
Metadata embedding using mutagen.
"""

import logging
from pathlib import Path
from typing import Dict, Optional

import requests
from mutagen import File
from mutagen.id3 import APIC, ID3, TALB, TDRC, TIT2, TPE1, TPE2, TRCK, TYER, WOAS

from core.exceptions import MetadataError
from core.models import Song

logger = logging.getLogger(__name__)


class MetadataEmbedder:
    """Metadata embedding using mutagen."""

    def embed(
        self, file_path: Path, song: Song, cover_url: Optional[str] = None
    ) -> None:
        """
        Embed metadata into audio file.

        Args:
            file_path: Path to audio file
            song: Song metadata
            cover_url: Optional cover art URL (uses song.cover_url if not provided)
        """
        if not file_path.exists():
            raise MetadataError(f"File not found: {file_path}")

        cover_url = cover_url or song.cover_url
        file_ext = file_path.suffix[1:].lower()

        try:
            if file_ext == "mp3":
                self._embed_mp3(file_path, song, cover_url)
            elif file_ext in ["flac", "ogg", "opus"]:
                self._embed_vorbis(file_path, song, cover_url)
            elif file_ext == "m4a":
                self._embed_m4a(file_path, song, cover_url)
            else:
                logger.warning(f"Unsupported format for metadata: {file_ext}")
        except Exception as e:
            raise MetadataError(f"Failed to embed metadata: {e}") from e

    def _embed_mp3(self, file_path: Path, song: Song, cover_url: Optional[str]) -> None:
        """Embed metadata in MP3 file."""
        try:
            audio_file = ID3(str(file_path))
        except Exception:
            audio_file = ID3()

        # Basic tags
        audio_file["TIT2"] = TIT2(encoding=3, text=song.title)
        audio_file["TPE1"] = TPE1(encoding=3, text=song.artist)
        if song.album:
            audio_file["TALB"] = TALB(encoding=3, text=song.album)
        if song.album_artist:
            audio_file["TPE2"] = TPE2(encoding=3, text=song.album_artist)

        # Track number
        if song.track_number:
            track_str = f"{song.track_number}"
            if song.tracks_count:
                track_str += f"/{song.tracks_count}"
            audio_file["TRCK"] = TRCK(encoding=3, text=track_str)

        # Date/Year
        if song.date:
            audio_file["TDRC"] = TDRC(encoding=3, text=song.date)
        elif song.year:
            audio_file["TYER"] = TYER(encoding=3, text=str(song.year))

        # Spotify URL
        if song.spotify_url:
            audio_file["WOAS"] = WOAS(encoding=3, url=song.spotify_url)

        # Cover art
        if cover_url:
            self._embed_cover_mp3(audio_file, cover_url)

        # Save with filename - required when ID3() was created without filename
        audio_file.save(str(file_path), v2_version=3)

    def _embed_vorbis(
        self, file_path: Path, song: Song, cover_url: Optional[str]
    ) -> None:
        """Embed metadata in FLAC/OGG/Opus files."""
        audio_file = File(str(file_path))

        if audio_file is None:
            raise MetadataError(f"Unable to load file: {file_path}")

        # Basic tags
        audio_file["title"] = song.title
        audio_file["artist"] = song.artist
        if song.album:
            audio_file["album"] = song.album
        if song.album_artist:
            audio_file["albumartist"] = song.album_artist

        # Track number
        if song.track_number:
            audio_file["tracknumber"] = str(song.track_number)
        if song.tracks_count:
            audio_file["tracktotal"] = str(song.tracks_count)

        # Date
        if song.date:
            audio_file["date"] = song.date
        elif song.year:
            audio_file["year"] = str(song.year)

        # Spotify URL
        if song.spotify_url:
            audio_file["woas"] = song.spotify_url

        # Cover art (for FLAC)
        if cover_url and file_path.suffix.lower() == ".flac":
            self._embed_cover_flac(audio_file, cover_url)

        audio_file.save()

    def _embed_m4a(self, file_path: Path, song: Song, cover_url: Optional[str]) -> None:
        """Embed metadata in M4A file."""
        audio_file = File(str(file_path))

        if audio_file is None:
            raise MetadataError(f"Unable to load file: {file_path}")

        # Basic tags
        audio_file["\xa9nam"] = song.title  # Title
        audio_file["\xa9ART"] = song.artist  # Artist
        if song.album:
            audio_file["\xa9alb"] = song.album  # Album
        if song.album_artist:
            audio_file["aART"] = song.album_artist  # Album Artist

        # Track number
        if song.track_number:
            track_tuple = (song.track_number, song.tracks_count or 0)
            audio_file["trkn"] = [track_tuple]

        # Date
        if song.date:
            audio_file["\xa9day"] = song.date

        # Cover art
        if cover_url:
            self._embed_cover_m4a(audio_file, cover_url)

        audio_file.save()

    def _embed_cover_mp3(self, audio_file: ID3, cover_url: str) -> None:
        """Embed cover art in MP3 file."""
        try:
            cover_data = requests.get(cover_url, timeout=10).content
            if "APIC" in audio_file:
                del audio_file["APIC"]
            audio_file["APIC"] = APIC(
                encoding=3,
                mime="image/jpeg",
                type=3,
                desc="Cover",
                data=cover_data,
            )
        except Exception as e:
            logger.warning(f"Failed to embed cover art: {e}")

    def _embed_cover_flac(self, audio_file: File, cover_url: str) -> None:
        """Embed cover art in FLAC file."""
        try:
            from mutagen.flac import Picture

            cover_data = requests.get(cover_url, timeout=10).content
            picture = Picture()
            picture.type = 3
            picture.desc = "Cover"
            picture.mime = "image/jpeg"
            picture.data = cover_data

            if audio_file.pictures:
                audio_file.clear_pictures()
            audio_file.add_picture(picture)
        except Exception as e:
            logger.warning(f"Failed to embed cover art: {e}")

    def _embed_cover_m4a(self, audio_file: File, cover_url: str) -> None:
        """Embed cover art in M4A file."""
        try:
            from mutagen.mp4 import MP4Cover

            cover_data = requests.get(cover_url, timeout=10).content
            if "covr" in audio_file:
                del audio_file["covr"]
            audio_file["covr"] = [
                MP4Cover(cover_data, imageformat=MP4Cover.FORMAT_JPEG)
            ]
        except Exception as e:
            logger.warning(f"Failed to embed cover art: {e}")

