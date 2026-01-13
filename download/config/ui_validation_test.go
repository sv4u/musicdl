package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUISettings_SetDefaults_NegativeSnapshotInterval(t *testing.T) {
	ui := &UISettings{
		SnapshotInterval: -5, // Negative value
	}
	planPath := "/test/plan/path"

	ui.SetDefaults(planPath)

	// Should be corrected to default value
	if ui.SnapshotInterval != 10 {
		t.Errorf("Expected SnapshotInterval 10 (default for negative), got %d", ui.SnapshotInterval)
	}
}

func TestUISettings_SetDefaults_ZeroSnapshotInterval(t *testing.T) {
	ui := &UISettings{
		SnapshotInterval: 0,
	}
	planPath := "/test/plan/path"

	ui.SetDefaults(planPath)

	// Should be set to default value
	if ui.SnapshotInterval != 10 {
		t.Errorf("Expected SnapshotInterval 10 (default), got %d", ui.SnapshotInterval)
	}
}

func TestUISettings_SetDefaults_NegativeHistoryRetention(t *testing.T) {
	ui := &UISettings{
		HistoryRetention: -10, // Negative value
	}
	planPath := "/test/plan/path"

	ui.SetDefaults(planPath)

	// Should be corrected to 0 (unlimited)
	if ui.HistoryRetention != 0 {
		t.Errorf("Expected HistoryRetention 0 (unlimited for negative), got %d", ui.HistoryRetention)
	}
}

func TestUISettings_SetDefaults_PositiveSnapshotInterval(t *testing.T) {
	ui := &UISettings{
		SnapshotInterval: 15, // Positive value
	}
	planPath := "/test/plan/path"

	ui.SetDefaults(planPath)

	// Should preserve positive value
	if ui.SnapshotInterval != 15 {
		t.Errorf("Expected SnapshotInterval 15 (preserved), got %d", ui.SnapshotInterval)
	}
}

func TestMusicDLConfig_WithNegativeSnapshotInterval(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
ui:
  snapshot_interval: -5
`

	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	// SetDefaults should be called by the caller, but we can test it directly
	config.UI.SetDefaults("/test/plan/path")

	// Should be corrected to default
	if config.UI.SnapshotInterval != 10 {
		t.Errorf("Expected SnapshotInterval 10 (corrected from negative), got %d", config.UI.SnapshotInterval)
	}
}
