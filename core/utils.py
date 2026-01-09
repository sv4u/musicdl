"""
Shared utility functions for musicdl.

This module provides common utility functions used across the codebase.
"""

import os
import logging
from pathlib import Path
from typing import Optional

logger = logging.getLogger(__name__)


def get_plan_path() -> Path:
    """
    Get the plan file directory path from environment variable or default.

    Reads the MUSICDL_PLAN_PATH environment variable and returns a Path object
    for the plan directory. If the environment variable is not set, defaults to
    `/var/lib/musicdl/plans`. The directory is created if it doesn't exist.

    Returns:
        Path object pointing to the plan directory

    Raises:
        OSError: If the directory cannot be created or accessed
    """
    plan_path_str = os.getenv("MUSICDL_PLAN_PATH", "/var/lib/musicdl/plans")
    plan_path = Path(plan_path_str)

    try:
        # Create directory if it doesn't exist (with parents if needed)
        plan_path.mkdir(parents=True, exist_ok=True)
        logger.debug(f"Plan directory: {plan_path}")
    except OSError as e:
        logger.error(f"Failed to create plan directory {plan_path}: {e}")
        raise

    return plan_path


def get_log_path() -> Path:
    """
    Get the log file path from environment variable or default.

    Reads the MUSICDL_LOG_PATH environment variable and returns a Path object
    for the log file. If the environment variable is not set, defaults to
    `/var/lib/musicdl/logs/musicdl.log`. The directory is created if it doesn't exist.

    Returns:
        Path object pointing to the log file

    Raises:
        OSError: If the directory cannot be created or accessed, or if the file is not writable
    """
    log_path_str = os.getenv("MUSICDL_LOG_PATH", "/var/lib/musicdl/logs/musicdl.log")
    log_path = Path(log_path_str)

    try:
        # Create directory if it doesn't exist (with parents if needed)
        log_path.parent.mkdir(parents=True, exist_ok=True)
        
        # Test file writability by creating a temporary test file
        try:
            test_file = log_path.parent / ".musicdl_write_test"
            test_file.touch()
            test_file.unlink()
        except (OSError, PermissionError) as e:
            logger.error(f"Log directory {log_path.parent} is not writable: {e}")
            raise OSError(f"Cannot write to log directory {log_path.parent}: {e}") from e
        
        logger.debug(f"Log file: {log_path}")
    except OSError as e:
        logger.error(f"Failed to create log directory {log_path.parent}: {e}")
        raise

    return log_path

