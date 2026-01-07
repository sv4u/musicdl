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

