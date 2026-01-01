"""
Configuration models and loader.
"""

import os
import yaml
from pathlib import Path
from typing import List, Literal, Optional

from pydantic import BaseModel, Field, model_validator

from core.exceptions import ConfigError


class DownloadSettings(BaseModel):
    """Download configuration settings."""

    client_id: Optional[str] = Field(default=None)
    client_secret: Optional[str] = Field(default=None)
    threads: int = 4
    max_retries: int = 3
    format: str = "mp3"
    bitrate: str = "128k"
    output: str = "{artist}/{album}/{track-number} - {title}.{output-ext}"
    audio_providers: List[str] = Field(default_factory=lambda: ["youtube-music"])
    cache_max_size: int = 1000  # Maximum cached entries
    cache_ttl: int = 3600  # Cache TTL in seconds (1 hour)
    overwrite: Literal["skip", "overwrite", "metadata"] = "skip"
    # Rate limiting settings
    spotify_max_retries: int = 3  # Maximum retry attempts for rate-limited requests
    spotify_retry_base_delay: float = 1.0  # Base delay in seconds for exponential backoff
    spotify_retry_max_delay: float = 120.0  # Maximum delay in seconds for exponential backoff
    spotify_rate_limit_enabled: bool = True  # Enable proactive rate limiting
    spotify_rate_limit_requests: int = 10  # Maximum requests per window
    spotify_rate_limit_window: float = 1.0  # Window size in seconds

    @staticmethod
    def _resolve_credential(
        field_name: str, env_var: str, config_value: Optional[str]
    ) -> Optional[str]:
        """
        Resolve credential from environment variable or config value.

        Args:
            field_name: Name of the credential field (for error messages)
            env_var: Environment variable name
            config_value: Value from config file (may be None)

        Returns:
            Resolved credential value, or None if not found
        """
        # Try environment variable first (highest priority)
        env_value = os.getenv(env_var)
        if env_value is not None:
            env_value = env_value.strip()
            if env_value:  # Not empty after stripping
                return env_value

        # Fall back to config file value
        if config_value is not None:
            config_value = config_value.strip()
            if config_value:  # Not empty after stripping
                return config_value

        return None

    @model_validator(mode="after")
    def validate_credentials(self) -> "DownloadSettings":
        """
        Validate that both credentials are present from either environment variables or config.

        Returns:
            Self with resolved credentials

        Raises:
            ConfigError: If credentials are missing
        """
        # Resolve credentials from environment variables or config
        resolved_client_id = self._resolve_credential(
            "client_id", "SPOTIFY_CLIENT_ID", self.client_id
        )
        resolved_client_secret = self._resolve_credential(
            "client_secret", "SPOTIFY_CLIENT_SECRET", self.client_secret
        )

        # Build error message if credentials are missing
        missing = []
        if not resolved_client_id:
            missing.append("client_id")
        if not resolved_client_secret:
            missing.append("client_secret")

        if missing:
            missing_str = " and ".join(missing)
            raise ConfigError(
                f"Missing Spotify {missing_str}. Both client_id and client_secret must be provided via:\n"
                "  - Environment variables: SPOTIFY_CLIENT_ID and SPOTIFY_CLIENT_SECRET, OR\n"
                "  - Configuration file: download.client_id and download.client_secret"
            )

        # Update fields with resolved values
        self.client_id = resolved_client_id
        self.client_secret = resolved_client_secret

        return self


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

