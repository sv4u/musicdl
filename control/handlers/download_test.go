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

func TestDownloadStart(t *testing.T) {
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

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test DownloadStart
	req := httptest.NewRequest("POST", "/api/download/start", nil)
	w := httptest.NewRecorder()
	handlers.DownloadStart(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("DownloadStart() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["status"] != "running" {
		t.Errorf("Expected status 'running', got %v", response["status"])
	}
}

func TestDownloadStart_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create invalid config (missing credentials)
	cfg := `version: "1.2"
download:
  threads: 4
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test DownloadStart with invalid config
	req := httptest.NewRequest("POST", "/api/download/start", nil)
	w := httptest.NewRecorder()
	handlers.DownloadStart(w, req)

	// Should fail with internal server error
	if w.Code != http.StatusInternalServerError {
		t.Errorf("DownloadStart() returned status %d, expected %d", w.Code, http.StatusInternalServerError)
	}
}

func TestDownloadStop(t *testing.T) {
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

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Initialize service (but don't start it)
	_, err = handlers.getService()
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	// Test DownloadStop on idle service
	// Note: Stopping an idle service returns an error, which the handler converts to 400
	req := httptest.NewRequest("POST", "/api/download/stop", nil)
	w := httptest.NewRecorder()
	handlers.DownloadStop(w, req)

	// Handler returns 400 when service is not running
	if w.Code != http.StatusBadRequest {
		t.Errorf("DownloadStop() returned status %d, expected %d (service is idle)", w.Code, http.StatusBadRequest)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should have error message
	if response["error"] == nil {
		t.Error("Expected error message in response")
	}
}

func TestDownloadStatus(t *testing.T) {
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

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test DownloadStatus before service initialization
	req := httptest.NewRequest("GET", "/api/download/status", nil)
	w := httptest.NewRecorder()
	handlers.DownloadStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("DownloadStatus() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should return idle state when not initialized
	if response["state"] != "idle" {
		t.Errorf("Expected state 'idle', got %v", response["state"])
	}

	// Initialize service and test again
	_, err = handlers.getService()
	if err != nil {
		t.Fatalf("Failed to get service: %v", err)
	}

	req2 := httptest.NewRequest("GET", "/api/download/status", nil)
	w2 := httptest.NewRecorder()
	handlers.DownloadStatus(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("DownloadStatus() returned status %d, expected %d", w2.Code, http.StatusOK)
	}

	var response2 map[string]interface{}
	if err := json.NewDecoder(w2.Body).Decode(&response2); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should return service status
	if response2["state"] == nil {
		t.Error("Expected state in response")
	}

	// Verify state is valid
	state := response2["state"].(string)
	validStates := map[string]bool{
		"idle":      true,
		"running":   true,
		"stopping":  true,
		"completed": true,
		"error":     true,
	}
	if !validStates[state] {
		t.Errorf("Invalid state: %s", state)
	}
}

func TestGetService(t *testing.T) {
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

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test lazy initialization
	service1, err := handlers.getService()
	if err != nil {
		t.Fatalf("getService() failed: %v", err)
	}
	if service1 == nil {
		t.Fatal("getService() returned nil")
	}

	// Test that subsequent calls return the same service (lazy init)
	service2, err := handlers.getService()
	if err != nil {
		t.Fatalf("getService() failed: %v", err)
	}
	if service1 != service2 {
		t.Error("getService() should return the same instance on subsequent calls")
	}
}

func TestGetService_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create invalid config (missing credentials)
	cfg := `version: "1.2"
download:
  threads: 4
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test that getService fails with invalid config
	_, err = handlers.getService()
	if err == nil {
		t.Error("getService() should fail with invalid config")
	}
}
