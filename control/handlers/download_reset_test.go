package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sv4u/musicdl/control/service"
)

func TestDownloadReset(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "test.log")

	// Create config file
	configData := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
  format: "mp3"
  bitrate: "128k"
  audio_providers:
    - "youtube-music"
  overwrite: "skip"
`
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Create plan directory and files
	if err := os.MkdirAll(planPath, 0755); err != nil {
		t.Fatalf("Failed to create plan directory: %v", err)
	}

	planFile := filepath.Join(planPath, "download_plan_progress.json")
	planData := `{"items": [], "metadata": {}}`
	if err := os.WriteFile(planFile, []byte(planData), 0644); err != nil {
		t.Fatalf("Failed to create plan file: %v", err)
	}

	// Create handlers
	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Queue a pending config update
	cfg, err := handlers.configManager.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	cfg.Download.Threads = 8
	if err := handlers.configManager.QueueUpdate(cfg); err != nil {
		t.Fatalf("Failed to queue update: %v", err)
	}

	// Verify plan file exists
	if _, err := os.Stat(planFile); err != nil {
		t.Fatalf("Plan file should exist before reset: %v", err)
	}

	// Verify pending update exists
	_, hasPending := handlers.configManager.GetPendingUpdate()
	if !hasPending {
		t.Fatal("Expected pending update before reset")
	}

	// Create request
	req := httptest.NewRequest("POST", "/api/download/reset", nil)
	w := httptest.NewRecorder()

	// Call handler
	handlers.DownloadReset(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response["message"] == nil {
		t.Error("Response should contain 'message' field")
	}

	// Verify plan file was deleted
	if _, err := os.Stat(planFile); !os.IsNotExist(err) {
		t.Error("Plan file should be deleted after reset")
	}

	// Verify pending update was cleared
	_, hasPending = handlers.configManager.GetPendingUpdate()
	if hasPending {
		t.Error("Pending update should be cleared after reset")
	}
}

func TestDownloadReset_WithRunningService(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "test.log")

	// Create config file
	configData := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
  format: "mp3"
  bitrate: "128k"
  audio_providers:
    - "youtube-music"
  overwrite: "skip"
`
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Create handlers
	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Simulate running service (we can't actually start it in unit tests)
	// The handler should handle the case gracefully
	handlers.serviceMu.Lock()
	handlers.serviceManager = service.NewManager("localhost:30025", "v1.0.0", planPath, logPath)
	handlers.serviceMu.Unlock()

	// Create request
	req := httptest.NewRequest("POST", "/api/download/reset", nil)
	w := httptest.NewRecorder()

	// Call handler
	handlers.DownloadReset(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestDownloadReset_ClearsMultiplePlanFiles(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "test.log")

	// Create config file
	configData := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
  format: "mp3"
  bitrate: "128k"
  audio_providers:
    - "youtube-music"
  overwrite: "skip"
`
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Create plan directory and files
	if err := os.MkdirAll(planPath, 0755); err != nil {
		t.Fatalf("Failed to create plan directory: %v", err)
	}

	planFiles := []string{
		filepath.Join(planPath, "download_plan_progress.json"),
		filepath.Join(planPath, "download_plan.json"),
	}

	for _, file := range planFiles {
		if err := os.WriteFile(file, []byte(`{"items": []}`), 0644); err != nil {
			t.Fatalf("Failed to create plan file %s: %v", file, err)
		}
	}

	// Create handlers
	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Create request
	req := httptest.NewRequest("POST", "/api/download/reset", nil)
	w := httptest.NewRecorder()

	// Call handler
	handlers.DownloadReset(w, req)

	// Verify all plan files were deleted
	for _, file := range planFiles {
		if _, err := os.Stat(file); !os.IsNotExist(err) {
			t.Errorf("Plan file %s should be deleted after reset", file)
		}
	}
}

func TestDownloadReset_HandlesMissingFiles(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "test.log")

	// Create config file
	configData := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
  format: "mp3"
  bitrate: "128k"
  audio_providers:
    - "youtube-music"
  overwrite: "skip"
`
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Create plan directory (but no files)
	if err := os.MkdirAll(planPath, 0755); err != nil {
		t.Fatalf("Failed to create plan directory: %v", err)
	}

	// Create handlers
	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Create request
	req := httptest.NewRequest("POST", "/api/download/reset", nil)
	w := httptest.NewRecorder()

	// Call handler - should not error even if files don't exist
	handlers.DownloadReset(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}
