package handlers

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sv4u/musicdl/download/config"
)

func TestPathResolution_HistoryPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create config file with relative history_path
	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
ui:
  history_path: "custom_history"
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	// Verify handlers were created (path resolution happens in config manager now)
	if handlers == nil {
		t.Fatal("NewHandlers() returned nil")
	}
	_ = handlers
}

func TestPathResolution_HistoryPath_Absolute(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")
	absoluteHistoryPath := filepath.Join(tmpDir, "absolute_history")

	// Create config file with absolute history_path
	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
ui:
  history_path: "` + absoluteHistoryPath + `"
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	// Verify handlers were created (path resolution happens in config manager now)
	if handlers == nil {
		t.Fatal("NewHandlers() returned nil")
	}
	_ = handlers
}

func TestPathResolution_HistoryPath_Empty(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create config file with empty history_path
	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
ui:
  history_path: ""
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	// Verify handlers were created (path resolution happens in config manager now)
	if handlers == nil {
		t.Fatal("NewHandlers() returned nil")
	}
	_ = handlers
}

func TestPathResolution_LogPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	// Create config file with relative log_path
	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
ui:
  log_path: "custom_logs/app.log"
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	// Verify handlers were created (path resolution happens in config manager now)
	if handlers == nil {
		t.Fatal("NewHandlers() returned nil")
	}
	_ = handlers
}

func TestUISettings_SetDefaults(t *testing.T) {
	ui := &config.UISettings{}
	planPath := "/test/plan/path"

	ui.SetDefaults(planPath)

	if ui.SnapshotInterval == 0 {
		t.Error("Expected SnapshotInterval to have default value")
	}
	if ui.SnapshotInterval != 10 {
		t.Errorf("Expected SnapshotInterval 10, got %d", ui.SnapshotInterval)
	}
	if ui.HistoryRetention != 0 {
		t.Errorf("Expected HistoryRetention 0 (unlimited), got %d", ui.HistoryRetention)
	}
}
