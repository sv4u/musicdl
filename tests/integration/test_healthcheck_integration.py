"""
Integration tests for healthcheck server HTTP endpoints.

Tests the actual HTTP server functionality with real HTTP requests.
"""

import json
import os
import pytest
import sys
import threading
import time
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch

import requests

# Add project root to path
project_root = Path(__file__).parent.parent.parent
sys.path.insert(0, str(project_root))

from core.plan import DownloadPlan, PlanItem, PlanItemStatus, PlanItemType


class TestHealthcheckServerIntegration:
    """Integration tests for healthcheck server HTTP endpoints."""

    @pytest.fixture
    def plan_dir(self, tmp_test_dir):
        """Create temporary plan directory."""
        plan_dir = tmp_test_dir / "plans"
        plan_dir.mkdir()
        return plan_dir

    @pytest.fixture
    def healthcheck_server(self, plan_dir, monkeypatch):
        """Start healthcheck server in background for testing."""
        # Use a unique port for each test to avoid conflicts
        import socket
        with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as s:
            s.bind(('', 0))
            test_port = s.getsockname()[1]
        
        # Set environment variables BEFORE importing module
        # This ensures the server reads the correct plan directory when it starts
        monkeypatch.setenv("MUSICDL_PLAN_PATH", str(plan_dir))
        monkeypatch.setenv("HEALTHCHECK_PORT", str(test_port))
        
        # Import and start server
        import importlib.util
        scripts_path = project_root / "scripts"
        healthcheck_server_path = scripts_path / "healthcheck_server.py"
        spec = importlib.util.spec_from_file_location("healthcheck_server", healthcheck_server_path)
        healthcheck_module = importlib.util.module_from_spec(spec)
        
        # Temporarily modify sys.path for imports
        original_path = sys.path.copy()
        sys.path.insert(0, str(project_root))
        try:
            spec.loader.exec_module(healthcheck_module)
        finally:
            sys.path = original_path
        
        # Start server in background thread
        server_thread = threading.Thread(
            target=healthcheck_module.main,
            daemon=True,
        )
        server_thread.start()
        
        # Wait for server to start and verify it's responding
        max_wait = 20
        server_ready = False
        for _ in range(max_wait):
            try:
                response = requests.get(f"http://localhost:{test_port}/health", timeout=1.0)
                if response.status_code in (200, 503):  # Server is responding
                    server_ready = True
                    break
            except (requests.exceptions.ConnectionError, requests.exceptions.ReadTimeout):
                pass
            time.sleep(0.1)
        if not server_ready:
            pytest.fail("Healthcheck server did not start within 2 seconds")
        
        # Store the port in the module for tests to use
        healthcheck_module._test_port = test_port
        
        yield healthcheck_module
        
        # Stop the server gracefully
        # Access the global server variable from the module
        if hasattr(healthcheck_module, 'server') and healthcheck_module.server:
            try:
                # Shutdown the server
                healthcheck_module.server.shutdown()
                # Wait for the server thread to finish
                server_thread.join(timeout=2.0)
            except Exception:
                pass

    def test_health_endpoint_no_plan(self, healthcheck_server, plan_dir):
        """Test /health endpoint when no plan files exist."""
        port = healthcheck_server._test_port
        response = requests.get(f"http://localhost:{port}/health", timeout=2)
        
        assert response.status_code == 503  # Unhealthy
        data = response.json()
        assert data["status"] == "unhealthy"
        assert "No plan files found" in data["reason"]

    def test_health_endpoint_with_plan(self, healthcheck_server, plan_dir):
        """Test /health endpoint with valid plan file."""
        # Create a plan file
        plan_file = plan_dir / "download_plan.json"
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Test Track",
            ).to_dict()
        ]
        plan_data = {
            "items": items,
            "created_at": time.time(),
            "metadata": {},
        }
        plan_file.write_text(json.dumps(plan_data))
        
        # Wait a bit for file to be written
        time.sleep(0.2)
        
        port = healthcheck_server._test_port
        response = requests.get(f"http://localhost:{port}/health", timeout=2)
        
        assert response.status_code == 200  # Healthy
        data = response.json()
        assert data["status"] == "healthy"
        assert "plan_file" in data
        assert "statistics" in data

    def test_health_endpoint_all_failed(self, healthcheck_server, plan_dir):
        """Test /health endpoint when all items failed."""
        # Create a plan file with all failed items
        plan_file = plan_dir / "download_plan.json"
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Failed Track",
            )
        ]
        items[0].mark_failed("Error message")
        plan_data = {
            "items": [item.to_dict() for item in items],
            "created_at": time.time(),
            "metadata": {},
        }
        plan_file.write_text(json.dumps(plan_data))
        
        time.sleep(0.2)
        
        port = healthcheck_server._test_port
        response = requests.get(f"http://localhost:{port}/health", timeout=2)
        
        assert response.status_code == 503  # Unhealthy
        data = response.json()
        assert data["status"] == "unhealthy"
        assert "failed" in data["reason"].lower()

    def test_status_endpoint_no_plan(self, healthcheck_server, plan_dir):
        """Test /status endpoint when no plan files exist."""
        port = healthcheck_server._test_port
        response = requests.get(f"http://localhost:{port}/status", timeout=2)
        
        assert response.status_code == 200
        assert "text/html" in response.headers["Content-Type"]
        assert "No Plan Files Found" in response.text or "no plan files" in response.text.lower()

    def test_status_endpoint_with_plan(self, healthcheck_server, plan_dir):
        """Test /status endpoint with valid plan file."""
        # Create a plan file
        plan_file = plan_dir / "download_plan.json"
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Test Track",
            ).to_dict()
        ]
        plan_data = {
            "items": items,
            "created_at": time.time(),
            "metadata": {},
        }
        plan_file.write_text(json.dumps(plan_data))
        
        time.sleep(0.2)
        
        port = healthcheck_server._test_port
        response = requests.get(f"http://localhost:{port}/status", timeout=2)
        
        assert response.status_code == 200
        assert "text/html" in response.headers["Content-Type"]
        assert "musicdl Status Dashboard" in response.text
        assert "Test Track" in response.text

    def test_status_endpoint_refresh_parameter(self, healthcheck_server, plan_dir):
        """Test /status endpoint with refresh query parameter."""
        port = healthcheck_server._test_port
        response = requests.get(f"http://localhost:{port}/status?refresh=5", timeout=2)
        
        assert response.status_code == 200
        # Check that refresh interval is set to 5 seconds
        assert 'content="5"' in response.text or 'http-equiv="refresh"' in response.text

    def test_unknown_endpoint(self, healthcheck_server, plan_dir):
        """Test that unknown endpoints return 404."""
        port = healthcheck_server._test_port
        response = requests.get(f"http://localhost:{port}/unknown", timeout=2)
        
        assert response.status_code == 404
        data = response.json()
        assert "error" in data
        assert "Not found" in data["error"]

    def test_health_endpoint_statistics(self, healthcheck_server, plan_dir):
        """Test /health endpoint includes correct statistics."""
        # Create a plan with multiple items in different states
        plan_file = plan_dir / "download_plan.json"
        items = [
            PlanItem(
                item_id="track1",
                item_type=PlanItemType.TRACK,
                name="Completed Track",
            ),
            PlanItem(
                item_id="track2",
                item_type=PlanItemType.TRACK,
                name="Failed Track",
            ),
            PlanItem(
                item_id="track3",
                item_type=PlanItemType.TRACK,
                name="Pending Track",
            ),
        ]
        items[0].mark_completed()
        items[1].mark_failed("Error")
        # items[2] remains pending
        
        plan_data = {
            "items": [item.to_dict() for item in items],
            "created_at": time.time(),
            "metadata": {},
        }
        plan_file.write_text(json.dumps(plan_data))
        
        time.sleep(0.2)
        
        port = healthcheck_server._test_port
        response = requests.get(f"http://localhost:{port}/health", timeout=2)
        
        assert response.status_code == 200
        data = response.json()
        assert "statistics" in data
        stats = data["statistics"]
        assert "total_items" in stats
        assert "by_status" in stats
        assert stats["by_status"]["completed"] == 1
        assert stats["by_status"]["failed"] == 1
        assert stats["by_status"]["pending"] == 1

