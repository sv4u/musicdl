package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Test with valid config
	configYAML := `version: "1.2"
download:
  client_id: "test_client_id"
  client_secret: "test_client_secret"
  threads: 4
  format: "mp3"
  bitrate: "128k"
songs: []
artists: []
playlists: []
albums: []
`

	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if config.Version != "1.2" {
		t.Errorf("Expected version 1.2, got %s", config.Version)
	}

	if config.Download.ClientID != "test_client_id" {
		t.Errorf("Expected client_id 'test_client_id', got '%s'", config.Download.ClientID)
	}

	if config.Download.ClientSecret != "test_client_secret" {
		t.Errorf("Expected client_secret 'test_client_secret', got '%s'", config.Download.ClientSecret)
	}

	// Test defaults
	if config.Download.Threads != 4 {
		t.Errorf("Expected threads 4, got %d", config.Download.Threads)
	}
	if config.Download.Format != "mp3" {
		t.Errorf("Expected format 'mp3', got '%s'", config.Download.Format)
	}
}

func TestLoadConfig_MissingFile(t *testing.T) {
	_, err := LoadConfig("/nonexistent/config.yaml")
	if err == nil {
		t.Error("LoadConfig() should fail with non-existent file")
	}
	if _, ok := err.(*ConfigError); !ok {
		t.Errorf("Expected ConfigError, got %T", err)
	}
}

func TestLoadConfig_MissingCredentials(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configYAML := `version: "1.2"
download:
  threads: 4
songs: []
artists: []
playlists: []
albums: []
`

	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("LoadConfig() should fail with missing credentials")
	}
	if _, ok := err.(*ConfigError); !ok {
		t.Errorf("Expected ConfigError, got %T", err)
	}
}

func TestLoadConfig_InvalidVersion(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configYAML := `version: "1.0"
download:
  client_id: "test_id"
  client_secret: "test_secret"
songs: []
artists: []
playlists: []
albums: []
`

	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("LoadConfig() should fail with invalid version")
	}
}

func TestLoadConfig_Sources(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Test dict format
	configYAML := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
artists:
  "Artist 1": "https://open.spotify.com/artist/1"
  "Artist 2": "https://open.spotify.com/artist/2"
songs: []
playlists: []
albums: []
`

	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if len(config.Artists) != 2 {
		t.Errorf("Expected 2 artists, got %d", len(config.Artists))
	}

	// Check that both artists are present (order is not guaranteed for map format)
	expectedArtists := map[string]string{
		"Artist 1": "https://open.spotify.com/artist/1",
		"Artist 2": "https://open.spotify.com/artist/2",
	}
	
	actualArtists := make(map[string]string)
	for _, artist := range config.Artists {
		actualArtists[artist.Name] = artist.URL
	}
	
	for name, expectedURL := range expectedArtists {
		if actualURL, ok := actualArtists[name]; !ok {
			t.Errorf("Expected artist '%s' not found", name)
		} else if actualURL != expectedURL {
			t.Errorf("Expected artist '%s' to have URL '%s', got '%s'", name, expectedURL, actualURL)
		}
	}
}

func TestLoadConfig_AlbumsWithM3U(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Test extended album format with create_m3u
	configYAML := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
albums:
  - name: "Album 1"
    url: "https://open.spotify.com/album/1"
    create_m3u: true
songs: []
artists: []
playlists: []
`

	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if len(config.Albums) != 1 {
		t.Fatalf("Expected 1 album, got %d", len(config.Albums))
	}

	if !config.Albums[0].CreateM3U {
		t.Error("Expected create_m3u to be true")
	}
}
