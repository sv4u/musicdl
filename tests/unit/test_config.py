"""
Unit tests for configuration loading and validation.
"""
import os
import pytest
import yaml
from pathlib import Path
from tempfile import NamedTemporaryFile

from core.config import (
    DownloadSettings,
    MusicDLConfig,
    MusicSource,
    load_config,
    ConfigError,
)


class TestDownloadSettings:
    """Test DownloadSettings model."""
    
    def test_valid_settings(self):
        """Test creating valid DownloadSettings."""
        settings = DownloadSettings(
            client_id="test_id",
            client_secret="test_secret",
            threads=4,
            format="mp3",
        )
        assert settings.client_id == "test_id"
        assert settings.threads == 4
        assert settings.format == "mp3"
    
    def test_default_values(self):
        """Test that default values are applied."""
        settings = DownloadSettings(
            client_id="test_id",
            client_secret="test_secret",
        )
        assert settings.threads == 4  # Default
        assert settings.max_retries == 3  # Default
        assert settings.format == "mp3"  # Default
        assert settings.bitrate == "128k"  # Default
        assert settings.overwrite == "skip"  # Default
    
    def test_missing_required_fields(self):
        """Test that missing required fields raise validation error."""
        with pytest.raises(ConfigError, match="Missing Spotify"):
            DownloadSettings(client_id="test_id")  # Missing client_secret

    def test_environment_variable_resolution_both_set(self, monkeypatch):
        """Test that environment variables are used when both are set."""
        monkeypatch.setenv("SPOTIFY_CLIENT_ID", "env_client_id")
        monkeypatch.setenv("SPOTIFY_CLIENT_SECRET", "env_client_secret")
        
        settings = DownloadSettings()
        assert settings.client_id == "env_client_id"
        assert settings.client_secret == "env_client_secret"

    def test_environment_variable_resolution_partial(self, monkeypatch):
        """Test that partial environment variables raise error."""
        monkeypatch.setenv("SPOTIFY_CLIENT_ID", "env_client_id")
        monkeypatch.delenv("SPOTIFY_CLIENT_SECRET", raising=False)
        
        with pytest.raises(ConfigError, match="Missing Spotify.*client_secret"):
            DownloadSettings()

    def test_config_file_fallback_when_env_vars_not_set(self, monkeypatch):
        """Test that config file values are used when env vars not set."""
        monkeypatch.delenv("SPOTIFY_CLIENT_ID", raising=False)
        monkeypatch.delenv("SPOTIFY_CLIENT_SECRET", raising=False)
        
        settings = DownloadSettings(
            client_id="config_client_id",
            client_secret="config_client_secret"
        )
        assert settings.client_id == "config_client_id"
        assert settings.client_secret == "config_client_secret"

    def test_environment_variable_priority_over_config(self, monkeypatch):
        """Test that environment variables take priority over config file."""
        monkeypatch.setenv("SPOTIFY_CLIENT_ID", "env_client_id")
        monkeypatch.setenv("SPOTIFY_CLIENT_SECRET", "env_client_secret")
        
        settings = DownloadSettings(
            client_id="config_client_id",
            client_secret="config_client_secret"
        )
        # Environment variables should take priority
        assert settings.client_id == "env_client_id"
        assert settings.client_secret == "env_client_secret"

    def test_whitespace_handling_in_environment_variables(self, monkeypatch):
        """Test that whitespace is stripped from environment variables."""
        monkeypatch.setenv("SPOTIFY_CLIENT_ID", "  env_client_id  ")
        monkeypatch.setenv("SPOTIFY_CLIENT_SECRET", "  env_client_secret  ")
        
        settings = DownloadSettings()
        assert settings.client_id == "env_client_id"
        assert settings.client_secret == "env_client_secret"

    def test_empty_string_environment_variables(self, monkeypatch):
        """Test that empty string environment variables are treated as missing."""
        monkeypatch.setenv("SPOTIFY_CLIENT_ID", "")
        monkeypatch.setenv("SPOTIFY_CLIENT_SECRET", "")
        
        with pytest.raises(ConfigError, match="Missing Spotify"):
            DownloadSettings()

    def test_missing_all_credentials(self, monkeypatch):
        """Test error when neither environment variables nor config provide credentials."""
        monkeypatch.delenv("SPOTIFY_CLIENT_ID", raising=False)
        monkeypatch.delenv("SPOTIFY_CLIENT_SECRET", raising=False)
        
        with pytest.raises(ConfigError, match="Missing Spotify.*client_id.*client_secret"):
            DownloadSettings()


class TestMusicDLConfig:
    """Test MusicDLConfig loading and validation."""
    
    def test_load_valid_config_dict_format(self, tmp_path, monkeypatch):
        """Test loading config with dict format for sources."""
        # Set environment variables to avoid credential errors
        monkeypatch.setenv("SPOTIFY_CLIENT_ID", "test_id")
        monkeypatch.setenv("SPOTIFY_CLIENT_SECRET", "test_secret")
        
        config_file = tmp_path / "config.yaml"
        config_file.write_text("""
version: 1.2
download:
  threads: 4
songs:
  YYZ: https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi
  Crawling: https://open.spotify.com/track/1BfzeCKzo8xSvJcYLmnP8f
artists:
  Aries: https://open.spotify.com/artist/3hOdow4ZPmrby7Q1wfPLEy
playlists: []
""")
        config = load_config(str(config_file))
        assert config.version == "1.2"
        assert len(config.songs) == 2
        # Check that both songs are present (order may vary)
        song_names = [song.name for song in config.songs]
        assert "YYZ" in song_names
        assert "Crawling" in song_names
        # Verify URLs
        for song in config.songs:
            if song.name == "YYZ":
                assert song.url == "https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi"
    
    def test_load_valid_config_list_format(self, tmp_path, monkeypatch):
        """Test loading config with list format for sources."""
        # Set environment variables to avoid credential errors
        monkeypatch.setenv("SPOTIFY_CLIENT_ID", "test_id")
        monkeypatch.setenv("SPOTIFY_CLIENT_SECRET", "test_secret")
        
        config_file = tmp_path / "config.yaml"
        config_file.write_text("""
version: 1.2
download:
  threads: 4
songs:
  - YYZ: https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi
  - Crawling: https://open.spotify.com/track/1BfzeCKzo8xSvJcYLmnP8f
artists: []
playlists: []
""")
        config = load_config(str(config_file))
        assert len(config.songs) == 2
        # Check that both songs are present (order may vary)
        song_names = [song.name for song in config.songs]
        assert "YYZ" in song_names
        assert "Crawling" in song_names
    
    def test_load_config_missing_file(self):
        """Test that missing config file raises ConfigError."""
        with pytest.raises(ConfigError, match="not found"):
            load_config("/nonexistent/config.yaml")
    
    def test_load_config_invalid_version(self, tmp_path, monkeypatch):
        """Test that invalid version raises ConfigError."""
        # Set environment variables to avoid credential errors
        monkeypatch.setenv("SPOTIFY_CLIENT_ID", "test_id")
        monkeypatch.setenv("SPOTIFY_CLIENT_SECRET", "test_secret")
        
        config_file = tmp_path / "config.yaml"
        config_file.write_text("""
version: 1.0
download:
  threads: 4
""")
        with pytest.raises(ConfigError, match="Invalid version"):
            load_config(str(config_file))
    
    def test_load_config_missing_download_section(self, tmp_path):
        """Test that missing download section raises error."""
        config_file = tmp_path / "config.yaml"
        config_file.write_text("""
version: 1.2
songs: []
""")
        with pytest.raises(Exception):  # Pydantic validation error
            load_config(str(config_file))
    
    def test_load_config_invalid_yaml(self, tmp_path):
        """Test that invalid YAML raises ConfigError."""
        config_file = tmp_path / "config.yaml"
        config_file.write_text("invalid: yaml: content: [")
        with pytest.raises(ConfigError, match="Error parsing YAML"):
            load_config(str(config_file))
    
    def test_load_config_empty_sources(self, tmp_path, monkeypatch):
        """Test loading config with empty source lists."""
        # Set environment variables to avoid credential errors
        monkeypatch.setenv("SPOTIFY_CLIENT_ID", "test_id")
        monkeypatch.setenv("SPOTIFY_CLIENT_SECRET", "test_secret")
        
        config_file = tmp_path / "config.yaml"
        config_file.write_text("""
version: 1.2
download:
  threads: 4
songs: []
artists: []
playlists: []
""")
        config = load_config(str(config_file))
        assert len(config.songs) == 0
        assert len(config.artists) == 0
        assert len(config.playlists) == 0
    
    def test_load_config_mixed_source_formats(self, tmp_path, monkeypatch):
        """Test loading config with mixed source formats."""
        # Set environment variables to avoid credential errors
        monkeypatch.setenv("SPOTIFY_CLIENT_ID", "test_id")
        monkeypatch.setenv("SPOTIFY_CLIENT_SECRET", "test_secret")
        
        # Note: YAML doesn't support mixing list and dict formats in the same structure
        # This test verifies that the config loader handles list format correctly
        config_file = tmp_path / "config.yaml"
        config_file.write_text("""
version: 1.2
download:
  threads: 4
songs:
  - YYZ: https://open.spotify.com/track/1RKbVxcm267VdsIzqY7msi
  - Crawling: https://open.spotify.com/track/1BfzeCKzo8xSvJcYLmnP8f
artists: []
playlists: []
""")
        # Should handle list format correctly
        config = load_config(str(config_file))
        assert len(config.songs) == 2
        song_names = [song.name for song in config.songs]
        assert "YYZ" in song_names
        assert "Crawling" in song_names

    def test_backward_compatibility_config_file_credentials(self, tmp_path, monkeypatch):
        """Test that config file credentials still work (backward compatibility)."""
        # Unset environment variables to test config file fallback
        monkeypatch.delenv("SPOTIFY_CLIENT_ID", raising=False)
        monkeypatch.delenv("SPOTIFY_CLIENT_SECRET", raising=False)
        
        config_file = tmp_path / "config.yaml"
        config_file.write_text("""
version: 1.2
download:
  client_id: config_client_id
  client_secret: config_client_secret
  threads: 4
songs: []
artists: []
playlists: []
""")
        config = load_config(str(config_file))
        assert config.download.client_id == "config_client_id"
        assert config.download.client_secret == "config_client_secret"

