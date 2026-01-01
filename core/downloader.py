"""
Main download orchestrator.
"""

import logging
import re
import time
from functools import wraps
from pathlib import Path
from typing import Callable, Dict, List, Optional, Tuple

from core.audio_provider import AudioProvider
from core.exceptions import DownloadError, MetadataError, SpotifyError
from core.metadata import MetadataEmbedder
from core.models import Song
from core.spotify_client import SpotifyClient

logger = logging.getLogger(__name__)


def retry_on_failure(max_retries: int = 3):
    """Decorator for retry logic with exponential backoff."""

    def decorator(func: Callable) -> Callable:
        @wraps(func)
        def wrapper(*args, **kwargs):
            for attempt in range(1, max_retries + 1):
                try:
                    return func(*args, **kwargs)
                except Exception as e:
                    if attempt == max_retries:
                        raise
                    wait_time = 2 ** attempt
                    logger.warning(
                        f"Attempt {attempt} failed: {e}. Retrying in {wait_time}s..."
                    )
                    time.sleep(wait_time)

        return wrapper

    return decorator


def format_filename(template: str, song: Song, file_ext: str) -> str:
    """
    Format filename from template.

    Args:
        template: Filename template with placeholders
        song: Song metadata
        file_ext: File extension (without dot)

    Returns:
        Formatted filename
    """
    # Replace placeholders
    filename = template
    filename = filename.replace("{artist}", _sanitize(song.artist))
    filename = filename.replace("{title}", _sanitize(song.title))
    filename = filename.replace("{album}", _sanitize(song.album or ""))
    filename = filename.replace(
        "{track-number}", f"{song.track_number:02d}" if song.track_number else ""
    )
    filename = filename.replace(
        "{disc-number}", str(song.disc_number) if song.disc_number else ""
    )
    filename = filename.replace(
        "{album-artist}", _sanitize(song.album_artist or song.artist)
    )
    filename = filename.replace("{year}", str(song.year) if song.year else "")
    filename = filename.replace("{date}", song.date or "")
    filename = filename.replace("{output-ext}", file_ext)

    # Clean up double slashes and trailing slashes
    filename = re.sub(r"/+", "/", filename)
    filename = filename.strip("/")

    return filename


def _sanitize(text: str) -> str:
    """Sanitize string for filename."""
    # Remove invalid characters
    text = re.sub(r'[<>:"/\\|?*]', "", text)
    # Remove leading/trailing dots and spaces
    text = text.strip(". ")
    return text


def spotify_track_to_song(track_data: Dict, album_data: Dict) -> Song:
    """
    Convert Spotify track and album data to Song model.

    Args:
        track_data: Spotify track API response
        album_data: Spotify album API response

    Returns:
        Song object
    """
    # Get best cover image
    cover_url = None
    if album_data.get("images"):
        cover_url = max(
            album_data["images"], key=lambda i: i.get("width", 0) * i.get("height", 0)
        )["url"]

    # Extract year from date
    year = None
    date = album_data.get("release_date", "")
    if date:
        year = int(date[:4]) if len(date) >= 4 else None

    # Calculate disc_count from tracks (not total_tracks)
    # Use the maximum disc_number from available tracks, matching spotDL's approach
    disc_count = track_data.get("disc_number", 1)  # Default to current track's disc number
    if album_data.get("tracks") and album_data["tracks"].get("items"):
        # Find maximum disc_number from available tracks
        max_disc = max(
            (track.get("disc_number", 1) for track in album_data["tracks"]["items"]),
            default=1
        )
        disc_count = max_disc

    return Song(
        title=track_data["name"],
        artist=track_data["artists"][0]["name"],
        album=album_data.get("name", ""),
        track_number=track_data.get("track_number", 1),
        duration=int(track_data.get("duration_ms", 0) / 1000),
        spotify_url=track_data["external_urls"]["spotify"],
        cover_url=cover_url,
        album_artist=album_data["artists"][0]["name"] if album_data.get("artists") else None,
        year=year,
        date=date,
        disc_number=track_data.get("disc_number", 1),
        disc_count=disc_count,
        tracks_count=album_data.get("total_tracks", 1),
        genre=None,  # Can be extracted from album_data["genres"] if needed
        explicit=track_data.get("explicit", False),
        isrc=track_data.get("external_ids", {}).get("isrc"),
    )


class Downloader:
    """Main download orchestrator."""

    def __init__(self, config):
        """
        Initialize with configuration.

        Args:
            config: DownloadSettings configuration object
        """
        self.config = config
        self.spotify = SpotifyClient(
            config.client_id,
            config.client_secret,
            cache_max_size=config.cache_max_size,
            cache_ttl=config.cache_ttl,
        )
        self.audio = AudioProvider(
            output_format=config.format,
            bitrate=config.bitrate,
            audio_providers=config.audio_providers,
        )
        self.metadata = MetadataEmbedder()

    def download_track(self, track_url: str) -> Tuple[bool, Optional[Path]]:
        """
        Download a single track with retry logic.

        Args:
            track_url: Spotify track URL or ID

        Returns:
            Tuple of (success, file_path)
        """
        max_retries = self.config.max_retries
        for attempt in range(1, max_retries + 1):
            try:
                # 1. Get metadata from Spotify
                logger.info(f"Fetching metadata for: {track_url}")
                track_data = self.spotify.get_track(track_url)
                album_id = track_data["album"]["id"]
                album_data = self.spotify.get_album(album_id)

                song = spotify_track_to_song(track_data, album_data)
                logger.info(f"Found: {song.artist} - {song.title}")

                # 2. Check if file already exists
                output_path = self._get_output_path(song)
                if output_path.exists() and self.config.overwrite == "skip":
                    logger.info(f"Skipping (already exists): {output_path}")
                    return True, output_path

                # 3. Search for audio using audio provider
                search_query = f"{song.artist} - {song.title}"
                logger.info(f"Searching for audio: {search_query}")
                audio_url = self.audio.search(search_query)

                if not audio_url:
                    raise DownloadError(f"No audio found for: {search_query}")

                logger.info(f"Found audio URL: {audio_url}")

                # 4. Download audio file
                logger.info(f"Downloading audio to: {output_path}")
                downloaded_path = self.audio.download(audio_url, output_path)

                # 5. Embed metadata
                logger.info("Embedding metadata...")
                self.metadata.embed(downloaded_path, song, song.cover_url)

                logger.info(f"Successfully downloaded: {downloaded_path}")
                return True, downloaded_path

            except Exception as e:
                if attempt < max_retries:
                    wait_time = 2 ** attempt
                    logger.warning(
                        f"Attempt {attempt}/{max_retries} failed for {track_url}: {e}. Retrying in {wait_time}s..."
                    )
                    time.sleep(wait_time)
                else:
                    logger.error(f"Failed to download {track_url} after {max_retries} attempts: {e}")
                    return False, None

        return False, None

    def download_album(self, album_url: str) -> List[Tuple[bool, Optional[Path]]]:
        """
        Download all tracks in an album.

        Args:
            album_url: Spotify album URL or ID

        Returns:
            List of (success, file_path) tuples
        """
        try:
            album_data = self.spotify.get_album(album_url)
            tracks = []

            # Get all tracks (handle pagination)
            items = album_data["tracks"]["items"]
            tracks_obj = album_data["tracks"]
            while tracks_obj.get("next"):
                # Get next page using spotipy's next() method
                next_data = self.spotify.client.next(tracks_obj)
                items.extend(next_data["items"])
                tracks_obj = next_data

            logger.info(f"Found {len(items)} tracks in album: {album_data['name']}")

            for track_item in items:
                track_id = track_item["id"]
                track_url = f"https://open.spotify.com/track/{track_id}"
                result = self.download_track(track_url)
                tracks.append(result)

            return tracks

        except Exception as e:
            logger.error(f"Failed to download album {album_url}: {e}")
            return [(False, None)]

    def download_playlist(
        self, playlist_url: str, create_m3u: bool = False
    ) -> List[Tuple[bool, Optional[Path]]]:
        """
        Download all tracks in a playlist.

        Args:
            playlist_url: Spotify playlist URL or ID
            create_m3u: Whether to create M3U playlist file

        Returns:
            List of (success, file_path) tuples
        """
        try:
            playlist_data = self.spotify.get_playlist(playlist_url)
            tracks = []

            # Get all tracks (handle pagination)
            # Note: playlist tracks structure is different - items contain track objects
            tracks_obj = playlist_data["tracks"]
            items = tracks_obj["items"]
            while tracks_obj.get("next"):
                next_data = self.spotify.client.next(tracks_obj)
                items.extend(next_data["items"])
                tracks_obj = next_data

            logger.info(
                f"Found {len(items)} tracks in playlist: {playlist_data['name']}"
            )

            for track_item in items:
                track = track_item.get("track")
                if not track or track.get("is_local"):
                    continue
                track_url = track["external_urls"]["spotify"]
                result = self.download_track(track_url)
                tracks.append(result)

            # Create M3U file if requested
            if create_m3u:
                self._create_m3u(playlist_data["name"], tracks)

            return tracks

        except Exception as e:
            logger.error(f"Failed to download playlist {playlist_url}: {e}")
            return [(False, None)]

    def download_artist(self, artist_url: str) -> List[Tuple[bool, Optional[Path]]]:
        """
        Download all albums and singles for an artist (discography only).
        
        Downloads the artist's discography, including:
        - Full studio albums
        - Single releases
        
        Excludes:
        - Compilation albums
        - "Appears On" albums (where artist is featured but not main artist)

        Args:
            artist_url: Spotify artist URL or ID

        Returns:
            List of (success, file_path) tuples
        """
        try:
            albums = self.spotify.get_artist_albums(artist_url)
            all_tracks = []

            logger.info(f"Found {len(albums)} albums for artist")

            for album in albums:
                album_url = album["external_urls"]["spotify"]
                logger.info(f"Downloading album: {album['name']}")
                tracks = self.download_album(album_url)
                all_tracks.extend(tracks)

            return all_tracks

        except Exception as e:
            logger.error(f"Failed to download artist {artist_url}: {e}")
            return [(False, None)]

    def _get_output_path(self, song: Song) -> Path:
        """Get output path for song based on template."""
        filename = format_filename(self.config.output, song, self.config.format)
        return Path(filename)

    def _create_m3u(self, playlist_name: str, tracks: List[Tuple[bool, Optional[Path]]]):
        """Create M3U playlist file."""
        playlist_name_safe = _sanitize(playlist_name)
        m3u_path = Path(f"{playlist_name_safe}.m3u")

        with open(m3u_path, "w", encoding="utf-8") as f:
            f.write("#EXTM3U\n")
            for success, file_path in tracks:
                if success and file_path and file_path.exists():
                    # Extract title from filename
                    title = file_path.stem
                    f.write(f"#EXTINF:-1,{title}\n")
                    f.write(f"{file_path.absolute()}\n")

        logger.info(f"Created M3U playlist: {m3u_path}")

