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

func TestConfigDigest(t *testing.T) {
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
songs:
  - name: "Song 1"
    url: "https://open.spotify.com/track/1"
artists:
  - name: "Artist 1"
    url: "https://open.spotify.com/artist/1"
`
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Create handlers
	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Create request
	req := httptest.NewRequest("GET", "/api/config/digest", nil)
	w := httptest.NewRecorder()

	// Call handler
	handlers.ConfigDigest(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify response fields
	if response["digest"] == nil {
		t.Error("Response should contain 'digest' field")
	}

	if response["version"] != "1.2" {
		t.Errorf("Expected version '1.2', got '%v'", response["version"])
	}

	if response["has_pending"] == nil {
		t.Error("Response should contain 'has_pending' field")
	}

	configStats, ok := response["config_stats"].(map[string]interface{})
	if !ok {
		t.Error("Response should contain 'config_stats' field")
	}

	if configStats["songs"] == nil {
		t.Error("config_stats should contain 'songs' field")
	}
}

func TestConfigDigest_WithPendingUpdate(t *testing.T) {
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

	// Queue a pending update
	cfg, err := handlers.configManager.Get()
	if err != nil {
		t.Fatalf("Failed to get config: %v", err)
	}
	cfg.Download.Threads = 8
	if err := handlers.configManager.QueueUpdate(cfg); err != nil {
		t.Fatalf("Failed to queue update: %v", err)
	}

	// Create request
	req := httptest.NewRequest("GET", "/api/config/digest", nil)
	w := httptest.NewRecorder()

	// Call handler
	handlers.ConfigDigest(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify has_pending is true
	if response["has_pending"] != true {
		t.Errorf("Expected has_pending true, got %v", response["has_pending"])
	}
}
