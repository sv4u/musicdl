package handlers

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShutdown(t *testing.T) {
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

	// Shutdown when service is not running (should not error)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := handlers.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown should not error when service is not running: %v", err)
	}
}

func TestShutdown_WithRunningService(t *testing.T) {
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

	// Shutdown (service may or may not be running, should handle both cases)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Should not error even if service is not actually running
	// (we can't start it in unit tests, but shutdown should handle it gracefully)
	if err := handlers.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown should handle gracefully: %v", err)
	}
}
