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

func TestStatus(t *testing.T) {
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

	// Test Status before service initialization
	req := httptest.NewRequest("GET", "/api/status", nil)
	w := httptest.NewRecorder()
	handlers.Status(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Status() returned status %d, expected %d", w.Code, http.StatusOK)
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

	req2 := httptest.NewRequest("GET", "/api/status", nil)
	w2 := httptest.NewRecorder()
	handlers.Status(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Status() returned status %d, expected %d", w2.Code, http.StatusOK)
	}

	var response2 map[string]interface{}
	if err := json.NewDecoder(w2.Body).Decode(&response2); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Should return service status
	if response2["state"] == nil {
		t.Error("Expected state in response")
	}
}

func TestStatusPage(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	os.WriteFile(configPath, []byte("version: \"1.2\"\n"), 0644)

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test StatusPage endpoint
	req := httptest.NewRequest("GET", "/status", nil)
	w := httptest.NewRecorder()
	handlers.StatusPage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("StatusPage() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("StatusPage() returned Content-Type %s, expected text/html; charset=utf-8", w.Header().Get("Content-Type"))
	}

	// Verify HTML content
	body := w.Body.String()
	if len(body) == 0 {
		t.Error("StatusPage() returned empty body")
	}
	if !contains(body, "musicdl Control Platform") {
		t.Error("StatusPage() should contain 'musicdl Control Platform'")
	}
	if !contains(body, "Service Status") {
		t.Error("StatusPage() should contain 'Service Status'")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
