#!/usr/bin/env python3
"""
Healthcheck server for musicdl container.

This server provides HTTP endpoints to monitor the health and status of plan execution.
It reads plan JSON files from a configurable directory and provides:
- /health: JSON endpoint for Docker HEALTHCHECK and monitoring systems
- /status: HTML dashboard for human-readable status monitoring

The server runs in the background alongside download.py and provides real-time
status information based on plan file contents.
"""

import html
import http.server
import json
import logging
import os
import signal
import sys
import threading
import time
import urllib.parse
from datetime import datetime
from pathlib import Path
from typing import Dict, List, Optional, Tuple

# Add /scripts to path to import core modules
sys.path.insert(0, "/scripts")

try:
    from core.plan import DownloadPlan, PlanItem, PlanItemStatus, PlanItemType
    from core.utils import get_plan_path, get_log_path
except ImportError as e:
    # Fallback if running outside container
    import sys
    from pathlib import Path
    # Try to import from current directory structure
    sys.path.insert(0, str(Path(__file__).parent.parent))
    from core.plan import DownloadPlan, PlanItem, PlanItemStatus, PlanItemType
    from core.utils import get_plan_path, get_log_path

# Configure logging to stderr with structured format
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s - %(name)s - %(levelname)s - %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
    stream=sys.stderr,
)
logger = logging.getLogger(__name__)

# Plan file priority order (highest to lowest)
PLAN_FILE_PRIORITY = [
    "download_plan_progress.json",
    "download_plan_optimized.json",
    "download_plan.json",
]


def get_port() -> int:
    """
    Get healthcheck port from environment variable or default.

    Returns:
        Port number (default: 8080)
    """
    port_str = os.getenv("HEALTHCHECK_PORT", "8080")
    try:
        port = int(port_str)
        if port < 1 or port > 65535:
            raise ValueError(f"Port {port} out of range")
        return port
    except ValueError as e:
        logger.warning(f"Invalid HEALTHCHECK_PORT '{port_str}', using default 8080: {e}")
        return 8080


def load_plan_file(filepath: Path) -> Optional[Dict]:
    """
    Safely load and parse a plan JSON file.

    Handles race conditions where the file may be written while reading.
    Uses retry logic to handle partial writes during concurrent access.

    Args:
        filepath: Path to plan file

    Returns:
        Parsed JSON dictionary or None if file doesn't exist or is invalid
    """
    if not filepath.exists():
        return None

    # Retry logic for handling concurrent writes
    # Plan files are written using json.dump() which is not atomic
    max_retries = 3
    retry_delay = 0.1  # 100ms between retries

    for attempt in range(max_retries):
        try:
            with open(filepath, "r", encoding="utf-8") as f:
                data = json.load(f)
            return data
        except FileNotFoundError:
            # File was deleted between existence check and open
            logger.warning(f"Plan file {filepath} was deleted during read")
            return None
        except json.JSONDecodeError as e:
            # If this is the last attempt, log and return None
            if attempt == max_retries - 1:
                logger.warning(f"Invalid JSON in plan file {filepath} after {max_retries} attempts: {e}")
                return None
            # Otherwise, wait and retry (file may be partially written)
            time.sleep(retry_delay)
            continue
        except (IOError, PermissionError) as e:
            # IO errors are not transient, don't retry
            logger.warning(f"Cannot read plan file {filepath}: {e}")
            return None

    return None


def find_and_aggregate_plans(plan_dir: Path) -> Tuple[Optional[DownloadPlan], Optional[str]]:
    """
    Find and aggregate plan files, preferring higher priority files.

    Args:
        plan_dir: Directory containing plan files

    Returns:
        Tuple of (aggregated DownloadPlan, source file name)
        Returns (None, None) if no valid plan files found
    """
    plans_data = {}
    found_files = []

    # Load all available plan files
    for filename in PLAN_FILE_PRIORITY:
        filepath = plan_dir / filename
        try:
            data = load_plan_file(filepath)
            if data is not None:
                # Validate that data has expected structure
                if isinstance(data, dict):
                    plans_data[filename] = data
                    found_files.append(filename)
        except Exception as e:
            # Log but continue - one corrupted file shouldn't stop aggregation
            logger.warning(f"Error loading plan file {filepath}: {e}")
            continue

    if not plans_data:
        return None, None

    # Aggregate plans: prefer items from higher priority files
    # For each item_id, use the version from the highest priority file that contains it
    aggregated_items = {}
    source_file = found_files[0]  # Use highest priority file as source

    # Process files in priority order
    for filename in PLAN_FILE_PRIORITY:
        if filename not in plans_data:
            continue

        data = plans_data[filename]
        items_data = data.get("items", [])
        
        # Validate items_data is a list
        if not isinstance(items_data, list):
            logger.warning(f"Plan file {filename} has invalid items format (expected list, got {type(items_data).__name__})")
            continue

        for item_data in items_data:
            # Validate item_data is a dict
            if not isinstance(item_data, dict):
                logger.debug(f"Skipping invalid item in {filename}: not a dictionary")
                continue
            
            item_id = item_data.get("item_id")
            if item_id and isinstance(item_id, str):
                # Only add if not already present (higher priority already has it)
                if item_id not in aggregated_items:
                    aggregated_items[item_id] = item_data

    # Build aggregated plan
    if not aggregated_items:
        # No items found in any plan file
        return None, None

    # Use metadata from highest priority file
    highest_priority_data = plans_data[found_files[0]]
    
    # Validate and build aggregated plan dict
    try:
        aggregated_plan_dict = {
            "items": list(aggregated_items.values()),
            "created_at": highest_priority_data.get("created_at", time.time()),
            "metadata": highest_priority_data.get("metadata", {}),
        }
        
        # Validate created_at is a number
        if not isinstance(aggregated_plan_dict["created_at"], (int, float)):
            logger.warning(f"Invalid created_at in plan file {found_files[0]}, using current time")
            aggregated_plan_dict["created_at"] = time.time()
        
        # Validate metadata is a dict
        if not isinstance(aggregated_plan_dict["metadata"], dict):
            logger.warning(f"Invalid metadata in plan file {found_files[0]}, using empty dict")
            aggregated_plan_dict["metadata"] = {}
        
        plan = DownloadPlan.from_dict(aggregated_plan_dict)
        return plan, source_file
    except (ValueError, KeyError, TypeError, AttributeError) as e:
        logger.error(f"Failed to create plan from aggregated data: {e}", exc_info=True)
        return None, source_file


def aggregate_plan_status(plan: DownloadPlan) -> Dict[str, any]:
    """
    Calculate aggregated statistics from plan.

    Args:
        plan: DownloadPlan instance

    Returns:
        Dictionary with statistics
    """
    stats = plan.get_statistics()
    return stats


def determine_health_status(plan: Optional[DownloadPlan]) -> Tuple[str, str]:
    """
    Determine overall health status based on plan state.

    Excludes SKIPPED items from health calculations since they require no updates.
    The HTTP server health is implicit - if the server cannot respond, Docker will
    mark the container as unhealthy.

    Args:
        plan: DownloadPlan instance or None

    Returns:
        Tuple of (status, reason) where status is "healthy" or "unhealthy"
    """
    if plan is None:
        return "unhealthy", "No plan files found"

    # Check if plan is in generation or optimization phase
    phase = plan.metadata.get("phase", "unknown")
    if phase == "generating":
        return "healthy", "Plan generation in progress"
    if phase == "optimizing":
        return "healthy", "Plan optimization in progress"

    if not plan.items:
        return "unhealthy", "Plan has no items"

    # Filter out SKIPPED items - they don't need updates and shouldn't affect health
    items_to_check = [item for item in plan.items if item.status != PlanItemStatus.SKIPPED]
    
    if not items_to_check:
        # All items are skipped - this is healthy (no work needed)
        return "healthy", "All items skipped (no updates needed)"

    # Count items by status (excluding SKIPPED)
    by_status = {
        status.value: len([item for item in items_to_check if item.status == status])
        for status in PlanItemStatus
    }

    total_items = len(items_to_check)
    completed = by_status.get("completed", 0)
    failed = by_status.get("failed", 0)
    in_progress = by_status.get("in_progress", 0)
    pending = by_status.get("pending", 0)

    # Healthy conditions:
    # 1. Has items with in_progress or completed status (work is happening or done)
    # 2. At least one item is not failed
    # 3. All pending is healthy (plan ready to execute)

    if failed == total_items and total_items > 0:
        return "unhealthy", f"All {total_items} items failed"

    if in_progress > 0 or completed > 0:
        return "healthy", f"{completed} completed, {in_progress} in progress, {failed} failed"

    if pending == total_items:
        return "healthy", f"All {total_items} items pending (plan ready to execute)"

    # Mixed state with some pending and some failed (but not all failed)
    if pending > 0 and failed > 0:
        return "healthy", f"{pending} pending, {failed} failed (work in progress)"

    # Default to healthy if we can't determine (shouldn't happen)
    return "healthy", "Plan exists with items"


def format_timestamp(timestamp: float) -> str:
    """
    Format timestamp to human-readable string.

    Args:
        timestamp: Unix timestamp

    Returns:
        Formatted timestamp string
    """
    return time.strftime("%Y-%m-%d %H:%M:%S", time.localtime(timestamp))


def truncate_error(error: Optional[str], max_length: int = 200) -> str:
    """
    Truncate error message if too long.

    Args:
        error: Error message
        max_length: Maximum length

    Returns:
        Truncated error message
    """
    if not error:
        return ""
    if len(error) <= max_length:
        return error
    return error[:max_length] + "..."


def read_logs(
    log_path: Path,
    log_level: Optional[str] = None,
    search_query: Optional[str] = None,
    start_time: Optional[float] = None,
    end_time: Optional[float] = None,
    max_lines: int = 10000,
) -> List[Dict[str, any]]:
    """
    Read and filter logs from log file.

    Args:
        log_path: Path to log file
        log_level: Filter by log level (DEBUG, INFO, WARNING, ERROR, CRITICAL)
        search_query: Search query to filter log messages
        start_time: Filter logs after this timestamp (Unix timestamp)
        end_time: Filter logs before this timestamp (Unix timestamp)
        max_lines: Maximum number of lines to return (for performance)

    Returns:
        List of log entries as dictionaries with keys: timestamp, level, logger, message
    """
    if not log_path.exists():
        return []

    log_entries = []
    log_levels = {
        "DEBUG": logging.DEBUG,
        "INFO": logging.INFO,
        "WARNING": logging.WARNING,
        "ERROR": logging.ERROR,
        "CRITICAL": logging.CRITICAL,
    }
    # Safely get filter level, handling None and invalid values
    filter_level = None
    if log_level:
        try:
            filter_level = log_levels.get(log_level.upper())
        except (AttributeError, TypeError):
            # log_level is not a string or doesn't have upper() method
            filter_level = None

    try:
        # Read file in chunks from end to avoid loading entire file into memory
        # This is more memory-efficient for large log files (up to 96MB)
        with open(log_path, "rb") as f:
            # Seek to end
            f.seek(0, 2)  # 2 = SEEK_END
            file_size = f.tell()
            
            # Handle empty file
            if file_size == 0:
                return []
            
            # Safety check: warn if file is very large (over 100MB)
            # Still process it, but log a warning
            if file_size > 100 * 1024 * 1024:  # 100MB
                logger.warning(f"Log file is very large ({file_size / (1024*1024):.1f}MB), reading may be slow")
            
            # Read in chunks from end (most recent logs first)
            chunk_size = 8192  # 8KB chunks
            buffer = b""
            tail = b""  # Track tail content (after last newline) separately - only set from first chunk
            position = file_size
            lines_collected = []
            is_first_chunk = True  # Track if this is the first chunk (from end of file)
            
            # Collect lines from end of file
            while position > 0 and len(lines_collected) < max_lines:
                # Calculate how much to read
                read_size = min(chunk_size, position)
                position -= read_size
                f.seek(position)
                
                # Read chunk
                chunk = f.read(read_size)
                buffer = chunk + buffer
                
                # Process complete lines (split by newline)
                # Split on all newlines to extract all complete lines
                if b'\n' in buffer:
                    # Split into lines, keeping incomplete line in buffer
                    parts = buffer.rsplit(b'\n', 1)
                    if len(parts) == 2:
                        # parts[0] = everything before the last newline
                        # parts[1] = everything after the last newline
                        complete_lines = parts[0].split(b'\n')
                        
                        # Handle parts[1]:
                        # - If first chunk: parts[1] is the tail (content after last newline, if file doesn't end with newline)
                        # - If not first chunk: parts[1] is a complete line that should be processed
                        if is_first_chunk:
                            # First chunk: parts[1] is the tail from end of file
                            tail = parts[1]
                        else:
                            # Not first chunk: parts[1] is a complete line, process it
                            if parts[1]:
                                try:
                                    line_str = parts[1].decode('utf-8')
                                    lines_collected.append(line_str)
                                    if len(lines_collected) >= max_lines:
                                        break
                                except UnicodeDecodeError:
                                    pass
                        
                        # The first element of complete_lines might be a partial line from previous chunk
                        # Keep it in buffer for concatenation with next chunk
                        if len(complete_lines) > 1:
                            # Keep first element (might be partial) in buffer
                            buffer = complete_lines[0]
                            # Process complete lines (skip first, process rest in reverse)
                            for line_bytes in reversed(complete_lines[1:]):
                                if len(lines_collected) >= max_lines:
                                    break
                                if line_bytes:
                                    try:
                                        line_str = line_bytes.decode('utf-8')
                                        lines_collected.append(line_str)
                                    except UnicodeDecodeError:
                                        # Skip invalid UTF-8 sequences
                                        continue
                        else:
                            # Only one element before last newline - might be partial
                            # Keep it in buffer
                            buffer = complete_lines[0]
                
                is_first_chunk = False  # After first iteration, we're no longer at the end
                
                # If we have enough lines, break early
                if len(lines_collected) >= max_lines:
                    break
            
            # Process remaining buffer and tail separately
            # Buffer contains the first line from the beginning of the file (if any) - oldest
            # Tail contains the last line from the end of the file (if file doesn't end with newline) - newest
            # These are separate log entries and should be processed separately
            # Since we're reading backwards (most recent first), tail should be at position 0
            if tail and len(lines_collected) < max_lines:
                try:
                    line_str = tail.decode('utf-8')
                    lines_collected.insert(0, line_str)  # Insert at beginning (most recent)
                except UnicodeDecodeError:
                    pass
            
            if buffer and len(lines_collected) < max_lines:
                try:
                    line_str = buffer.decode('utf-8')
                    lines_collected.append(line_str)  # Append at end (oldest)
                except UnicodeDecodeError:
                    pass
            
            # Process collected lines (already in reverse order - most recent first)
            for line in lines_collected:
                if not line.strip():
                    continue

                # Parse log line: "2026-01-08 12:34:16 - spotipy.util - WARNING - message"
                parts = line.strip().split(" - ", 3)
                if len(parts) < 4:
                    # Try to parse as-is if format doesn't match
                    log_entries.append({
                        "timestamp": None,
                        "level": "UNKNOWN",
                        "logger": "unknown",
                        "message": line.strip(),
                    })
                    continue

                timestamp_str = parts[0]
                logger_name = parts[1]
                level_str = parts[2]
                message = parts[3]

                # Parse timestamp using datetime for better timezone handling
                # Note: Log timestamps are in local time (no timezone info in log format)
                # datetime.strptime creates a naive datetime (assumes local timezone)
                # timestamp() converts to Unix timestamp (UTC-based, but calculated from local time)
                try:
                    dt = datetime.strptime(timestamp_str, "%Y-%m-%d %H:%M:%S")
                    timestamp = dt.timestamp()
                except ValueError:
                    timestamp = None

                # Apply filters
                # Filter by log level: only show entries at or above the filter level
                if filter_level is not None:
                    entry_level = log_levels.get(level_str)
                    # If entry level is unknown, include it (don't filter out)
                    # Otherwise, only include if entry level >= filter level
                    if entry_level is not None and entry_level < filter_level:
                        continue

                # Filter by time range (exclusive boundaries: start_time < timestamp < end_time)
                # If start_time is set, exclude entries before or at start_time
                if start_time is not None and timestamp is not None:
                    if timestamp <= start_time:
                        continue
                
                # If end_time is set, exclude entries at or after end_time
                if end_time is not None and timestamp is not None:
                    if timestamp >= end_time:
                        continue

                # Filter by search query (case-insensitive substring match)
                if search_query:
                    # Strip search query to handle whitespace-only queries
                    search_query_clean = search_query.strip()
                    if search_query_clean:
                        if search_query_clean.lower() not in message.lower():
                            continue
                    # If search query is empty/whitespace only, don't filter

                log_entries.append({
                    "timestamp": timestamp,
                    "timestamp_str": timestamp_str,
                    "level": level_str,
                    "logger": logger_name,
                    "message": message,
                })

                # Limit results
                if len(log_entries) >= max_lines:
                    break

    except (IOError, PermissionError, OSError) as e:
        # File I/O errors - return error entry
        logger.error(f"Error reading log file {log_path}: {e}", exc_info=True)
        return [{"error": f"Failed to read logs: {str(e)}", "timestamp": None, "level": "ERROR", "logger": "healthcheck_server", "message": f"Error reading log file: {str(e)}"}]
    except Exception as e:
        # Unexpected errors - log and return error entry
        logger.error(f"Unexpected error reading log file {log_path}: {e}", exc_info=True)
        return [{"error": f"Unexpected error reading logs: {str(e)}", "timestamp": None, "level": "ERROR", "logger": "healthcheck_server", "message": f"Unexpected error: {str(e)}"}]

    # Return in chronological order (oldest first)
    return list(reversed(log_entries))


def generate_logs_html(
    log_entries: List[Dict[str, any]],
    log_level: Optional[str] = None,
    search_query: Optional[str] = None,
    start_time: Optional[float] = None,
    end_time: Optional[float] = None,
    refresh_interval: int = 5,
) -> str:
    """
    Generate HTML log viewer page.

    Args:
        log_entries: List of log entries
        log_level: Current log level filter
        search_query: Current search query
        start_time: Current start time filter
        end_time: Current end time filter
        refresh_interval: Auto-refresh interval in seconds (clamped to 1-300)

    Returns:
        HTML string
    """
    # Validate and clamp refresh_interval
    try:
        refresh_interval = int(refresh_interval)
        if refresh_interval < 1:
            refresh_interval = 1
        elif refresh_interval > 300:
            refresh_interval = 300
    except (ValueError, TypeError):
        refresh_interval = 5  # Default on error
    
    # Build filter form values
    level_value = html.escape(log_level) if log_level else ""
    search_value = html.escape(search_query) if search_query else ""
    start_date_value = ""
    end_date_value = ""
    if start_time:
        start_date_value = time.strftime("%Y-%m-%dT%H:%M", time.localtime(start_time))
    if end_time:
        end_date_value = time.strftime("%Y-%m-%dT%H:%M", time.localtime(end_time))

    # Color mapping for log levels
    level_colors = {
        "DEBUG": "#9e9e9e",
        "INFO": "#4caf50",
        "WARNING": "#ff9800",
        "ERROR": "#f44336",
        "CRITICAL": "#d32f2f",
    }

    # Generate log entries HTML
    log_entries_html = ""
    if log_entries and "error" in log_entries[0]:
        log_entries_html = f'<div class="log-error">{html.escape(log_entries[0]["error"])}</div>'
    elif not log_entries:
        log_entries_html = '<div class="log-empty">No log entries found</div>'
    else:
        for entry in log_entries:
            level = entry.get("level", "UNKNOWN")
            level_color = level_colors.get(level, "#9e9e9e")
            timestamp_str = entry.get("timestamp_str", "N/A")
            logger_name = html.escape(entry.get("logger", "unknown"))
            original_message = entry.get("message", "")
            message = html.escape(original_message)

            # Highlight search query if present (case-insensitive)
            # Only highlight if search_query is non-empty and message contains it
            if search_query and search_query.strip():
                search_query_clean = search_query.strip()
                # Find all case-insensitive matches in the original message
                original_lower = original_message.lower()
                query_lower = search_query_clean.lower()
                
                if query_lower and query_lower in original_lower:
                    # Build highlighted message by finding matches in original and replacing in escaped version
                    # We need to track positions in original, then map to escaped positions
                    highlighted_parts = []
                    last_pos = 0
                    search_pos = 0
                    query_len = len(search_query_clean)
                    
                    while True:
                        # Find next case-insensitive match in original message
                        match_pos = original_lower.find(query_lower, search_pos)
                        if match_pos == -1:
                            break
                        
                        # Skip if this match overlaps with the previous one (match_pos < last_pos)
                        # This shouldn't happen with proper search_pos updates, but safety check
                        if match_pos < last_pos:
                            search_pos = match_pos + 1
                            continue
                        
                        # Get the actual matched substring (preserving original case)
                        matched_text = original_message[match_pos:match_pos + query_len]
                        
                        # Escape everything up to the match
                        highlighted_parts.append(html.escape(original_message[last_pos:match_pos]))
                        
                        # Add the highlighted match (escape the matched text)
                        highlighted_parts.append(f'<mark>{html.escape(matched_text)}</mark>')
                        
                        last_pos = match_pos + query_len
                        # Skip to after this match to avoid overlapping matches
                        search_pos = match_pos + query_len
                    
                    # Add remaining text
                    highlighted_parts.append(html.escape(original_message[last_pos:]))
                    message = ''.join(highlighted_parts)

            log_entries_html += f"""
            <div class="log-entry log-{level.lower()}">
                <span class="log-timestamp">{html.escape(timestamp_str)}</span>
                <span class="log-level" style="color: {level_color};">{html.escape(level)}</span>
                <span class="log-logger">{logger_name}</span>
                <span class="log-message">{message}</span>
            </div>
"""

    html_content = f"""<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="refresh" content="{refresh_interval}">
    <title>musicdl Logs</title>
    <style>
        * {{
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }}
        body {{
            font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
            background-color: #1a1a1a;
            color: #e0e0e0;
            line-height: 1.6;
            padding: 20px;
        }}
        .container {{
            max-width: 1400px;
            margin: 0 auto;
        }}
        header {{
            background-color: #2a2a2a;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
        }}
        h1 {{
            color: #ffffff;
            margin-bottom: 15px;
        }}
        .filters {{
            background-color: #2a2a2a;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
        }}
        .filters h2 {{
            color: #ffffff;
            margin-bottom: 15px;
            font-size: 1.2em;
        }}
        .filter-row {{
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-bottom: 15px;
        }}
        .filter-group {{
            display: flex;
            flex-direction: column;
        }}
        .filter-group label {{
            color: #b0b0b0;
            font-size: 0.9em;
            margin-bottom: 5px;
        }}
        .filter-group input,
        .filter-group select {{
            background-color: #1a1a1a;
            color: #e0e0e0;
            border: 1px solid #3a3a3a;
            padding: 8px;
            border-radius: 4px;
            font-family: inherit;
            font-size: 0.9em;
        }}
        .filter-group input:focus,
        .filter-group select:focus {{
            outline: none;
            border-color: #4caf50;
        }}
        .filter-buttons {{
            display: flex;
            gap: 10px;
            margin-top: 10px;
        }}
        button {{
            background-color: #4caf50;
            color: #ffffff;
            border: none;
            padding: 10px 20px;
            border-radius: 4px;
            cursor: pointer;
            font-size: 0.9em;
            font-weight: bold;
        }}
        button:hover {{
            background-color: #45a049;
        }}
        button.secondary {{
            background-color: #3a3a3a;
        }}
        button.secondary:hover {{
            background-color: #4a4a4a;
        }}
        .logs-container {{
            background-color: #1a1a1a;
            border: 1px solid #3a3a3a;
            border-radius: 8px;
            padding: 10px;
            max-height: 70vh;
            overflow-y: auto;
            font-size: 0.85em;
        }}
        .log-entry {{
            padding: 5px 10px;
            border-bottom: 1px solid #2a2a2a;
            display: grid;
            grid-template-columns: 180px 80px 150px 1fr;
            gap: 15px;
            align-items: start;
        }}
        .log-entry:hover {{
            background-color: #2a2a2a;
        }}
        .log-timestamp {{
            color: #9e9e9e;
            font-size: 0.9em;
        }}
        .log-level {{
            font-weight: bold;
            font-size: 0.9em;
        }}
        .log-logger {{
            color: #b0b0b0;
            font-size: 0.9em;
        }}
        .log-message {{
            color: #e0e0e0;
            word-wrap: break-word;
        }}
        .log-error {{
            color: #f44336;
            padding: 20px;
            text-align: center;
        }}
        .log-empty {{
            color: #9e9e9e;
            padding: 40px;
            text-align: center;
        }}
        mark {{
            background-color: #ffeb3b;
            color: #1a1a1a;
            padding: 2px 4px;
            border-radius: 2px;
        }}
        footer {{
            text-align: center;
            color: #b0b0b0;
            margin-top: 20px;
            font-size: 0.9em;
        }}
        .info {{
            color: #b0b0b0;
            font-size: 0.9em;
            margin-top: 10px;
        }}
        @media (max-width: 768px) {{
            .log-entry {{
                grid-template-columns: 1fr;
                gap: 5px;
            }}
            .filter-row {{
                grid-template-columns: 1fr;
            }}
        }}
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>musicdl Log Viewer</h1>
            <div class="info">
                Showing {len(log_entries)} log entries | Auto-refresh: {refresh_interval}s | 
                <a href="/status" style="color: #4caf50;">Back to Status</a>
            </div>
        </header>
        <div class="filters">
            <h2>Filters</h2>
            <form method="GET" action="/logs">
                <div class="filter-row">
                    <div class="filter-group">
                        <label for="level">Log Level</label>
                        <select name="level" id="level">
                            <option value="">All Levels</option>
                            <option value="DEBUG" {"selected" if level_value == "DEBUG" else ""}>DEBUG</option>
                            <option value="INFO" {"selected" if level_value == "INFO" else ""}>INFO</option>
                            <option value="WARNING" {"selected" if level_value == "WARNING" else ""}>WARNING</option>
                            <option value="ERROR" {"selected" if level_value == "ERROR" else ""}>ERROR</option>
                            <option value="CRITICAL" {"selected" if level_value == "CRITICAL" else ""}>CRITICAL</option>
                        </select>
                    </div>
                    <div class="filter-group">
                        <label for="search">Search</label>
                        <input type="text" name="search" id="search" value="{search_value}" placeholder="Search in messages...">
                    </div>
                    <div class="filter-group">
                        <label for="start_time">Start Time</label>
                        <input type="datetime-local" name="start_time" id="start_time" value="{start_date_value}">
                    </div>
                    <div class="filter-group">
                        <label for="end_time">End Time</label>
                        <input type="datetime-local" name="end_time" id="end_time" value="{end_date_value}">
                    </div>
                </div>
                <div class="filter-buttons">
                    <button type="submit">Apply Filters</button>
                    <button type="button" class="secondary" onclick="window.location.href='/logs'">Clear Filters</button>
                </div>
            </form>
        </div>
        <div class="logs-container">
            {log_entries_html}
        </div>
        <footer>
            musicdl Log Viewer | Auto-refreshing every {refresh_interval} seconds
        </footer>
    </div>
</body>
</html>
"""

    return html_content


def generate_status_html(
    plan: Optional[DownloadPlan],
    health_status: str,
    plan_file: Optional[str],
    plan_path: str,
    refresh_interval: int = 8,
) -> str:
    """
    Generate HTML status page.

    Args:
        plan: DownloadPlan instance or None
        health_status: Health status ("healthy" or "unhealthy")
        plan_file: Name of plan file being used
        plan_path: Path to plan directory
        refresh_interval: Auto-refresh interval in seconds (clamped to 1-300)

    Returns:
        HTML string
    """
    # Validate and clamp refresh_interval
    try:
        refresh_interval = int(refresh_interval)
        if refresh_interval < 1:
            refresh_interval = 1
        elif refresh_interval > 300:
            refresh_interval = 300
    except (ValueError, TypeError):
        refresh_interval = 8  # Default on error
    
    status_color = "#4caf50" if health_status == "healthy" else "#f44336"
    status_badge = "✓ Healthy" if health_status == "healthy" else "✗ Unhealthy"
    
    # Get phase information from plan metadata
    phase = None
    phase_updated_at = None
    if plan:
        phase = plan.metadata.get("phase")
        phase_updated_at = plan.metadata.get("phase_updated_at")
    
    phase_display = ""
    if phase:
        phase_labels = {
            "generating": "🔄 Generating Plan",
            "optimizing": "⚙️ Optimizing Plan",
            "executing": "▶️ Executing Plan",
        }
        phase_label = phase_labels.get(phase, phase.title())
        phase_display = f'<div class="phase-info">Current Phase: <strong>{html.escape(phase_label)}</strong>'
        if phase_updated_at:
            phase_display += f' (since {html.escape(format_timestamp(phase_updated_at))})'
        phase_display += "</div>"

    # Get rate limit information from plan metadata
    rate_limit_display = ""
    if plan:
        rate_limit_info = plan.metadata.get("rate_limit")
        if rate_limit_info and rate_limit_info.get("active"):
            current_time = time.time()
            retry_after_seconds = rate_limit_info.get("retry_after_seconds", 0)
            retry_after_timestamp = rate_limit_info.get("retry_after_timestamp", current_time)
            detected_at = rate_limit_info.get("detected_at", current_time)
            
            # Check if rate limit has expired
            # Use a small buffer (1 second) to account for timing precision
            if current_time < retry_after_timestamp - 1:
                # Calculate remaining time (ensure non-negative)
                remaining_seconds = max(0, int(retry_after_timestamp - current_time))
                remaining_hours = remaining_seconds // 3600
                remaining_minutes = (remaining_seconds % 3600) // 60
                remaining_secs = remaining_seconds % 60
                
                if remaining_hours > 0:
                    remaining_time_str = f"{remaining_hours}h {remaining_minutes}m {remaining_secs}s"
                elif remaining_minutes > 0:
                    remaining_time_str = f"{remaining_minutes}m {remaining_secs}s"
                else:
                    remaining_time_str = f"{remaining_secs}s"
                
                expires_at_str = format_timestamp(retry_after_timestamp)
                detected_at_str = format_timestamp(detected_at)
                
                rate_limit_display = f'''
        <div class="section rate-limit-section">
            <h2>⚠️ Spotify Rate Limit</h2>
            <div class="rate-limit-info">
                <div class="rate-limit-status active">
                    <strong>Status:</strong> <span class="rate-limit-active">Active</span>
                </div>
                <div class="rate-limit-details">
                    <p><strong>Retry After:</strong> {html.escape(str(retry_after_seconds))} seconds</p>
                    <p><strong>Time Remaining:</strong> {html.escape(remaining_time_str)}</p>
                    <p><strong>Expires At:</strong> {html.escape(expires_at_str)}</p>
                    <p><strong>Detected At:</strong> {html.escape(detected_at_str)}</p>
                </div>
                <div class="rate-limit-message">
                    <p>Your application has reached Spotify's rate/request limit. Operations will resume automatically after the rate limit expires.</p>
                </div>
            </div>
        </div>
'''

    # Use html_content instead of html to avoid shadowing the html module
    html_content = f"""<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta http-equiv="refresh" content="{refresh_interval}">
    <title>musicdl Status</title>
    <style>
        * {{
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }}
        body {{
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, Cantarell, sans-serif;
            background-color: #1a1a1a;
            color: #e0e0e0;
            line-height: 1.6;
            padding: 20px;
        }}
        .container {{
            max-width: 1200px;
            margin: 0 auto;
        }}
        header {{
            background-color: #2a2a2a;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
        }}
        h1 {{
            color: #ffffff;
            margin-bottom: 10px;
        }}
        .status-badge {{
            display: inline-block;
            padding: 8px 16px;
            border-radius: 4px;
            font-weight: bold;
            background-color: {status_color};
            color: #ffffff;
            margin-top: 10px;
        }}
        .info {{
            margin-top: 10px;
            color: #b0b0b0;
            font-size: 0.9em;
        }}
        .section {{
            background-color: #2a2a2a;
            padding: 20px;
            border-radius: 8px;
            margin-bottom: 20px;
        }}
        .section h2 {{
            color: #ffffff;
            margin-bottom: 15px;
            border-bottom: 2px solid #3a3a3a;
            padding-bottom: 10px;
        }}
        .stats-grid {{
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 15px;
            margin-bottom: 20px;
        }}
        .stat-card {{
            background-color: #1a1a1a;
            padding: 15px;
            border-radius: 4px;
            border-left: 4px solid #4caf50;
        }}
        .stat-card.failed {{
            border-left-color: #f44336;
        }}
        .stat-card.in-progress {{
            border-left-color: #ff9800;
        }}
        .stat-card.pending {{
            border-left-color: #9e9e9e;
        }}
        .stat-label {{
            color: #b0b0b0;
            font-size: 0.9em;
            margin-bottom: 5px;
        }}
        .stat-value {{
            color: #ffffff;
            font-size: 1.5em;
            font-weight: bold;
        }}
        table {{
            width: 100%;
            border-collapse: collapse;
            margin-top: 15px;
        }}
        th, td {{
            padding: 12px;
            text-align: left;
            border-bottom: 1px solid #3a3a3a;
        }}
        th {{
            background-color: #1a1a1a;
            color: #ffffff;
            font-weight: bold;
        }}
        tr:hover {{
            background-color: #3a3a3a;
        }}
        .status-cell {{
            font-weight: bold;
        }}
        .status-completed {{
            color: #4caf50;
        }}
        .status-failed {{
            color: #f44336;
        }}
        .status-in-progress {{
            color: #ff9800;
        }}
        .status-pending {{
            color: #9e9e9e;
        }}
        .status-skipped {{
            color: #9e9e9e;
        }}
        .error-cell {{
            color: #f44336;
            font-size: 0.9em;
            max-width: 300px;
            word-wrap: break-word;
        }}
        .phase-info {{
            margin-top: 10px;
            padding: 10px;
            background-color: #1a1a1a;
            border-radius: 4px;
            border-left: 4px solid #2196f3;
            color: #ffffff;
            font-size: 1.1em;
        }}
        .rate-limit-section {{
            border-left: 4px solid #ff9800;
        }}
        .rate-limit-info {{
            background-color: #1a1a1a;
            padding: 15px;
            border-radius: 4px;
            margin-top: 10px;
        }}
        .rate-limit-status {{
            margin-bottom: 15px;
            font-size: 1.1em;
        }}
        .rate-limit-active {{
            color: #ff9800;
            font-weight: bold;
        }}
        .rate-limit-details {{
            margin: 15px 0;
        }}
        .rate-limit-details p {{
            margin: 8px 0;
            color: #e0e0e0;
        }}
        .rate-limit-details strong {{
            color: #ffffff;
        }}
        .rate-limit-message {{
            margin-top: 15px;
            padding: 10px;
            background-color: #2a2a2a;
            border-radius: 4px;
            color: #b0b0b0;
            font-style: italic;
        }}
        .empty-state {{
            text-align: center;
            padding: 40px;
            color: #b0b0b0;
        }}
        footer {{
            text-align: center;
            color: #b0b0b0;
            margin-top: 20px;
            font-size: 0.9em;
        }}
        @media (max-width: 768px) {{
            .stats-grid {{
                grid-template-columns: 1fr;
            }}
            table {{
                font-size: 0.9em;
            }}
            th, td {{
                padding: 8px;
            }}
        }}
    </style>
</head>
<body>
    <div class="container">
        <header>
            <h1>musicdl Status Dashboard</h1>
            <div class="status-badge">{html.escape(status_badge)}</div>
            {phase_display}
            <div class="info">
                Plan file: {html.escape(plan_file or "None")} | Plan path: {html.escape(plan_path)} | Last updated: {html.escape(format_timestamp(time.time()))} | Auto-refresh: {refresh_interval}s | 
                <a href="/logs" style="color: #4caf50;">View Logs</a>
            </div>
        </header>
        {rate_limit_display}
"""

    if plan is None:
        html_content += """
        <div class="section">
            <h2>No Plan Files Found</h2>
            <div class="empty-state">
                <p>No plan files were found in the plan directory.</p>
                <p>This may indicate that plan generation has not started yet, or plan persistence is disabled.</p>
            </div>
        </div>
"""
    else:
        stats = aggregate_plan_status(plan)
        by_status = stats.get("by_status", {})
        by_type = stats.get("by_type", {})

        html_content += f"""
        <div class="section">
            <h2>Statistics</h2>
            <div class="stats-grid">
                <div class="stat-card">
                    <div class="stat-label">Total Items</div>
                    <div class="stat-value">{stats.get('total_items', 0)}</div>
                </div>
                <div class="stat-card">
                    <div class="stat-label">Completed</div>
                    <div class="stat-value">{by_status.get('completed', 0)}</div>
                </div>
                <div class="stat-card in-progress">
                    <div class="stat-label">In Progress</div>
                    <div class="stat-value">{by_status.get('in_progress', 0)}</div>
                </div>
                <div class="stat-card failed">
                    <div class="stat-label">Failed</div>
                    <div class="stat-value">{by_status.get('failed', 0)}</div>
                </div>
                <div class="stat-card pending">
                    <div class="stat-label">Pending</div>
                    <div class="stat-value">{by_status.get('pending', 0)}</div>
                </div>
                <div class="stat-card">
                    <div class="stat-label">Skipped</div>
                    <div class="stat-value">{by_status.get('skipped', 0)}</div>
                </div>
            </div>
            <h3 style="margin-top: 20px; color: #ffffff;">By Type</h3>
            <div class="stats-grid">
"""

        for item_type, count in by_type.items():
            if count > 0:
                # Escape item_type to prevent XSS (though it's from enum, better safe)
                item_type_escaped = html.escape(item_type.title())
                html_content += f"""
                <div class="stat-card">
                    <div class="stat-label">{item_type_escaped}</div>
                    <div class="stat-value">{count}</div>
                </div>
"""

        html_content += """
            </div>
        </div>
        <div class="section">
            <h2>Plan Items</h2>
"""

        if plan.items:
            html_content += """
            <table>
                <thead>
                    <tr>
                        <th>ID</th>
                        <th>Name</th>
                        <th>Type</th>
                        <th>Status</th>
                        <th>Progress</th>
                        <th>Error</th>
                    </tr>
                </thead>
                <tbody>
"""

            for item in plan.items:
                status_class = f"status-{item.status.value}"
                progress_pct = f"{item.progress * 100:.1f}%" if item.progress > 0 else "-"
                error_display = truncate_error(item.error) if item.error else "-"

                # Escape user-controlled values to prevent XSS
                item_id_escaped = html.escape(item.item_id[:8])
                item_name_escaped = html.escape(item.name or '-')
                item_type_escaped = html.escape(item.item_type.value)
                status_value_escaped = html.escape(item.status.value)
                error_display_escaped = html.escape(error_display)

                html_content += f"""
                    <tr>
                        <td>{item_id_escaped}...</td>
                        <td>{item_name_escaped}</td>
                        <td>{item_type_escaped}</td>
                        <td class="status-cell {status_class}">{status_value_escaped}</td>
                        <td>{progress_pct}</td>
                        <td class="error-cell">{error_display_escaped}</td>
                    </tr>
"""

            html_content += """
                </tbody>
            </table>
"""
        else:
            html_content += """
            <div class="empty-state">
                <p>No items in plan</p>
            </div>
"""

        html_content += """
        </div>
"""

    html_content += f"""
        <footer>
            musicdl Healthcheck Server | Auto-refreshing every {refresh_interval} seconds
        </footer>
    </div>
</body>
</html>
"""

    return html_content


class HealthcheckHandler(http.server.BaseHTTPRequestHandler):
    """HTTP request handler for healthcheck endpoints."""

    def __init__(self, plan_dir: Path, log_path: Path, *args, **kwargs):
        """
        Initialize handler with plan directory and log path.

        Args:
            plan_dir: Directory containing plan files
            log_path: Path to log file
            *args: Positional arguments for BaseHTTPRequestHandler
            **kwargs: Keyword arguments for BaseHTTPRequestHandler
        """
        self.plan_dir = plan_dir
        self.log_path = log_path
        super().__init__(*args, **kwargs)

    def log_message(self, format: str, *args: any) -> None:
        """
        Override to use our logger instead of default logging.

        Args:
            format: Log format string
            *args: Format arguments
        """
        logger.debug(f"{self.address_string()} - {format % args}")

    def do_GET(self) -> None:
        """
        Handle GET requests for /health and /status endpoints.

        Returns:
            JSON response for /health, HTML response for /status
        """
        parsed_path = urllib.parse.urlparse(self.path)
        path = parsed_path.path

        # Parse query parameters for refresh interval
        query_params = urllib.parse.parse_qs(parsed_path.query)
        refresh_interval = 8  # Default
        if "refresh" in query_params:
            try:
                refresh_value = query_params["refresh"][0]
                if refresh_value:  # Check for empty string
                    refresh_interval = int(refresh_value)
                    # Clamp to reasonable range (1-300 seconds)
                    if refresh_interval < 1:
                        refresh_interval = 1
                    elif refresh_interval > 300:
                        refresh_interval = 300
            except (ValueError, IndexError, TypeError):
                # Invalid refresh parameter - use default
                refresh_interval = 8

        try:
            # Load and aggregate plans
            plan, plan_file = find_and_aggregate_plans(self.plan_dir)
            health_status, reason = determine_health_status(plan)

            if path == "/health":
                # JSON healthcheck endpoint
                # Note: HTTP server health is implicit - if server cannot respond,
                # Docker will automatically mark container as unhealthy
                stats = aggregate_plan_status(plan) if plan else {}
                response_data = {
                    "status": health_status,
                    "reason": reason,
                    "timestamp": time.time(),
                    "plan_file": plan_file,
                    "statistics": stats,
                    "server_health": "healthy",  # Server is responding (implicit check)
                }
                # Add phase information if available
                if plan:
                    phase = plan.metadata.get("phase")
                    if phase:
                        response_data["phase"] = phase
                        phase_updated_at = plan.metadata.get("phase_updated_at")
                        if phase_updated_at:
                            response_data["phase_updated_at"] = phase_updated_at
                    
                    # Add rate limit information if available
                    rate_limit_info = plan.metadata.get("rate_limit")
                    if rate_limit_info and rate_limit_info.get("active"):
                        current_time = time.time()
                        retry_after_timestamp = rate_limit_info.get("retry_after_timestamp", current_time)
                        # Only include if rate limit hasn't expired (with 1 second buffer for timing precision)
                        if current_time < retry_after_timestamp - 1:
                            remaining_seconds = max(0, int(retry_after_timestamp - current_time))
                            response_data["rate_limit"] = {
                                "active": True,
                                "retry_after_seconds": rate_limit_info.get("retry_after_seconds", 0),
                                "retry_after_timestamp": retry_after_timestamp,
                                "detected_at": rate_limit_info.get("detected_at", current_time),
                                "remaining_seconds": remaining_seconds,
                            }

                # Set status code based on health
                # 200 = healthy, 503 = unhealthy (service unavailable)
                status_code = 200 if health_status == "healthy" else 503

                self.send_response(status_code)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                self.wfile.write(json.dumps(response_data, indent=2).encode("utf-8"))

            elif path == "/status":
                # HTML status page
                html = generate_status_html(
                    plan,
                    health_status,
                    plan_file,
                    str(self.plan_dir),
                    refresh_interval,
                )

                self.send_response(200)
                self.send_header("Content-Type", "text/html; charset=utf-8")
                self.end_headers()
                self.wfile.write(html.encode("utf-8"))

            elif path == "/logs":
                # HTML log viewer page
                # Parse query parameters for filters (safely handle missing or empty values)
                log_level = None
                search_query = None
                start_time_str = None
                end_time_str = None
                
                try:
                    if "level" in query_params and query_params["level"]:
                        log_level = query_params["level"][0] if query_params["level"][0] else None
                except (IndexError, TypeError, AttributeError):
                    log_level = None
                
                try:
                    if "search" in query_params and query_params["search"]:
                        search_query = query_params["search"][0] if query_params["search"][0] else None
                except (IndexError, TypeError, AttributeError):
                    search_query = None
                
                try:
                    if "start_time" in query_params and query_params["start_time"]:
                        start_time_str = query_params["start_time"][0] if query_params["start_time"][0] else None
                except (IndexError, TypeError, AttributeError):
                    start_time_str = None
                
                try:
                    if "end_time" in query_params and query_params["end_time"]:
                        end_time_str = query_params["end_time"][0] if query_params["end_time"][0] else None
                except (IndexError, TypeError, AttributeError):
                    end_time_str = None

                # Parse datetime strings to timestamps using datetime for better timezone handling
                # datetime-local input format: "YYYY-MM-DDTHH:MM" (local time, no timezone)
                start_time = None
                end_time = None
                if start_time_str and start_time_str.strip():
                    try:
                        # Parse as local time (naive datetime)
                        dt = datetime.strptime(start_time_str.strip(), "%Y-%m-%dT%H:%M")
                        # Convert to timestamp (assumes local timezone)
                        start_time = dt.timestamp()
                    except (ValueError, TypeError, AttributeError) as e:
                        logger.debug(f"Invalid start_time format '{start_time_str}': {e}")
                        # Invalid format - ignore filter
                        pass
                if end_time_str and end_time_str.strip():
                    try:
                        # Parse as local time (naive datetime)
                        dt = datetime.strptime(end_time_str.strip(), "%Y-%m-%dT%H:%M")
                        # Convert to timestamp (assumes local timezone)
                        end_time = dt.timestamp()
                    except (ValueError, TypeError, AttributeError) as e:
                        logger.debug(f"Invalid end_time format '{end_time_str}': {e}")
                        # Invalid format - ignore filter
                        pass

                # Read and filter logs
                log_entries = read_logs(
                    self.log_path,
                    log_level=log_level,
                    search_query=search_query,
                    start_time=start_time,
                    end_time=end_time,
                )

                # Generate HTML
                html = generate_logs_html(
                    log_entries,
                    log_level=log_level,
                    search_query=search_query,
                    start_time=start_time,
                    end_time=end_time,
                    refresh_interval=refresh_interval,
                )

                self.send_response(200)
                self.send_header("Content-Type", "text/html; charset=utf-8")
                self.end_headers()
                self.wfile.write(html.encode("utf-8"))

            else:
                # 404 for unknown endpoints
                self.send_response(404)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                error_response = {
                    "error": "Not found",
                    "message": f"Unknown endpoint: {path}",
                    "available_endpoints": ["/health", "/status", "/logs"],
                }
                self.wfile.write(json.dumps(error_response, indent=2).encode("utf-8"))

        except Exception as e:
            # Handle any unexpected errors
            logger.error(f"Error handling request {path}: {e}", exc_info=True)
            self.send_response(500)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            error_response = {
                "error": "Internal server error",
                "message": str(e),
            }
            self.wfile.write(json.dumps(error_response, indent=2).encode("utf-8"))


def create_handler_class(plan_dir: Path, log_path: Path):
    """
    Create a handler class with plan_dir and log_path bound.

    Args:
        plan_dir: Directory containing plan files
        log_path: Path to log file

    Returns:
        Handler class
    """

    class Handler(HealthcheckHandler):
        def __init__(self, *args, **kwargs):
            super().__init__(plan_dir, log_path, *args, **kwargs)

    return Handler


# Global server instance for signal handling
server: Optional[http.server.HTTPServer] = None


def shutdown_handler(signum, frame):
    """
    Handle shutdown signals gracefully.

    Args:
        signum: Signal number
        frame: Current stack frame
    """
    logger.info(f"Received signal {signum}, shutting down healthcheck server...")
    if server:
        server.shutdown()
    sys.exit(0)


def main() -> None:
    """Main entry point for healthcheck server."""
    # Read configuration
    plan_dir = get_plan_path()
    log_path = get_log_path()
    port = get_port()

    # Validate plan directory
    if not plan_dir.exists():
        logger.warning(f"Plan directory does not exist: {plan_dir}")
        logger.warning("Healthcheck will return unhealthy until plan files are created")

    if not plan_dir.is_dir():
        logger.error(f"Plan path is not a directory: {plan_dir}")
        sys.exit(1)

    # Log file will be created by download.py, so we don't need to validate it exists
    logger.info(f"Log file path: {log_path}")

    # Register signal handlers (only in main thread)
    # Signal handlers can only be registered in the main thread
    if threading.current_thread() == threading.main_thread():
        try:
            signal.signal(signal.SIGTERM, shutdown_handler)
            signal.signal(signal.SIGINT, shutdown_handler)
            logger.debug("Registered signal handlers for graceful shutdown")
        except ValueError as e:
            # Should not happen if we checked main thread, but handle gracefully
            logger.warning(f"Cannot register signal handlers: {e}")
    else:
        logger.debug(
            "Skipping signal handler registration (not in main thread). "
            "Graceful shutdown via signals will not be available."
        )

    # Create HTTP server
    handler_class = create_handler_class(plan_dir, log_path)
    global server
    server = http.server.HTTPServer(("0.0.0.0", port), handler_class)

    logger.info(f"Healthcheck server started on port {port}")
    logger.info(f"Monitoring plan directory: {plan_dir}")
    logger.info(f"Log file: {log_path}")
    logger.info("Endpoints: /health (JSON), /status (HTML), /logs (HTML)")

    try:
        server.serve_forever()
    except KeyboardInterrupt:
        logger.info("Server interrupted by user")
    finally:
        if server:
            server.shutdown()
        logger.info("Healthcheck server stopped")


if __name__ == "__main__":
    main()

