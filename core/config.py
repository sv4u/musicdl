"""
Configuration models and loader.
"""

import yaml
from pathlib import Path
from typing import List, Literal

from pydantic import BaseModel, Field

from core.exceptions import ConfigError


class DownloadSettings(BaseModel):
    """Download configuration settings."""

    client_id: str
    client_secret: str
    threads: int = 4
    max_retries: int = 3
    format: str = "mp3"
    bitrate: str = "128k"
    output: str = "{artist}/{album}/{track-number} - {title}.{output-ext}"
    audio_providers: List[str] = Field(default_factory=lambda: ["youtube-music"])
    cache_max_size: int = 1000  # Maximum cached entries
    cache_ttl: int = 3600  # Cache TTL in seconds (1 hour)
    overwrite: Literal["skip", "overwrite", "metadata"] = "skip"


class MusicSource(BaseModel):
    """Music source entry."""

    name: str
    url: str


class MusicDLConfig(BaseModel):
    """Main configuration model."""

    version: Literal["1.2"]
    download: DownloadSettings
    songs: List[MusicSource] = Field(default_factory=list)
    artists: List[MusicSource] = Field(default_factory=list)
    playlists: List[MusicSource] = Field(default_factory=list)

    @classmethod
    def from_yaml(cls, path: str) -> "MusicDLConfig":
        """
        Load and validate configuration from YAML file.

        Args:
            path: Path to YAML configuration file

        Returns:
            MusicDLConfig instance

        Raises:
            ConfigError: If file not found or invalid
        """
        config_path = Path(path)
        if not config_path.exists():
            raise ConfigError(f"Configuration file not found: {path}")

        try:
            with open(config_path, "r", encoding="utf-8") as f:
                data = yaml.safe_load(f)
        except yaml.YAMLError as e:
            raise ConfigError(f"Error parsing YAML file: {e}") from e

        # Validate version (handle both string and float from YAML)
        version = data.get("version")
        if version != "1.2" and str(version) != "1.2":
            raise ConfigError(
                f"Invalid version: {version}. Expected 1.2"
            )
        # Normalize to string for Pydantic
        data["version"] = "1.2"

        # Convert songs/artists/playlists from dict format to list
        # Handle format: {name: url} or [{name: url}, ...]
        def convert_sources(sources):
            """Convert dict or list format to MusicSource list."""
            if not sources:
                return []
            result = []
            if isinstance(sources, list):
                for item in sources:
                    if isinstance(item, dict):
                        # Handle [{name: url}, ...]
                        for name, url in item.items():
                            result.append(MusicSource(name=name, url=url))
                    elif isinstance(item, str):
                        # Handle [url, ...] - use URL as name
                        result.append(MusicSource(name=item, url=item))
            elif isinstance(sources, dict):
                # Handle {name: url, ...}
                for name, url in sources.items():
                    result.append(MusicSource(name=name, url=url))
            return result

        # Convert sources
        if "songs" in data:
            data["songs"] = convert_sources(data["songs"])
        if "artists" in data:
            data["artists"] = convert_sources(data["artists"])
        if "playlists" in data:
            data["playlists"] = convert_sources(data["playlists"])

        try:
            return cls(**data)
        except Exception as e:
            raise ConfigError(f"Invalid configuration: {e}") from e


def load_config(config_path: str) -> MusicDLConfig:
    """
    Load configuration from YAML file.

    Args:
        config_path: Path to configuration file

    Returns:
        MusicDLConfig instance
    """
    return MusicDLConfig.from_yaml(config_path)

