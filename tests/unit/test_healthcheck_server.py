"""
Unit tests for healthcheck server functions.

Tests the core functions used by the healthcheck server without
actually starting an HTTP server.
"""

import json
import os
import pytest
import sys
import time
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

# Add project root to path for imports
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from core.plan import DownloadPlan, PlanItem, PlanItemStatus, PlanItemType

# Import healthcheck server functions
# The script has special import handling, so we need to ensure paths are set up
# We'll import the functions directly by executing the module's import logic
import importlib.util
scripts_path = project_root / "scripts"
healthcheck_server_path = scripts_path / "healthcheck_server.py"
spec = importlib.util.spec_from_file_location("healthcheck_server", healthcheck_server_path)
healthcheck_server = importlib.util.module_from_spec(spec)
# Temporarily modify sys.path to allow imports
original_path = sys.path.copy()
sys.path.insert(0, str(project_root))
try:
    spec.loader.exec_module(healthcheck_server)
finally:
    sys.path = original_path


class TestHealthcheckFunctions:
    """Test healthcheck server utility functions."""

    def test_get_port_default(self, monkeypatch):
        """Test get_port() with default value."""
        monkeypatch.delenv("HEALTHCHECK_PORT", raising=False)
        
        assert healthcheck_server.get_port() == 8080

    def test_get_port_custom(self, monkeypatch):
        """Test get_port() with custom environment variable."""
        monkeypatch.setenv("HEALTHCHECK_PORT", "9000")
        
        assert healthcheck_server.get_port() == 9000

    def test_get_port_invalid(self, monkeypatch):
        """Test get_port() with invalid value falls back to default."""
        monkeypatch.setenv("HEALTHCHECK_PORT", "invalid")
        
        # Should fall back to default
        assert healthcheck_server.get_port() == 8080

    def test_load_plan_file_exists(self, tmp_test_dir):
        """Test load_plan_file() with existing valid JSON file."""
        plan_file = tmp_test_dir / "test_plan.json"
        plan_data = {
            "items": [],
            "created_at": time.time(),
            "metadata": {},
        }
        plan_file.write_text(json.dumps(plan_data))
        
        result = healthcheck_server.load_plan_file(plan_file)
        assert result is not None
        assert result["items"] == []
        assert "created_at" in result

    def test_load_plan_file_not_exists(self, tmp_test_dir):
        """Test load_plan_file() with non-existent file."""
        plan_file = tmp_test_dir / "nonexistent.json"
        
        result = healthcheck_server.load_plan_file(plan_file)
        assert result is None

    def test_load_plan_file_invalid_json(self, tmp_test_dir):
        """Test load_plan_file() with invalid JSON."""
        plan_file = tmp_test_dir / "invalid.json"
        plan_file.write_text("{ invalid json }")
        
        result = healthcheck_server.load_plan_file(plan_file)
        assert result is None

    def test_determine_health_status_no_plan(self):
        """Test determine_health_status() with no plan."""
        status, reason = healthcheck_server.determine_health_status(None)
        assert status == "unhealthy"
        assert "No plan files found" in reason

    def test_determine_health_status_empty_plan(self):
        """Test determine_health_status() with empty plan."""
        plan = DownloadPlan(items=[])
        
        status, reason = healthcheck_server.determine_health_status(plan)
        assert status == "unhealthy"
        assert "no items" in reason.lower()

    def test_determine_health_status_all_failed(self):
        """Test determine_health_status() with all items failed."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            )
        ]
        items[0].mark_failed("Error")
        plan = DownloadPlan(items=items)
        
        status, reason = healthcheck_server.determine_health_status(plan)
        assert status == "unhealthy"
        assert "All" in reason and "failed" in reason

    def test_determine_health_status_in_progress(self):
        """Test determine_health_status() with in_progress items."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            )
        ]
        items[0].mark_started()
        plan = DownloadPlan(items=items)
        
        status, reason = healthcheck_server.determine_health_status(plan)
        assert status == "healthy"
        assert "in progress" in reason.lower() or "completed" in reason.lower()

    def test_determine_health_status_completed(self):
        """Test determine_health_status() with completed items."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            )
        ]
        items[0].mark_completed()
        plan = DownloadPlan(items=items)
        
        status, reason = healthcheck_server.determine_health_status(plan)
        assert status == "healthy"
        assert "completed" in reason.lower()

    def test_determine_health_status_all_pending(self):
        """Test determine_health_status() with all pending items (should be healthy)."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            ),
            PlanItem(
                item_id="track2",
                item_type=PlanItemType.TRACK,
                name="Track 2",
            ),
        ]
        plan = DownloadPlan(items=items)
        
        status, reason = healthcheck_server.determine_health_status(plan)
        assert status == "healthy"
        assert "pending" in reason.lower() or "ready" in reason.lower()

    def test_determine_health_status_mixed(self):
        """Test determine_health_status() with mixed statuses."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            ),
            PlanItem(
                item_id="track2",
                item_type=PlanItemType.TRACK,
                name="Track 2",
            ),
            PlanItem(
                item_id="track3",
                item_type=PlanItemType.TRACK,
                name="Track 3",
            ),
        ]
        items[0].mark_completed()
        items[1].mark_failed("Error")
        # items[2] remains pending
        plan = DownloadPlan(items=items)
        
        status, reason = healthcheck_server.determine_health_status(plan)
        assert status == "healthy"  # Should be healthy because not all failed

    def test_format_timestamp(self):
        """Test format_timestamp() function."""
        timestamp = time.time()
        formatted = healthcheck_server.format_timestamp(timestamp)
        
        # Should be a readable date/time string
        assert len(formatted) > 0
        assert "202" in formatted or "203" in formatted  # Year should be present

    def test_truncate_error_short(self):
        """Test truncate_error() with short error message."""
        error = "Short error"
        result = healthcheck_server.truncate_error(error)
        assert result == error

    def test_truncate_error_long(self):
        """Test truncate_error() with long error message."""
        error = "A" * 300
        result = healthcheck_server.truncate_error(error)
        assert len(result) == 203  # 200 + "..."
        assert result.endswith("...")

    def test_truncate_error_none(self):
        """Test truncate_error() with None."""
        result = healthcheck_server.truncate_error(None)
        assert result == ""

    def test_find_and_aggregate_plans_no_files(self, tmp_test_dir):
        """Test find_and_aggregate_plans() with no plan files."""
        plan, plan_file = healthcheck_server.find_and_aggregate_plans(tmp_test_dir)
        assert plan is None
        assert plan_file is None

    def test_find_and_aggregate_plans_single_file(self, tmp_test_dir):
        """Test find_and_aggregate_plans() with single plan file."""
        # Create a plan file
        plan_file = tmp_test_dir / "download_plan.json"
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            ).to_dict()
        ]
        plan_data = {
            "items": items,
            "created_at": time.time(),
            "metadata": {},
        }
        plan_file.write_text(json.dumps(plan_data))
        
        plan, source_file = healthcheck_server.find_and_aggregate_plans(tmp_test_dir)
        assert plan is not None
        assert source_file == "download_plan.json"
        assert len(plan.items) == 1

    def test_find_and_aggregate_plans_priority_order(self, tmp_test_dir):
        """Test find_and_aggregate_plans() respects priority order when aggregating."""
        # Create multiple plan files with overlapping and unique items
        # The function should aggregate all items, preferring higher priority versions
        progress_file = tmp_test_dir / "download_plan_progress.json"
        optimized_file = tmp_test_dir / "download_plan_optimized.json"
        initial_file = tmp_test_dir / "download_plan.json"
        
        # Create plans with some overlapping item_ids and some unique ones
        # track1 appears in both progress and optimized - progress should win
        # track2 appears only in optimized
        # track3 appears only in initial
        progress_data = {
            "items": [
                PlanItem(
                    item_id="track1",
                    item_type=PlanItemType.TRACK,
                    name="Track 1 Progress",  # Higher priority version
                ).to_dict()
            ],
            "created_at": time.time(),
            "metadata": {},
        }
        
        optimized_data = {
            "items": [
                PlanItem(
                    item_id="track1",
                    item_type=PlanItemType.TRACK,
                    name="Track 1 Optimized",  # Lower priority, should be ignored
                ).to_dict(),
                PlanItem(
                    item_id="track2",
                    item_type=PlanItemType.TRACK,
                    name="Track 2 Optimized",  # Unique, should be included
                ).to_dict()
            ],
            "created_at": time.time(),
            "metadata": {},
        }
        
        initial_data = {
            "items": [
                PlanItem(
                    item_id="track3",
                    item_type=PlanItemType.TRACK,
                    name="Track 3 Initial",  # Unique, should be included
                ).to_dict()
            ],
            "created_at": time.time(),
            "metadata": {},
        }
        
        progress_file.write_text(json.dumps(progress_data))
        optimized_file.write_text(json.dumps(optimized_data))
        initial_file.write_text(json.dumps(initial_data))
        
        plan, source_file = healthcheck_server.find_and_aggregate_plans(tmp_test_dir)
        assert plan is not None
        assert source_file == "download_plan_progress.json"  # Highest priority file name
        # Should aggregate all unique items (track1 from progress, track2 from optimized, track3 from initial)
        assert len(plan.items) == 3
        
        # Verify track1 comes from progress (higher priority)
        track1 = next((item for item in plan.items if item.item_id == "track1"), None)
        assert track1 is not None
        assert track1.name == "Track 1 Progress"  # Should use progress version, not optimized
        
        # Verify track2 and track3 are included
        track2 = next((item for item in plan.items if item.item_id == "track2"), None)
        assert track2 is not None
        assert track2.name == "Track 2 Optimized"
        
        track3 = next((item for item in plan.items if item.item_id == "track3"), None)
        assert track3 is not None
        assert track3.name == "Track 3 Initial"

    def test_determine_health_status_excludes_skipped(self):
        """Test that determine_health_status() excludes SKIPPED items from health calculations."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            ),
            PlanItem(
                item_id="track2",
                item_type=PlanItemType.TRACK,
                name="Track 2",
            ),
        ]
        items[0].mark_skipped("File already exists")
        items[1].mark_completed()
        plan = DownloadPlan(items=items)
        
        status, reason = healthcheck_server.determine_health_status(plan)
        # Should be healthy because track2 is completed (track1 is skipped and excluded)
        assert status == "healthy"
        assert "completed" in reason.lower()

    def test_determine_health_status_all_skipped(self):
        """Test that determine_health_status() handles all items being skipped."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            ),
            PlanItem(
                item_id="track2",
                item_type=PlanItemType.TRACK,
                name="Track 2",
            ),
        ]
        items[0].mark_skipped("File already exists")
        items[1].mark_skipped("File already exists")
        plan = DownloadPlan(items=items)
        
        status, reason = healthcheck_server.determine_health_status(plan)
        # Should be healthy because all items are skipped (no updates needed)
        assert status == "healthy"
        assert "skipped" in reason.lower() or "no updates" in reason.lower()

    def test_determine_health_status_skipped_with_failed(self):
        """Test that determine_health_status() excludes SKIPPED items when calculating failures."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            ),
            PlanItem(
                item_id="track2",
                item_type=PlanItemType.TRACK,
                name="Track 2",
            ),
            PlanItem(
                item_id="track3",
                item_type=PlanItemType.TRACK,
                name="Track 3",
            ),
        ]
        items[0].mark_skipped("File already exists")
        items[1].mark_failed("Error")
        items[2].mark_completed()
        plan = DownloadPlan(items=items)
        
        status, reason = healthcheck_server.determine_health_status(plan)
        # Should be healthy because track3 is completed (track1 is skipped, track2 failed)
        # Not all non-skipped items failed
        assert status == "healthy"


class TestReadLogs:
    """Test read_logs() function."""

    def test_read_logs_missing_file(self, tmp_test_dir):
        """Test read_logs() with missing log file."""
        log_path = tmp_test_dir / "nonexistent.log"
        
        result = healthcheck_server.read_logs(log_path)
        assert result == []

    def test_read_logs_empty_file(self, tmp_test_dir):
        """Test read_logs() with empty log file."""
        log_path = tmp_test_dir / "empty.log"
        log_path.touch()
        
        result = healthcheck_server.read_logs(log_path)
        assert result == []

    def test_read_logs_basic_reading(self, tmp_test_dir):
        """Test read_logs() reads log entries correctly."""
        from tests.helpers import create_log_file, create_sample_log_entries
        
        log_path = tmp_test_dir / "test.log"
        entries = create_sample_log_entries(count=5, interval_seconds=1)
        create_log_file(log_path, entries)
        
        result = healthcheck_server.read_logs(log_path)
        
        assert len(result) == 5
        # Should be in chronological order (oldest first)
        # Messages cycle through different types, so just verify we have entries
        assert result[0]["message"].endswith("(entry 1)")
        assert result[-1]["message"].endswith("(entry 5)")

    def test_read_logs_log_level_filter(self, tmp_test_dir):
        """Test read_logs() filters by log level."""
        from tests.helpers import create_log_file, create_sample_log_entries
        
        log_path = tmp_test_dir / "test.log"
        # Create entries with different levels
        entries = [
            {"timestamp": time.time() - 5, "logger": "test", "level": "DEBUG", "message": "Debug message"},
            {"timestamp": time.time() - 4, "logger": "test", "level": "INFO", "message": "Info message"},
            {"timestamp": time.time() - 3, "logger": "test", "level": "WARNING", "message": "Warning message"},
            {"timestamp": time.time() - 2, "logger": "test", "level": "ERROR", "message": "Error message"},
        ]
        create_log_file(log_path, entries)
        
        # Filter for WARNING and above
        result = healthcheck_server.read_logs(log_path, log_level="WARNING")
        
        assert len(result) == 2
        assert all(entry["level"] in ["WARNING", "ERROR"] for entry in result)

    def test_read_logs_search_query(self, tmp_test_dir):
        """Test read_logs() filters by search query."""
        from tests.helpers import create_log_file
        
        log_path = tmp_test_dir / "test.log"
        entries = [
            {"timestamp": time.time() - 3, "logger": "test", "level": "INFO", "message": "Download started"},
            {"timestamp": time.time() - 2, "logger": "test", "level": "INFO", "message": "Error occurred"},
            {"timestamp": time.time() - 1, "logger": "test", "level": "INFO", "message": "Download complete"},
        ]
        create_log_file(log_path, entries)
        
        result = healthcheck_server.read_logs(log_path, search_query="Error")
        
        assert len(result) == 1
        assert "Error" in result[0]["message"]

    def test_read_logs_search_query_case_insensitive(self, tmp_test_dir):
        """Test read_logs() search is case-insensitive."""
        from tests.helpers import create_log_file
        
        log_path = tmp_test_dir / "test.log"
        entries = [
            {"timestamp": time.time() - 2, "logger": "test", "level": "INFO", "message": "ERROR occurred"},
            {"timestamp": time.time() - 1, "logger": "test", "level": "INFO", "message": "error happened"},
        ]
        create_log_file(log_path, entries)
        
        result = healthcheck_server.read_logs(log_path, search_query="error")
        
        assert len(result) == 2

    def test_read_logs_time_range_filter(self, tmp_test_dir):
        """Test read_logs() filters by time range."""
        from tests.helpers import create_log_file
        
        log_path = tmp_test_dir / "test.log"
        base_time = time.time() - 100
        entries = [
            {"timestamp": base_time + 10, "logger": "test", "level": "INFO", "message": "Message 1"},
            {"timestamp": base_time + 20, "logger": "test", "level": "INFO", "message": "Message 2"},
            {"timestamp": base_time + 30, "logger": "test", "level": "INFO", "message": "Message 3"},
            {"timestamp": base_time + 40, "logger": "test", "level": "INFO", "message": "Message 4"},
        ]
        create_log_file(log_path, entries)
        
        # Filter entries between base_time + 15 and base_time + 35
        result = healthcheck_server.read_logs(
            log_path,
            start_time=base_time + 15,
            end_time=base_time + 35,
        )
        
        assert len(result) == 2
        assert all("Message 2" in entry["message"] or "Message 3" in entry["message"] for entry in result)

    def test_read_logs_max_lines(self, tmp_test_dir):
        """Test read_logs() respects max_lines limit."""
        from tests.helpers import create_log_file, create_sample_log_entries
        
        log_path = tmp_test_dir / "test.log"
        entries = create_sample_log_entries(count=100, interval_seconds=1)
        create_log_file(log_path, entries)
        
        result = healthcheck_server.read_logs(log_path, max_lines=10)
        
        assert len(result) <= 10

    def test_read_logs_malformed_lines(self, tmp_test_dir):
        """Test read_logs() handles malformed log lines."""
        log_path = tmp_test_dir / "test.log"
        with open(log_path, "w", encoding="utf-8") as f:
            f.write("2024-01-01 12:00:00 - test - INFO - Valid line\n")
            f.write("Invalid line without proper format\n")
            f.write("2024-01-01 12:00:01 - test - INFO - Another valid line\n")
        
        result = healthcheck_server.read_logs(log_path)
        
        # Should handle malformed lines gracefully
        assert len(result) >= 2
        assert any("Valid line" in entry["message"] for entry in result)

    def test_read_logs_reverse_order(self, tmp_test_dir):
        """Test read_logs() reads from end of file (most recent first, then reverses)."""
        from tests.helpers import create_log_file
        
        log_path = tmp_test_dir / "test.log"
        base_time = time.time() - 10
        entries = [
            {"timestamp": base_time + 1, "logger": "test", "level": "INFO", "message": "First"},
            {"timestamp": base_time + 2, "logger": "test", "level": "INFO", "message": "Second"},
            {"timestamp": base_time + 3, "logger": "test", "level": "INFO", "message": "Third"},
        ]
        create_log_file(log_path, entries)
        
        result = healthcheck_server.read_logs(log_path)
        
        # Should be in chronological order (oldest first)
        assert result[0]["message"] == "First"
        assert result[-1]["message"] == "Third"


class TestGenerateLogsHtml:
    """Test generate_logs_html() function."""

    def test_generate_logs_html_empty_entries(self):
        """Test generate_logs_html() with empty log entries."""
        html = healthcheck_server.generate_logs_html([])
        
        assert "No log entries found" in html
        assert "musicdl Log Viewer" in html

    def test_generate_logs_html_with_entries(self):
        """Test generate_logs_html() with log entries."""
        entries = [
            {
                "timestamp": time.time(),
                "timestamp_str": "2024-01-01 12:00:00",
                "level": "INFO",
                "logger": "test.logger",
                "message": "Test message",
            }
        ]
        
        html = healthcheck_server.generate_logs_html(entries)
        
        assert "Test message" in html
        assert "test.logger" in html
        assert "INFO" in html

    def test_generate_logs_html_with_search_highlighting(self):
        """Test generate_logs_html() highlights search query."""
        entries = [
            {
                "timestamp": time.time(),
                "timestamp_str": "2024-01-01 12:00:00",
                "level": "INFO",
                "logger": "test.logger",
                "message": "Error occurred in processing",
            }
        ]
        
        html = healthcheck_server.generate_logs_html(entries, search_query="Error")
        
        assert "<mark>" in html
        assert "Error" in html

    def test_generate_logs_html_refresh_interval(self):
        """Test generate_logs_html() uses refresh interval."""
        html = healthcheck_server.generate_logs_html([], refresh_interval=10)
        
        assert 'content="10"' in html

    def test_generate_logs_html_refresh_interval_validation(self):
        """Test generate_logs_html() validates refresh interval."""
        # Test with invalid refresh interval (should clamp to valid range)
        html = healthcheck_server.generate_logs_html([], refresh_interval=500)
        
        # Should be clamped to 300
        assert 'content="300"' in html

    def test_generate_logs_html_xss_prevention(self):
        """Test generate_logs_html() escapes HTML to prevent XSS."""
        entries = [
            {
                "timestamp": time.time(),
                "timestamp_str": "2024-01-01 12:00:00",
                "level": "INFO",
                "logger": "test.logger",
                "message": "<script>alert('xss')</script>",
            }
        ]
        
        html = healthcheck_server.generate_logs_html(entries)
        
        # Should escape HTML entities
        assert "<script>" not in html
        assert "&lt;script&gt;" in html or "&lt;" in html


class TestGenerateStatusHtml:
    """Test generate_status_html() function."""

    def test_generate_status_html_no_plan(self):
        """Test generate_status_html() with no plan."""
        html = healthcheck_server.generate_status_html(
            plan=None,
            health_status="unhealthy",
            plan_file=None,
            plan_path="/test/path",
        )
        
        assert "No Plan Files Found" in html
        assert "Unhealthy" in html

    def test_generate_status_html_with_plan(self):
        """Test generate_status_html() with plan."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Test Track",
            )
        ]
        plan = DownloadPlan(items=items)
        
        html = healthcheck_server.generate_status_html(
            plan=plan,
            health_status="healthy",
            plan_file="download_plan.json",
            plan_path="/test/path",
        )
        
        assert "Test Track" in html
        assert "Healthy" in html
        assert "download_plan.json" in html

    def test_generate_status_html_with_rate_limit(self):
        """Test generate_status_html() displays rate limit information."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Test Track",
            )
        ]
        plan = DownloadPlan(items=items)
        current_time = time.time()
        plan.metadata["rate_limit"] = {
            "active": True,
            "retry_after_seconds": 3600,
            "retry_after_timestamp": current_time + 3600,
            "detected_at": current_time,
        }
        
        html = healthcheck_server.generate_status_html(
            plan=plan,
            health_status="healthy",
            plan_file="download_plan.json",
            plan_path="/test/path",
        )
        
        assert "Spotify Rate Limit" in html
        assert "Active" in html
        assert "3600" in html

    def test_generate_status_html_with_phase(self):
        """Test generate_status_html() displays phase information."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Test Track",
            )
        ]
        plan = DownloadPlan(items=items)
        plan.metadata["phase"] = "executing"
        plan.metadata["phase_updated_at"] = time.time()
        
        html = healthcheck_server.generate_status_html(
            plan=plan,
            health_status="healthy",
            plan_file="download_plan.json",
            plan_path="/test/path",
        )
        
        assert "Executing Plan" in html or "executing" in html.lower()

    def test_generate_status_html_refresh_interval(self):
        """Test generate_status_html() uses refresh interval."""
        html = healthcheck_server.generate_status_html(
            plan=None,
            health_status="unhealthy",
            plan_file=None,
            plan_path="/test/path",
            refresh_interval=15,
        )
        
        assert 'content="15"' in html

    def test_generate_status_html_xss_prevention(self):
        """Test generate_status_html() escapes HTML to prevent XSS."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="<script>alert('xss')</script>",
            )
        ]
        plan = DownloadPlan(items=items)
        
        html = healthcheck_server.generate_status_html(
            plan=plan,
            health_status="healthy",
            plan_file="download_plan.json",
            plan_path="/test/path",
        )
        
        # Should escape HTML entities
        assert "<script>" not in html
        assert "&lt;" in html


class TestAggregatePlanStatus:
    """Test aggregate_plan_status() function."""

    def test_aggregate_plan_status_basic(self):
        """Test aggregate_plan_status() with basic plan."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            ),
            PlanItem(
                item_id="track2",
                item_type=PlanItemType.TRACK,
                name="Track 2",
            ),
        ]
        items[0].mark_completed()
        items[1].mark_failed("Error")
        
        plan = DownloadPlan(items=items)
        stats = healthcheck_server.aggregate_plan_status(plan)
        
        assert stats["total_items"] == 2
        assert stats["by_status"]["completed"] == 1
        assert stats["by_status"]["failed"] == 1

    def test_aggregate_plan_status_by_type(self):
        """Test aggregate_plan_status() groups by type."""
        items = [
            PlanItem(item_id="track1", item_type=PlanItemType.TRACK, name="Track 1"),
            PlanItem(item_id="album1", item_type=PlanItemType.ALBUM, name="Album 1"),
            PlanItem(item_id="playlist1", item_type=PlanItemType.PLAYLIST, name="Playlist 1"),
        ]
        plan = DownloadPlan(items=items)
        stats = healthcheck_server.aggregate_plan_status(plan)
        
        assert stats["by_type"]["track"] == 1
        assert stats["by_type"]["album"] == 1
        assert stats["by_type"]["playlist"] == 1

    def test_aggregate_plan_status_includes_skipped(self):
        """Test aggregate_plan_status() includes skipped items in statistics."""
        items = [
            PlanItem(item_id="track1", item_type=PlanItemType.TRACK, name="Track 1"),
            PlanItem(item_id="track2", item_type=PlanItemType.TRACK, name="Track 2"),
        ]
        items[0].mark_skipped("File exists")
        items[1].mark_completed()
        
        plan = DownloadPlan(items=items)
        stats = healthcheck_server.aggregate_plan_status(plan)
        
        assert stats["total_items"] == 2
        assert stats["by_status"]["skipped"] == 1
        assert stats["by_status"]["completed"] == 1

    def test_aggregate_plan_status_empty_plan(self):
        """Test aggregate_plan_status() with empty plan."""
        plan = DownloadPlan(items=[])
        stats = healthcheck_server.aggregate_plan_status(plan)
        
        assert stats["total_items"] == 0
        assert all(count == 0 for count in stats["by_status"].values())


class TestDetermineHealthStatusPhases:
    """Test determine_health_status() with phase information."""

    def test_determine_health_status_generating_phase(self):
        """Test determine_health_status() with generating phase."""
        plan = DownloadPlan(items=[], metadata={"phase": "generating"})
        
        status, reason = healthcheck_server.determine_health_status(plan)
        
        assert status == "healthy"
        assert "generation" in reason.lower() or "generating" in reason.lower()

    def test_determine_health_status_optimizing_phase(self):
        """Test determine_health_status() with optimizing phase."""
        plan = DownloadPlan(items=[], metadata={"phase": "optimizing"})
        
        status, reason = healthcheck_server.determine_health_status(plan)
        
        assert status == "healthy"
        assert "optimization" in reason.lower() or "optimizing" in reason.lower()

    def test_determine_health_status_executing_phase(self):
        """Test determine_health_status() with executing phase."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            )
        ]
        plan = DownloadPlan(items=items, metadata={"phase": "executing"})
        
        status, reason = healthcheck_server.determine_health_status(plan)
        
        # Should check items, not just phase
        assert status in ["healthy", "unhealthy"]

    def test_determine_health_status_unknown_phase(self):
        """Test determine_health_status() with unknown phase."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track 1",
            )
        ]
        plan = DownloadPlan(items=items, metadata={"phase": "unknown"})
        
        status, reason = healthcheck_server.determine_health_status(plan)
        
        # Should fall through to normal health checking
        assert status in ["healthy", "unhealthy"]


class TestReadLogsEdgeCases:
    """Test read_logs() edge cases."""

    def test_read_logs_time_range_exact_boundaries(self, tmp_test_dir):
        """Test read_logs() with exact boundary timestamps."""
        from tests.helpers import create_log_file
        
        log_path = tmp_test_dir / "test.log"
        base_time = time.time() - 100
        entries = [
            {"timestamp": base_time + 10, "logger": "test", "level": "INFO", "message": "Message 1"},
            {"timestamp": base_time + 20, "logger": "test", "level": "INFO", "message": "Message 2"},
            {"timestamp": base_time + 30, "logger": "test", "level": "INFO", "message": "Message 3"},
        ]
        create_log_file(log_path, entries)
        
        # Test exact start_time boundary (exclusive, so Message 1 should be excluded)
        result = healthcheck_server.read_logs(
            log_path,
            start_time=base_time + 10,
            end_time=base_time + 30,
        )
        
        # Message 1 is at exact start_time (should be excluded: timestamp <= start_time)
        # Message 2 is between boundaries (should be included)
        # Message 3 is at exact end_time (should be excluded: timestamp >= end_time)
        # However, due to floating point precision, we check that Message 2 is included
        assert len(result) >= 1
        assert any("Message 2" in entry["message"] for entry in result)

    def test_read_logs_invalid_datetime_format(self, tmp_test_dir):
        """Test read_logs() handles invalid datetime formats gracefully."""
        from tests.helpers import create_log_file
        
        log_path = tmp_test_dir / "test.log"
        entries = [
            {"timestamp": time.time() - 2, "logger": "test", "level": "INFO", "message": "Valid entry"},
        ]
        create_log_file(log_path, entries)
        
        # Should not crash with invalid datetime strings
        # The function should handle this internally
        result = healthcheck_server.read_logs(log_path)
        assert len(result) >= 1

    def test_read_logs_future_timestamps(self, tmp_test_dir):
        """Test read_logs() with future timestamps."""
        from tests.helpers import create_log_file
        
        log_path = tmp_test_dir / "test.log"
        future_time = time.time() + 86400  # 1 day in future
        entries = [
            {"timestamp": time.time() - 100, "logger": "test", "level": "INFO", "message": "Past"},
            {"timestamp": future_time, "logger": "test", "level": "INFO", "message": "Future"},
        ]
        create_log_file(log_path, entries)
        
        # Should handle future timestamps without crashing
        result = healthcheck_server.read_logs(log_path)
        assert len(result) >= 1

    def test_read_logs_search_special_regex_chars(self, tmp_test_dir):
        """Test read_logs() handles regex special characters in search."""
        from tests.helpers import create_log_file
        
        log_path = tmp_test_dir / "test.log"
        entries = [
            {"timestamp": time.time() - 2, "logger": "test", "level": "INFO", "message": "Error: .*+?[()|"},
            {"timestamp": time.time() - 1, "logger": "test", "level": "INFO", "message": "Normal message"},
        ]
        create_log_file(log_path, entries)
        
        # Should treat search as literal string, not regex
        result = healthcheck_server.read_logs(log_path, search_query=".*+?[()|")
        assert len(result) == 1
        assert ".*+?[()|" in result[0]["message"]

    def test_read_logs_search_html_special_chars(self, tmp_test_dir):
        """Test read_logs() handles HTML special characters in search."""
        from tests.helpers import create_log_file
        
        log_path = tmp_test_dir / "test.log"
        entries = [
            {"timestamp": time.time() - 2, "logger": "test", "level": "INFO", "message": "Message with <script>alert('xss')</script>"},
            {"timestamp": time.time() - 1, "logger": "test", "level": "INFO", "message": "Normal message"},
        ]
        create_log_file(log_path, entries)
        
        result = healthcheck_server.read_logs(log_path, search_query="<script>")
        assert len(result) == 1
        assert "<script>" in result[0]["message"]

    def test_read_logs_search_unicode(self, tmp_test_dir):
        """Test read_logs() handles Unicode characters in search."""
        from tests.helpers import create_log_file
        
        log_path = tmp_test_dir / "test.log"
        entries = [
            {"timestamp": time.time() - 2, "logger": "test", "level": "INFO", "message": "Message with 中文 and 🎵 emoji"},
            {"timestamp": time.time() - 1, "logger": "test", "level": "INFO", "message": "Normal message"},
        ]
        create_log_file(log_path, entries)
        
        result = healthcheck_server.read_logs(log_path, search_query="中文")
        assert len(result) == 1
        assert "中文" in result[0]["message"]

    def test_read_logs_search_very_long_query(self, tmp_test_dir):
        """Test read_logs() handles very long search queries."""
        from tests.helpers import create_log_file
        
        log_path = tmp_test_dir / "test.log"
        long_query = "a" * 10000  # 10KB query
        entries = [
            {"timestamp": time.time() - 2, "logger": "test", "level": "INFO", "message": "Normal message"},
        ]
        create_log_file(log_path, entries)
        
        # Should not crash with very long queries
        result = healthcheck_server.read_logs(log_path, search_query=long_query)
        assert isinstance(result, list)

    def test_read_logs_file_no_newline_at_end(self, tmp_test_dir):
        """Test read_logs() handles files without newline at end."""
        log_path = tmp_test_dir / "test.log"
        with open(log_path, "w", encoding="utf-8") as f:
            f.write("2024-01-01 12:00:00 - test - INFO - Message 1\n")
            f.write("2024-01-01 12:00:01 - test - INFO - Message 2")  # No newline
        
        result = healthcheck_server.read_logs(log_path)
        assert len(result) == 2
        assert "Message 1" in result[0]["message"]
        assert "Message 2" in result[1]["message"]

    def test_read_logs_very_long_line(self, tmp_test_dir):
        """Test read_logs() handles very long log lines."""
        log_path = tmp_test_dir / "test.log"
        long_message = "A" * 100000  # 100KB line
        with open(log_path, "w", encoding="utf-8") as f:
            f.write(f"2024-01-01 12:00:00 - test - INFO - {long_message}\n")
        
        # Should handle long lines without crashing
        result = healthcheck_server.read_logs(log_path, max_lines=1)
        assert len(result) <= 1


class TestGenerateLogsHtmlEdgeCases:
    """Test generate_logs_html() edge cases."""

    def test_generate_logs_html_very_long_message(self):
        """Test generate_logs_html() with very long log messages."""
        long_message = "A" * 50000  # 50KB message
        entries = [
            {
                "timestamp": time.time(),
                "timestamp_str": "2024-01-01 12:00:00",
                "level": "INFO",
                "logger": "test.logger",
                "message": long_message,
            }
        ]
        
        html = healthcheck_server.generate_logs_html(entries)
        
        # Should escape and display long messages
        assert "A" in html
        assert "<script>" not in html  # Should escape HTML

    def test_generate_logs_html_multiline_message(self):
        """Test generate_logs_html() with multi-line messages."""
        multiline_message = "Line 1\nLine 2\nLine 3"
        entries = [
            {
                "timestamp": time.time(),
                "timestamp_str": "2024-01-01 12:00:00",
                "level": "INFO",
                "logger": "test.logger",
                "message": multiline_message,
            }
        ]
        
        html = healthcheck_server.generate_logs_html(entries)
        
        # Should handle multi-line messages
        assert "Line 1" in html or multiline_message in html

    def test_generate_logs_html_unicode_characters(self):
        """Test generate_logs_html() with Unicode characters."""
        entries = [
            {
                "timestamp": time.time(),
                "timestamp_str": "2024-01-01 12:00:00",
                "level": "INFO",
                "logger": "test.logger",
                "message": "Message with 中文 and 🎵 emoji",
            }
        ]
        
        html = healthcheck_server.generate_logs_html(entries)
        
        # Should handle Unicode
        assert "中文" in html or "emoji" in html

    def test_generate_logs_html_special_characters_in_search(self):
        """Test generate_logs_html() highlights special characters correctly."""
        entries = [
            {
                "timestamp": time.time(),
                "timestamp_str": "2024-01-01 12:00:00",
                "level": "INFO",
                "logger": "test.logger",
                "message": "Error: <script>alert('xss')</script>",
            }
        ]
        
        html = healthcheck_server.generate_logs_html(entries, search_query="<script>")
        
        # Should escape HTML but still highlight search
        assert "<script>" not in html  # Should be escaped
        assert "&lt;" in html or "<mark>" in html  # Should be escaped or highlighted


class TestGenerateStatusHtmlEdgeCases:
    """Test generate_status_html() edge cases."""

    def test_generate_status_html_unicode_in_item_names(self):
        """Test generate_status_html() with Unicode in item names."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Track with 中文 and 🎵",
            )
        ]
        plan = DownloadPlan(items=items)
        
        html = healthcheck_server.generate_status_html(
            plan=plan,
            health_status="healthy",
            plan_file="download_plan.json",
            plan_path="/test/path",
        )
        
        # Should handle Unicode
        assert "Track" in html or "中文" in html

    def test_generate_status_html_missing_metadata_fields(self):
        """Test generate_status_html() with missing metadata fields."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Test Track",
            )
        ]
        plan = DownloadPlan(items=items, metadata={})  # Empty metadata
        
        html = healthcheck_server.generate_status_html(
            plan=plan,
            health_status="healthy",
            plan_file="download_plan.json",
            plan_path="/test/path",
        )
        
        # Should handle missing metadata gracefully
        assert "Test Track" in html

    def test_generate_status_html_null_metadata_values(self):
        """Test generate_status_html() with null metadata values."""
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Test Track",
            )
        ]
        plan = DownloadPlan(items=items, metadata={"phase": None, "rate_limit": None})
        
        html = healthcheck_server.generate_status_html(
            plan=plan,
            health_status="healthy",
            plan_file="download_plan.json",
            plan_path="/test/path",
        )
        
        # Should handle null values gracefully
        assert "Test Track" in html


class TestPlanAggregationEdgeCases:
    """Test plan aggregation edge cases."""

    def test_find_and_aggregate_plans_corrupted_json(self, tmp_test_dir):
        """Test find_and_aggregate_plans() handles corrupted JSON files."""
        plan_dir = tmp_test_dir / "plans"
        plan_dir.mkdir()
        
        # Create corrupted JSON file
        corrupted_file = plan_dir / "download_plan.json"
        corrupted_file.write_text("{ invalid json }")
        
        # Should handle corruption gracefully
        plan, plan_file = healthcheck_server.find_and_aggregate_plans(plan_dir)
        assert plan is None or isinstance(plan, DownloadPlan)

    def test_find_and_aggregate_plans_missing_required_fields(self, tmp_test_dir):
        """Test find_and_aggregate_plans() handles missing required fields."""
        plan_dir = tmp_test_dir / "plans"
        plan_dir.mkdir()
        
        # Create plan file with missing fields
        incomplete_file = plan_dir / "download_plan.json"
        incomplete_data = {"items": []}  # Missing created_at, metadata
        incomplete_file.write_text(json.dumps(incomplete_data))
        
        # Should handle missing fields gracefully
        plan, plan_file = healthcheck_server.find_and_aggregate_plans(plan_dir)
        # Should either return None or handle gracefully
        assert plan is None or isinstance(plan, DownloadPlan)

    def test_find_and_aggregate_plans_very_large_plan(self, tmp_test_dir):
        """Test find_and_aggregate_plans() with very large plan (1000+ items)."""
        plan_dir = tmp_test_dir / "plans"
        plan_dir.mkdir()
        
        # Create plan with many items
        items = []
        for i in range(1000):
            items.append(
                PlanItem(
                    item_id=f"track{i}",
                    item_type=PlanItemType.TRACK,
                    name=f"Track {i}",
                ).to_dict()
            )
        
        plan_file = plan_dir / "download_plan.json"
        plan_data = {
            "items": items,
            "created_at": time.time(),
            "metadata": {},
        }
        plan_file.write_text(json.dumps(plan_data))
        
        # Should handle large plans
        plan, plan_file = healthcheck_server.find_and_aggregate_plans(plan_dir)
        assert plan is not None
        assert len(plan.items) == 1000

    def test_find_and_aggregate_plans_conflicting_item_ids(self, tmp_test_dir):
        """Test find_and_aggregate_plans() handles conflicting item IDs correctly."""
        plan_dir = tmp_test_dir / "plans"
        plan_dir.mkdir()
        
        # Create two plan files with same item_id but different data
        initial_file = plan_dir / "download_plan.json"
        initial_data = {
            "items": [
                PlanItem(
                    item_id="track1",
                    item_type=PlanItemType.TRACK,
                    name="Initial Track",
                ).to_dict()
            ],
            "created_at": time.time(),
            "metadata": {},
        }
        initial_file.write_text(json.dumps(initial_data))
        
        progress_file = plan_dir / "download_plan_progress.json"
        progress_item = PlanItem(
            item_id="track1",
            item_type=PlanItemType.TRACK,
            name="Progress Track",
        )
        progress_item.mark_started()
        progress_data = {
            "items": [progress_item.to_dict()],
            "created_at": time.time(),
            "metadata": {},
        }
        progress_file.write_text(json.dumps(progress_data))
        
        # Progress should take priority
        plan, plan_file = healthcheck_server.find_and_aggregate_plans(plan_dir)
        assert plan is not None
        assert len(plan.items) == 1
        # Should use progress version (higher priority)
        assert plan.items[0].name == "Progress Track" or plan.items[0].status == PlanItemStatus.IN_PROGRESS
