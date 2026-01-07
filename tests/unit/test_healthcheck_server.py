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

