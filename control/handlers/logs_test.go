package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create log file with test entries
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		t.Fatalf("Failed to create log directory: %v", err)
	}

	logContent := `2024/01/11 10:00:00 INFO: Test log entry 1
2024/01/11 10:01:00 ERROR: Test error entry
2024/01/11 10:02:00 WARN: Test warning entry
2024/01/11 10:03:00 INFO: Test log entry 2
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("Failed to create log file: %v", err)
	}

	os.WriteFile(configPath, []byte("version: \"1.2\"\n"), 0644)

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test Logs endpoint - all logs
	req := httptest.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()
	handlers.Logs(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Logs() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	logs, ok := response["logs"].([]interface{})
	if !ok {
		t.Fatal("Expected 'logs' array in response")
	}

	if len(logs) == 0 {
		t.Error("Expected at least one log entry")
	}

	// Test Logs endpoint - filter by level
	req2 := httptest.NewRequest("GET", "/api/logs?level=ERROR", nil)
	w2 := httptest.NewRecorder()
	handlers.Logs(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Logs() returned status %d, expected %d", w2.Code, http.StatusOK)
	}

	var response2 map[string]interface{}
	if err := json.NewDecoder(w2.Body).Decode(&response2); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	logs2, ok := response2["logs"].([]interface{})
	if !ok {
		t.Fatal("Expected 'logs' array in response")
	}

	// Should have filtered to ERROR logs only
	if len(logs2) == 0 {
		t.Error("Expected at least one ERROR log entry")
	}

	// Test Logs endpoint - search query
	req3 := httptest.NewRequest("GET", "/api/logs?search=error", nil)
	w3 := httptest.NewRecorder()
	handlers.Logs(w3, req3)

	if w3.Code != http.StatusOK {
		t.Errorf("Logs() returned status %d, expected %d", w3.Code, http.StatusOK)
	}
}

func TestLogs_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "nonexistent.log")

	os.WriteFile(configPath, []byte("version: \"1.2\"\n"), 0644)

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test Logs endpoint with non-existent log file
	req := httptest.NewRequest("GET", "/api/logs", nil)
	w := httptest.NewRecorder()
	handlers.Logs(w, req)

	// Should return empty logs, not an error
	if w.Code != http.StatusOK {
		t.Errorf("Logs() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	logs, ok := response["logs"].([]interface{})
	if !ok {
		t.Fatal("Expected 'logs' array in response")
	}

	// Should return empty array for non-existent file
	if len(logs) != 0 {
		t.Errorf("Expected empty logs array, got %d entries", len(logs))
	}
}

func TestLogsPage(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	os.WriteFile(configPath, []byte("version: \"1.2\"\n"), 0644)

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("Failed to create handlers: %v", err)
	}

	// Test LogsPage endpoint
	req := httptest.NewRequest("GET", "/logs", nil)
	w := httptest.NewRecorder()
	handlers.LogsPage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("LogsPage() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	if w.Header().Get("Content-Type") != "text/html; charset=utf-8" {
		t.Errorf("LogsPage() returned Content-Type %s, expected text/html; charset=utf-8", w.Header().Get("Content-Type"))
	}

	// Verify HTML content
	body := w.Body.String()
	if len(body) == 0 {
		t.Error("LogsPage() returned empty body")
	}
	if !strings.Contains(body, "musicdl Control Platform") {
		t.Error("LogsPage() should contain 'musicdl Control Platform'")
	}
	if !strings.Contains(body, "Log Viewer") {
		t.Error("LogsPage() should contain 'Log Viewer'")
	}
}
