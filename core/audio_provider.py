"""
Audio download provider using yt-dlp.
"""

import logging
import tempfile
from pathlib import Path
from typing import List, Optional

import yt_dlp

from core.exceptions import DownloadError

logger = logging.getLogger(__name__)


class AudioProvider:
    """Audio download provider using yt-dlp."""

    def __init__(
        self,
        output_format: str = "mp3",
        bitrate: str = "128k",
        audio_providers: Optional[List[str]] = None,
    ):
        """
        Initialize with format and bitrate settings.

        Args:
            output_format: Output audio format (mp3, flac, m4a, opus)
            bitrate: Audio bitrate (e.g., "128k", "320k")
            audio_providers: List of providers to use (youtube, youtube-music, soundcloud)
        """
        self.output_format = output_format
        self.bitrate = bitrate
        self.audio_providers = audio_providers or ["youtube-music", "youtube"]

        # Configure yt-dlp format based on output format
        if output_format == "m4a":
            ytdl_format = "bestaudio[ext=m4a]/bestaudio/best"
        elif output_format == "opus":
            ytdl_format = "bestaudio[ext=webm]/bestaudio/best"
        else:
            ytdl_format = "bestaudio"

        # yt-dlp options
        self.ytdl_opts = {
            "format": ytdl_format,
            "quiet": True,
            "no_warnings": True,
            "encoding": "UTF-8",
            "extract_flat": False,
        }

        # Create temp directory for downloads
        self.temp_dir = Path(tempfile.gettempdir()) / "musicdl"
        self.temp_dir.mkdir(parents=True, exist_ok=True)

    def search(self, query: str) -> Optional[str]:
        """
        Search for audio URL matching query.

        Args:
            query: Search query (e.g., "Artist - Song Name")

        Returns:
            URL of best matching audio, or None if not found
        """
        # Try each provider in order
        for provider in self.audio_providers:
            try:
                url = self._search_provider(provider, query)
                if url:
                    return url
            except Exception as e:
                logger.debug(f"Provider {provider} failed: {e}")
                continue

        return None

    def _search_provider(self, provider: str, query: str) -> Optional[str]:
        """Search using a specific provider."""
        # Build search query based on provider
        # yt-dlp supports ytsearch, ytmsearch, scsearch prefixes
        if provider == "youtube-music":
            search_query = f"ytmsearch:{query}"
        elif provider == "youtube":
            search_query = f"ytsearch:{query}"
        elif provider == "soundcloud":
            search_query = f"scsearch:{query}"
        else:
            # Default to YouTube
            search_query = f"ytsearch:{query}"

        # Use yt-dlp to search
        ytdl_opts = {
            **self.ytdl_opts,
            "quiet": True,
            "extract_flat": True,
            "default_search": "extract",  # Extract from search results
        }

        try:
            with yt_dlp.YoutubeDL(ytdl_opts) as ydl:
                # Extract info for search
                info = ydl.extract_info(search_query, download=False)
                if info:
                    # Handle both single result and playlist/entries
                    if "entries" in info:
                        entries = info["entries"]
                        if entries and len(entries) > 0:
                            first_entry = entries[0]
                            # Get URL from entry
                            url = first_entry.get("url") or first_entry.get("webpage_url") or first_entry.get("id")
                            if url and not url.startswith("http"):
                                # Construct full URL if we only have ID
                                if provider in ["youtube", "youtube-music"]:
                                    url = f"https://www.youtube.com/watch?v={url}"
                            return url
                    elif "url" in info or "webpage_url" in info:
                        # Single result
                        return info.get("url") or info.get("webpage_url")
        except Exception as e:
            logger.debug(f"Search failed for {provider}: {e}")
            return None

        return None

    def download(self, url: str, output_path: Path) -> Path:
        """
        Download audio to output path.

        Args:
            url: URL of audio to download
            output_path: Path where file should be saved

        Returns:
            Path to downloaded file

        Raises:
            DownloadError: If download fails
        """
        # Ensure output directory exists
        output_path.parent.mkdir(parents=True, exist_ok=True)

        # Configure yt-dlp for download
        ytdl_opts = {
            **self.ytdl_opts,
            "outtmpl": str(output_path.with_suffix(f".%(ext)s")),
        }

        # Add postprocessor for format conversion if needed
        if self.output_format and self.bitrate != "disable":
            ytdl_opts["postprocessors"] = [
                {
                    "key": "FFmpegExtractAudio",
                    "preferredcodec": self.output_format,
                    "preferredquality": self.bitrate,
                }
            ]

        try:
            with yt_dlp.YoutubeDL(ytdl_opts) as ydl:
                ydl.download([url])

            # Return the actual file path (yt-dlp may change extension)
            # Try to find the file with the correct extension
            if output_path.exists():
                return output_path

            # Try with different extensions
            for ext in [self.output_format, "m4a", "webm", "opus"]:
                candidate = output_path.with_suffix(f".{ext}")
                if candidate.exists():
                    return candidate

            # If still not found, look for any file in the directory
            # with similar name
            pattern = output_path.stem + ".*"
            matches = list(output_path.parent.glob(pattern))
            if matches:
                return matches[0]

            raise DownloadError(f"Downloaded file not found at {output_path}")

        except Exception as e:
            raise DownloadError(f"Failed to download {url}: {e}") from e

    def get_metadata(self, url: str) -> dict:
        """
        Get metadata for a URL without downloading.

        Args:
            url: URL to get metadata for

        Returns:
            Dictionary with metadata
        """
        ytdl_opts = {
            **self.ytdl_opts,
            "quiet": True,
        }

        try:
            with yt_dlp.YoutubeDL(ytdl_opts) as ydl:
                info = ydl.extract_info(url, download=False)
                return info or {}
        except Exception as e:
            logger.debug(f"Failed to get metadata for {url}: {e}")
            return {}

