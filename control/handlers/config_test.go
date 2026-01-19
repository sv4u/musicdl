package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestConfigGet(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create test config file
	configYAML := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
  threads: 4
  format: "mp3"
songs: []
artists: []
playlists: []
albums: []
`
	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create handlers
	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	// Create request
	req := httptest.NewRequest("GET", "/api/config", nil)
	w := httptest.NewRecorder()

	// Call handler
	handlers.ConfigGet(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/x-yaml" {
		t.Errorf("Expected Content-Type application/x-yaml, got %s", w.Header().Get("Content-Type"))
	}

	// Check body contains config
	body := w.Body.String()
	if !bytes.Contains([]byte(body), []byte("version: \"1.2\"")) {
		t.Error("Response body should contain config YAML")
	}
}

func TestConfigPut_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create initial config file
	initialConfig := `version: "1.2"
download:
  client_id: "old_id"
  client_secret: "old_secret"
songs: []
artists: []
playlists: []
albums: []
`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create handlers
	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	// Create new config
	newConfig := `version: "1.2"
download:
  client_id: "new_id"
  client_secret: "new_secret"
  threads: 8
  format: "flac"
songs: []
artists: []
playlists: []
albums: []
`

	// Create request
	req := httptest.NewRequest("PUT", "/api/config", bytes.NewReader([]byte(newConfig)))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	// Call handler
	handlers.ConfigPut(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["message"] != "Config updated successfully" {
		t.Errorf("Expected success message, got %v", response["message"])
	}

	// Verify file was updated
	updatedData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read updated config: %v", err)
	}

	if !bytes.Contains(updatedData, []byte("new_id")) {
		t.Error("Config file should contain new_id")
	}
}

func TestConfigPut_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create initial config file
	initialConfig := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
songs: []
artists: []
playlists: []
albums: []
`
	if err := os.WriteFile(configPath, []byte(initialConfig), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	// Create handlers
	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	// Create invalid config (missing credentials)
	invalidConfig := `version: "1.2"
download:
  threads: 4
songs: []
artists: []
playlists: []
albums: []
`

	// Create request
	req := httptest.NewRequest("PUT", "/api/config", bytes.NewReader([]byte(invalidConfig)))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	// Call handler
	handlers.ConfigPut(w, req)

	// Check response
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["error"] == nil {
		t.Error("Response should contain error")
	}

	// Verify file was NOT updated
	originalData, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	if !bytes.Contains(originalData, []byte("test_id")) {
		t.Error("Config file should still contain original values")
	}
}

func TestConfigValidate_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create dummy config file (required by NewHandlers validation)
	dummyConfig := `version: "1.2"
download:
  client_id: "dummy"
  client_secret: "dummy"
songs: []
artists: []
playlists: []
albums: []
`
	if err := os.WriteFile(configPath, []byte(dummyConfig), 0644); err != nil {
		t.Fatalf("Failed to write dummy config file: %v", err)
	}

	// Create handlers
	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	// Create valid config
	validConfig := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
  threads: 4
  format: "mp3"
songs: []
artists: []
playlists: []
albums: []
`

	// Create request
	req := httptest.NewRequest("POST", "/api/config/validate", bytes.NewReader([]byte(validConfig)))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	// Call handler
	handlers.ConfigValidate(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["valid"] != true {
		t.Errorf("Expected valid=true, got %v", response["valid"])
	}
}

func TestConfigValidate_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create dummy config file (required by NewHandlers validation)
	dummyConfig := `version: "1.2"
download:
  client_id: "dummy"
  client_secret: "dummy"
songs: []
artists: []
playlists: []
albums: []
`
	if err := os.WriteFile(configPath, []byte(dummyConfig), 0644); err != nil {
		t.Fatalf("Failed to write dummy config file: %v", err)
	}

	// Create handlers
	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	// Create invalid config (missing credentials)
	invalidConfig := `version: "1.2"
download:
  threads: 4
songs: []
artists: []
playlists: []
albums: []
`

	// Create request
	req := httptest.NewRequest("POST", "/api/config/validate", bytes.NewReader([]byte(invalidConfig)))
	req.Header.Set("Content-Type", "application/x-yaml")
	w := httptest.NewRecorder()

	// Call handler
	handlers.ConfigValidate(w, req)

	// Check response
	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200 (validation endpoint returns 200 even if invalid), got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	if response["valid"] != false {
		t.Errorf("Expected valid=false, got %v", response["valid"])
	}

	if response["error"] == nil {
		t.Error("Response should contain error message")
	}
}
