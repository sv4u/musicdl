package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHealthStats(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create valid config
	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
  threads: 4
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	startTime := time.Now()
	handlers, err := NewHandlers(configPath, planPath, logPath, startTime, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test HealthStats endpoint
	req := httptest.NewRequest("GET", "/api/health/stats", nil)
	w := httptest.NewRecorder()
	handlers.HealthStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("HealthStats() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if response["server_health"] != "healthy" {
		t.Errorf("Expected server_health 'healthy', got %v", response["server_health"])
	}

	if response["uptime_seconds"] == nil {
		t.Error("Expected uptime_seconds in response")
	}

	uptime, ok := response["uptime_seconds"].(float64)
	if !ok {
		// Try int64
		uptimeInt, ok := response["uptime_seconds"].(int64)
		if !ok {
			t.Error("uptime_seconds should be a number")
		} else {
			uptime = float64(uptimeInt)
		}
	}

	if uptime < 0 {
		t.Error("uptime_seconds should be non-negative")
	}

	// Verify statistics structure
	stats, ok := response["statistics"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected 'statistics' object in response")
	}

	if stats["total_processed"] == nil {
		t.Error("Expected total_processed in statistics")
	}
}

func TestHealth_WithService(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create valid config
	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
  threads: 4
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test Health endpoint (service may or may not be running)
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	handlers.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response structure
	if response["status"] == nil {
		t.Error("Expected 'status' in response")
	}

	if response["server_health"] != "healthy" {
		t.Errorf("Expected server_health 'healthy', got %v", response["server_health"])
	}
}
