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
from pathlib import Path
from typing import Dict, List, Optional, Tuple

# Add /scripts to path to import core modules
sys.path.insert(0, "/scripts")

try:
    from core.plan import DownloadPlan, PlanItem, PlanItemStatus, PlanItemType
    from core.utils import get_plan_path
except ImportError as e:
    # Fallback if running outside container
    import sys
    from pathlib import Path
    # Try to import from current directory structure
    sys.path.insert(0, str(Path(__file__).parent.parent))
    from core.plan import DownloadPlan, PlanItem, PlanItemStatus, PlanItemType
    from core.utils import get_plan_path

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
        data = load_plan_file(filepath)
        if data is not None:
            plans_data[filename] = data
            found_files.append(filename)

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

        for item_data in items_data:
            item_id = item_data.get("item_id")
            if item_id:
                # Only add if not already present (higher priority already has it)
                if item_id not in aggregated_items:
                    aggregated_items[item_id] = item_data

    # Build aggregated plan
    if not aggregated_items:
        # No items found in any plan file
        return None, None

    # Use metadata from highest priority file
    highest_priority_data = plans_data[found_files[0]]
    aggregated_plan_dict = {
        "items": list(aggregated_items.values()),
        "created_at": highest_priority_data.get("created_at", time.time()),
        "metadata": highest_priority_data.get("metadata", {}),
    }

    try:
        plan = DownloadPlan.from_dict(aggregated_plan_dict)
        return plan, source_file
    except (ValueError, KeyError) as e:
        logger.error(f"Failed to create plan from aggregated data: {e}")
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

    Args:
        plan: DownloadPlan instance or None

    Returns:
        Tuple of (status, reason) where status is "healthy" or "unhealthy"
    """
    if plan is None:
        return "unhealthy", "No plan files found"

    if not plan.items:
        return "unhealthy", "Plan has no items"

    # Count items by status
    by_status = {
        status.value: len(plan.get_items_by_status(status))
        for status in PlanItemStatus
    }

    total_items = len(plan.items)
    completed = by_status.get("completed", 0)
    failed = by_status.get("failed", 0)
    in_progress = by_status.get("in_progress", 0)
    pending = by_status.get("pending", 0)
    skipped = by_status.get("skipped", 0)

    # Healthy conditions:
    # 1. Has items with in_progress or completed status (work is happening or done)
    # 2. At least one item is not failed
    # 3. All pending is healthy (plan ready to execute)
    # 4. All skipped is healthy (plan completed, items skipped intentionally)

    if failed == total_items and total_items > 0:
        return "unhealthy", f"All {total_items} items failed"

    if in_progress > 0 or completed > 0 or skipped > 0:
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
        refresh_interval: Auto-refresh interval in seconds

    Returns:
        HTML string
    """
    status_color = "#4caf50" if health_status == "healthy" else "#f44336"
    status_badge = "✓ Healthy" if health_status == "healthy" else "✗ Unhealthy"

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
            <div class="info">
                Plan file: {html.escape(plan_file or "None")} | Plan path: {html.escape(plan_path)} | Last updated: {html.escape(format_timestamp(time.time()))} | Auto-refresh: {refresh_interval}s
            </div>
        </header>
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

    def __init__(self, plan_dir: Path, *args, **kwargs):
        """
        Initialize handler with plan directory.

        Args:
            plan_dir: Directory containing plan files
            *args: Positional arguments for BaseHTTPRequestHandler
            **kwargs: Keyword arguments for BaseHTTPRequestHandler
        """
        self.plan_dir = plan_dir
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
                refresh_interval = int(query_params["refresh"][0])
                if refresh_interval < 1 or refresh_interval > 300:
                    refresh_interval = 8  # Clamp to reasonable range
            except (ValueError, IndexError):
                pass

        try:
            # Load and aggregate plans
            plan, plan_file = find_and_aggregate_plans(self.plan_dir)
            health_status, reason = determine_health_status(plan)

            if path == "/health":
                # JSON healthcheck endpoint
                stats = aggregate_plan_status(plan) if plan else {}
                response_data = {
                    "status": health_status,
                    "reason": reason,
                    "timestamp": time.time(),
                    "plan_file": plan_file,
                    "statistics": stats,
                }

                # Set status code based on health
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

            else:
                # 404 for unknown endpoints
                self.send_response(404)
                self.send_header("Content-Type", "application/json")
                self.end_headers()
                error_response = {
                    "error": "Not found",
                    "message": f"Unknown endpoint: {path}",
                    "available_endpoints": ["/health", "/status"],
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


def create_handler_class(plan_dir: Path):
    """
    Create a handler class with plan_dir bound.

    Args:
        plan_dir: Directory containing plan files

    Returns:
        Handler class
    """

    class Handler(HealthcheckHandler):
        def __init__(self, *args, **kwargs):
            super().__init__(plan_dir, *args, **kwargs)

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
    port = get_port()

    # Validate plan directory
    if not plan_dir.exists():
        logger.warning(f"Plan directory does not exist: {plan_dir}")
        logger.warning("Healthcheck will return unhealthy until plan files are created")

    if not plan_dir.is_dir():
        logger.error(f"Plan path is not a directory: {plan_dir}")
        sys.exit(1)

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
    handler_class = create_handler_class(plan_dir)
    global server
    server = http.server.HTTPServer(("0.0.0.0", port), handler_class)

    logger.info(f"Healthcheck server started on port {port}")
    logger.info(f"Monitoring plan directory: {plan_dir}")
    logger.info("Endpoints: /health (JSON), /status (HTML)")

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

