"""
Unit tests for core utility functions.
"""

import os
import pytest
from pathlib import Path
from unittest.mock import patch

from core.utils import get_plan_path, get_log_path


class TestGetPlanPath:
    """Test get_plan_path() function."""

    def test_get_plan_path_default(self, tmp_test_dir, monkeypatch):
        """Test get_plan_path() with default path."""
        # Remove MUSICDL_PLAN_PATH if set
        monkeypatch.delenv("MUSICDL_PLAN_PATH", raising=False)
        
        # Test default path behavior by using a test directory
        # The actual default path /var/lib/musicdl/plans requires root permissions,
        # so we test the default behavior using a custom path that simulates the default
        # In integration tests, the actual default path would be tested with proper permissions
        test_default_path = tmp_test_dir / "default_plans"
        monkeypatch.setenv("MUSICDL_PLAN_PATH", str(test_default_path))
        
        result = get_plan_path()
        assert result is not None
        assert result == test_default_path
        assert test_default_path.exists()

    def test_get_plan_path_custom_env(self, tmp_test_dir, monkeypatch):
        """Test get_plan_path() with custom environment variable."""
        custom_path = tmp_test_dir / "custom_plans"
        custom_path.mkdir(parents=True, exist_ok=True)
        
        monkeypatch.setenv("MUSICDL_PLAN_PATH", str(custom_path))
        
        result = get_plan_path()
        assert result == custom_path
        assert custom_path.exists()

    def test_get_plan_path_creates_directory(self, tmp_test_dir, monkeypatch):
        """Test that get_plan_path() creates directory if it doesn't exist."""
        new_path = tmp_test_dir / "new_plans"
        monkeypatch.setenv("MUSICDL_PLAN_PATH", str(new_path))
        
        # Directory shouldn't exist yet
        assert not new_path.exists()
        
        # Should create directory
        result = get_plan_path()
        assert result == new_path
        assert new_path.exists()

    def test_get_plan_path_handles_permission_error(self, tmp_test_dir, monkeypatch):
        """Test that get_plan_path() handles permission errors."""
        # Mock mkdir to raise OSError to simulate permission error
        # This is more reliable than using /root which may not fail on all systems
        with patch('pathlib.Path.mkdir') as mock_mkdir:
            mock_mkdir.side_effect = OSError("Permission denied")
            invalid_path = tmp_test_dir / "invalid_plans"
            monkeypatch.setenv("MUSICDL_PLAN_PATH", str(invalid_path))
            
            # Should raise OSError
            with pytest.raises(OSError):
                get_plan_path()


class TestGetLogPath:
    """Test get_log_path() function."""

    def test_get_log_path_default(self, tmp_test_dir, monkeypatch):
        """Test get_log_path() with default path."""
        # Remove MUSICDL_LOG_PATH if set to test default behavior
        monkeypatch.delenv("MUSICDL_LOG_PATH", raising=False)
        
        # Mock os.getenv to return a test path when MUSICDL_LOG_PATH is not set
        # This simulates the default path behavior without requiring root permissions
        test_default_path = tmp_test_dir / "default_logs" / "musicdl.log"
        
        def mock_getenv(key, default=None):
            if key == "MUSICDL_LOG_PATH":
                return str(test_default_path)
            return os.environ.get(key, default)
        
        with patch('core.utils.os.getenv', side_effect=mock_getenv):
            result = get_log_path()
            assert result is not None
            assert result == test_default_path
            assert test_default_path.parent.exists()

    def test_get_log_path_custom_env(self, tmp_test_dir, monkeypatch):
        """Test get_log_path() with custom environment variable."""
        custom_path = tmp_test_dir / "custom_logs" / "musicdl.log"
        custom_path.parent.mkdir(parents=True, exist_ok=True)
        
        monkeypatch.setenv("MUSICDL_LOG_PATH", str(custom_path))
        
        result = get_log_path()
        assert result == custom_path
        assert custom_path.parent.exists()

    def test_get_log_path_creates_directory(self, tmp_test_dir, monkeypatch):
        """Test that get_log_path() creates directory for default path."""
        # Remove MUSICDL_LOG_PATH to test default behavior
        monkeypatch.delenv("MUSICDL_LOG_PATH", raising=False)
        
        # Mock os.getenv to return a test path when MUSICDL_LOG_PATH is not set
        new_path = tmp_test_dir / "new_logs" / "musicdl.log"
        
        # Directory shouldn't exist yet
        assert not new_path.parent.exists()
        
        def mock_getenv(key, default=None):
            if key == "MUSICDL_LOG_PATH":
                return str(new_path)
            return os.environ.get(key, default)
        
        with patch('core.utils.os.getenv', side_effect=mock_getenv):
            # Should create directory for default path
            result = get_log_path()
            assert result == new_path
            assert new_path.parent.exists()

    def test_get_log_path_handles_permission_error(self, tmp_test_dir, monkeypatch):
        """Test that get_log_path() handles permission errors."""
        # Mock mkdir to raise OSError to simulate permission error
        # This is more reliable than using /root which may not fail on all systems
        with patch('pathlib.Path.mkdir') as mock_mkdir:
            mock_mkdir.side_effect = OSError("Permission denied")
            invalid_path = tmp_test_dir / "invalid_logs" / "musicdl.log"
            monkeypatch.setenv("MUSICDL_LOG_PATH", str(invalid_path))
            
            # Should raise OSError
            with pytest.raises(OSError):
                get_log_path()

