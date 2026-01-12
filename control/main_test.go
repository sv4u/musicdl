package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sv4u/musicdl/download"
	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/spotify"
)

func TestPrintUsage(t *testing.T) {
	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printUsage()

	w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Check that usage contains expected text
	expected := []string{
		"musicdl",
		"USAGE",
		"COMMANDS",
		"serve",
		"download",
		"EXAMPLES",
	}

	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("printUsage() output should contain '%s', got: %s", exp, output)
		}
	}
}

func TestValidateDownloadPaths(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
		planPath   string
		logPath    string
		setup      func(t *testing.T, tmpDir string) (string, string, string)
		wantErr    bool
	}{
		{
			name: "valid paths",
			setup: func(t *testing.T, tmpDir string) (string, string, string) {
				configPath := filepath.Join(tmpDir, "config.yaml")
				planPath := filepath.Join(tmpDir, "plans")
				logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

				// Create config file
				if err := os.WriteFile(configPath, []byte("version: \"1.2\"\n"), 0644); err != nil {
					t.Fatalf("Failed to create config file: %v", err)
				}

				return configPath, planPath, logPath
			},
			wantErr: false,
		},
		{
			name: "missing config file",
			setup: func(t *testing.T, tmpDir string) (string, string, string) {
				configPath := filepath.Join(tmpDir, "nonexistent.yaml")
				planPath := filepath.Join(tmpDir, "plans")
				logPath := filepath.Join(tmpDir, "logs", "musicdl.log")
				return configPath, planPath, logPath
			},
			wantErr: true,
		},
		{
			name: "invalid plan path",
			setup: func(t *testing.T, tmpDir string) (string, string, string) {
				configPath := filepath.Join(tmpDir, "config.yaml")
				// Use a path that can't be created (e.g., /dev/null/plans on Unix)
				planPath := "/dev/null/plans"
				logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

				os.WriteFile(configPath, []byte("version: \"1.2\"\n"), 0644)
				return configPath, planPath, logPath
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath, planPath, logPath := tt.setup(t, tmpDir)

			err := validateDownloadPaths(configPath, planPath, logPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateDownloadPaths() error = %v, wantErr %v", err, tt.wantErr)
			}

			// If no error, verify directories were created
			if !tt.wantErr {
				if _, err := os.Stat(planPath); err != nil {
					t.Errorf("planPath should exist: %v", err)
				}
				logDir := filepath.Dir(logPath)
				if _, err := os.Stat(logDir); err != nil {
					t.Errorf("logDir should exist: %v", err)
				}
			}
		})
	}
}

func TestLoadDownloadConfig(t *testing.T) {
	tests := []struct {
		name      string
		config    string
		wantErr   bool
		checkFunc func(t *testing.T, cfg *config.MusicDLConfig)
	}{
		{
			name: "valid config",
			config: `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
  threads: 4
`,
			wantErr: false,
			checkFunc: func(t *testing.T, cfg *config.MusicDLConfig) {
				if cfg.Version != "1.2" {
					t.Errorf("Expected version 1.2, got %s", cfg.Version)
				}
				if cfg.Download.ClientID != "test_id" {
					t.Errorf("Expected client_id 'test_id', got %s", cfg.Download.ClientID)
				}
			},
		},
		{
			name:    "invalid yaml",
			config:  "invalid: yaml: content: [",
			wantErr: true,
		},
		{
			name:    "missing file",
			config:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")

			if tt.name == "missing file" {
				configPath = filepath.Join(tmpDir, "nonexistent.yaml")
			} else {
				if err := os.WriteFile(configPath, []byte(tt.config), 0644); err != nil {
					t.Fatalf("Failed to create config file: %v", err)
				}
			}

			cfg, err := loadDownloadConfig(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("loadDownloadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.checkFunc != nil {
				tt.checkFunc(t, cfg)
			}
		})
	}
}

func TestCreateDownloadService(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plans")

	cfg := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			ClientID:          "test_id",
			ClientSecret:      "test_secret",
			Threads:           4,
			Format:            "mp3",
			Bitrate:           "128k",
			CacheMaxSize:      1000,
			CacheTTL:          3600,
			AudioSearchCacheMaxSize: 500,
			AudioSearchCacheTTL: 86400,
		},
	}
	cfg.Download.SetDefaults()

	service, err := createDownloadService(cfg, planPath)
	if err != nil {
		t.Fatalf("createDownloadService() error = %v", err)
	}
	if service == nil {
		t.Fatal("createDownloadService() returned nil")
	}

	// Verify service is in idle state
	status := service.GetStatus()
	if status["state"] != download.ServiceStateIdle {
		t.Errorf("Expected state 'idle', got '%v'", status["state"])
	}
}

func TestCreateDownloadService_InvalidConfig(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plans")

	// Config with missing credentials
	cfg := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			// Missing ClientID and ClientSecret
		},
	}
	cfg.Download.SetDefaults()

	service, err := createDownloadService(cfg, planPath)
	if err == nil {
		t.Error("createDownloadService() should fail with missing credentials")
	}
	if service != nil {
		t.Error("createDownloadService() should return nil on error")
	}
}

func TestPrintCacheStats(t *testing.T) {
	// Create a minimal service for testing
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plans")

	cfg := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			ClientID:          "test_id",
			ClientSecret:      "test_secret",
			CacheMaxSize:      1000,
			CacheTTL:          3600,
			AudioSearchCacheMaxSize: 500,
			AudioSearchCacheTTL: 86400,
		},
	}
	cfg.Download.SetDefaults()

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
		CacheMaxSize: 1000,
		CacheTTL:     3600,
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		t.Fatalf("Failed to create Spotify client: %v", err)
	}

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
		CacheMaxSize: 500,
		CacheTTL:     86400,
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		t.Fatalf("Failed to create audio provider: %v", err)
	}

	service, err := download.NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), planPath)
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	// Capture log output
	// Note: This is a basic test - printCacheStats writes to log.Printf
	// In a real scenario, we might want to capture log output or refactor to accept a writer
	// For now, we just verify the function doesn't panic
	printCacheStats(service)
}
