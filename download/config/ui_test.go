package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUISettings_SetDefaults(t *testing.T) {
	ui := &UISettings{}
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

func TestUISettings_SetDefaults_WithValues(t *testing.T) {
	ui := &UISettings{
		SnapshotInterval: 20,
		HistoryRetention: 50,
	}
	planPath := "/test/plan/path"

	ui.SetDefaults(planPath)

	// Should not override existing values
	if ui.SnapshotInterval != 20 {
		t.Errorf("Expected SnapshotInterval 20, got %d", ui.SnapshotInterval)
	}
	if ui.HistoryRetention != 50 {
		t.Errorf("Expected HistoryRetention 50, got %d", ui.HistoryRetention)
	}
}

func TestMusicDLConfig_WithUISettings(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
ui:
  history_path: "custom_history"
  history_retention: 100
  snapshot_interval: 15
  log_path: "custom_logs/app.log"
`

	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if config.UI.HistoryPath != "custom_history" {
		t.Errorf("Expected HistoryPath 'custom_history', got '%s'", config.UI.HistoryPath)
	}
	if config.UI.HistoryRetention != 100 {
		t.Errorf("Expected HistoryRetention 100, got %d", config.UI.HistoryRetention)
	}
	if config.UI.SnapshotInterval != 15 {
		t.Errorf("Expected SnapshotInterval 15, got %d", config.UI.SnapshotInterval)
	}
	if config.UI.LogPath != "custom_logs/app.log" {
		t.Errorf("Expected LogPath 'custom_logs/app.log', got '%s'", config.UI.LogPath)
	}
}

func TestMusicDLConfig_WithEmptyUISettings(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
`

	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// UI settings should be empty but valid
	if config.UI.SnapshotInterval != 0 {
		t.Errorf("Expected SnapshotInterval 0 (not set), got %d", config.UI.SnapshotInterval)
	}
}

func TestUISettings_SetDefaults_ZeroValues(t *testing.T) {
	ui := &UISettings{
		SnapshotInterval: 0,
		HistoryRetention: 0,
	}
	planPath := "/test/plan/path"

	ui.SetDefaults(planPath)

	// Zero values should get defaults
	if ui.SnapshotInterval != 10 {
		t.Errorf("Expected SnapshotInterval 10 (default), got %d", ui.SnapshotInterval)
	}
	if ui.HistoryRetention != 0 {
		t.Errorf("Expected HistoryRetention 0 (unlimited), got %d", ui.HistoryRetention)
	}
}
