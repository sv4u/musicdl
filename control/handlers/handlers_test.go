package handlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewHandlers(t *testing.T) {
	// Create temporary directories for testing
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create config file
	if err := os.WriteFile(configPath, []byte("version: \"1.2\"\n"), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	// Test successful creation
	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}
	if handlers == nil {
		t.Fatal("NewHandlers() returned nil")
	}

	// Test with non-existent config file
	_, err = NewHandlers("/nonexistent/config.yaml", planPath, logPath, time.Now())
	if err == nil {
		t.Error("NewHandlers() should fail with non-existent config file")
	}
}

func TestHealth(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	os.WriteFile(configPath, []byte("version: \"1.2\"\n"), 0644)

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test Health endpoint
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()
	handlers.Health(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Health() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("Health() returned Content-Type %s, expected application/json", w.Header().Get("Content-Type"))
	}
}

func TestDashboard(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	os.WriteFile(configPath, []byte("version: \"1.2\"\n"), 0644)

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test Dashboard endpoint
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	handlers.Dashboard(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Dashboard() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("Dashboard() returned Content-Type %s, expected text/html; charset=utf-8", w.Header().Get("Content-Type"))
	}
}
