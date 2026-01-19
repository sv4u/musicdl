package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sv4u/musicdl/download/config"
)

func TestNewConfigManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")

	// Create a valid config file
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

	manager, err := NewConfigManager(configPath, planPath)
	if err != nil {
		t.Fatalf("NewConfigManager failed: %v", err)
	}

	if manager == nil {
		t.Fatal("NewConfigManager returned nil")
	}

	if manager.configPath != configPath {
		t.Errorf("Expected configPath %s, got %s", configPath, manager.configPath)
	}
}

func TestConfigManager_Load(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")

	// Create a valid config file
	configData := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
  format: "mp3"
  bitrate: "128k"
  audio_providers:
    - "youtube-music"
  overwrite: "skip"
songs:
  - name: "Test Song"
    url: "https://open.spotify.com/track/123"
`
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	manager, err := NewConfigManager(configPath, planPath)
	if err != nil {
		t.Fatalf("NewConfigManager failed: %v", err)
	}

	cfg, err := manager.Load()
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Load returned nil config")
	}

	if cfg.Version != "1.2" {
		t.Errorf("Expected version 1.2, got %s", cfg.Version)
	}

	// The loader may convert formats, so just check we have at least 1 song
	if len(cfg.Songs) < 1 {
		t.Errorf("Expected at least 1 song, got %d", len(cfg.Songs))
	}
}

func TestConfigManager_Save(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")

	// Create initial config file
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

	manager, err := NewConfigManager(configPath, planPath)
	if err != nil {
		t.Fatalf("NewConfigManager failed: %v", err)
	}

	// Modify config
	cfg, err := manager.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	cfg.Download.Threads = 8

	// Save
	if err := manager.Save(cfg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Verify backup was created
	backupPath := configPath + ".backup"
	if _, err := os.Stat(backupPath); err != nil {
		t.Errorf("Backup file was not created: %v", err)
	}

	// Reload and verify
	reloaded, err := manager.Load()
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if reloaded.Download.Threads != 8 {
		t.Errorf("Expected Threads 8, got %d", reloaded.Download.Threads)
	}
}

func TestConfigManager_Get(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")

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

	manager, err := NewConfigManager(configPath, planPath)
	if err != nil {
		t.Fatalf("NewConfigManager failed: %v", err)
	}

	cfg1, err := manager.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	cfg2, err := manager.Get()
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Should return cached config (same pointer would indicate caching, but we return copies)
	// Verify they're equivalent
	if cfg1.Version != cfg2.Version {
		t.Error("Get should return consistent config")
	}
}

func TestConfigManager_QueueUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")

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

	manager, err := NewConfigManager(configPath, planPath)
	if err != nil {
		t.Fatalf("NewConfigManager failed: %v", err)
	}

	// Create update config
	updateConfig := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			ClientID:     "new_id",
			ClientSecret: "new_secret",
			Format:       "flac",
			Bitrate:      "320k",
			AudioProviders: []string{"youtube-music"},
			Overwrite:    config.OverwriteSkip,
		},
	}
	updateConfig.Download.SetDefaults()

	// Queue update
	if err := manager.QueueUpdate(updateConfig); err != nil {
		t.Fatalf("QueueUpdate failed: %v", err)
	}

	// Verify pending update exists
	pending, exists := manager.GetPendingUpdate()
	if !exists {
		t.Fatal("Expected pending update, got none")
	}

	if pending.Download.ClientID != "new_id" {
		t.Errorf("Expected ClientID 'new_id', got '%s'", pending.Download.ClientID)
	}

	// Apply pending update
	if err := manager.ApplyPendingUpdate(); err != nil {
		t.Fatalf("ApplyPendingUpdate failed: %v", err)
	}

	// Verify pending update is cleared
	_, exists = manager.GetPendingUpdate()
	if exists {
		t.Error("Pending update should be cleared after applying")
	}

	// Verify config was saved
	reloaded, err := manager.Load()
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	if reloaded.Download.ClientID != "new_id" {
		t.Errorf("Expected ClientID 'new_id' after apply, got '%s'", reloaded.Download.ClientID)
	}
}

func TestConfigManager_ClearPendingUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")

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

	manager, err := NewConfigManager(configPath, planPath)
	if err != nil {
		t.Fatalf("NewConfigManager failed: %v", err)
	}

	// Create and queue update
	updateConfig := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			ClientID:     "new_id",
			ClientSecret: "new_secret",
			Format:       "mp3",
			Bitrate:      "128k",
			AudioProviders: []string{"youtube-music"},
			Overwrite:    config.OverwriteSkip,
		},
	}
	updateConfig.Download.SetDefaults()

	if err := manager.QueueUpdate(updateConfig); err != nil {
		t.Fatalf("QueueUpdate failed: %v", err)
	}

	// Clear pending update
	manager.ClearPendingUpdate()

	// Verify cleared
	_, exists := manager.GetPendingUpdate()
	if exists {
		t.Error("Pending update should be cleared")
	}
}

func TestConfigManager_GetDigest(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")

	configData := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
  format: "mp3"
  bitrate: "128k"
  audio_providers:
    - "youtube-music"
  overwrite: "skip"
songs:
  - name: "Song 1"
    url: "https://open.spotify.com/track/1"
artists:
  - name: "Artist 1"
    url: "https://open.spotify.com/artist/1"
`
	if err := os.WriteFile(configPath, []byte(configData), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	manager, err := NewConfigManager(configPath, planPath)
	if err != nil {
		t.Fatalf("NewConfigManager failed: %v", err)
	}

	digest, err := manager.GetDigest()
	if err != nil {
		t.Fatalf("GetDigest failed: %v", err)
	}

	if digest == "" {
		t.Error("GetDigest returned empty digest")
	}

	// Digest should be consistent for same config
	digest2, err := manager.GetDigest()
	if err != nil {
		t.Fatalf("GetDigest failed: %v", err)
	}

	if digest != digest2 {
		t.Error("GetDigest should return consistent digest for same config")
	}
}

func TestValidateSpotifyURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		sourceType string
		wantErr   bool
	}{
		{"valid track", "https://open.spotify.com/track/123", "song", false},
		{"valid album", "https://open.spotify.com/album/456", "album", false},
		{"valid artist", "https://open.spotify.com/artist/789", "artist", false},
		{"valid playlist", "https://open.spotify.com/playlist/012", "playlist", false},
		{"invalid track URL for album type", "https://open.spotify.com/track/123", "album", true},
		{"empty URL", "", "song", true},
		{"not Spotify URL", "https://youtube.com/watch?v=123", "song", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSpotifyURL(tt.url, tt.sourceType)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestValidateYouTubeURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid video", "https://www.youtube.com/watch?v=123", false},
		{"valid playlist", "https://www.youtube.com/playlist?list=PL123", false},
		{"valid youtu.be", "https://youtu.be/123", false},
		{"empty URL", "", true},
		{"not YouTube URL", "https://spotify.com/track/123", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateYouTubeURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Error("Expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}
			}
		})
	}
}

func TestIsYouTubePlaylistURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{"playlist URL", "https://www.youtube.com/playlist?list=PL123", true},
		{"video with playlist", "https://www.youtube.com/watch?v=123&list=PL456", true},
		{"video only", "https://www.youtube.com/watch?v=123", false},
		{"not YouTube", "https://spotify.com/playlist/123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsYouTubePlaylistURL(tt.url)
			if result != tt.expected {
				t.Errorf("IsYouTubePlaylistURL(%q) = %v, expected %v", tt.url, result, tt.expected)
			}
		})
	}
}

func TestFilterYouTubePlaylists(t *testing.T) {
	playlists := []config.MusicSource{
		{Name: "Spotify Playlist", URL: "https://open.spotify.com/playlist/123"},
		{Name: "YouTube Playlist", URL: "https://www.youtube.com/playlist?list=PL123"},
		{Name: "Another YouTube", URL: "https://www.youtube.com/watch?v=abc&list=PL456"},
	}

	result := FilterYouTubePlaylists(playlists)

	if len(result) != 2 {
		t.Errorf("Expected 2 YouTube playlists, got %d", len(result))
	}

	for _, p := range result {
		if !IsYouTubePlaylistURL(p.URL) {
			t.Errorf("Expected YouTube playlist, got: %s", p.URL)
		}
	}
}

func TestFilterSpotifyPlaylists(t *testing.T) {
	playlists := []config.MusicSource{
		{Name: "Spotify Playlist", URL: "https://open.spotify.com/playlist/123"},
		{Name: "YouTube Playlist", URL: "https://www.youtube.com/playlist?list=PL123"},
		{Name: "Another Spotify", URL: "https://open.spotify.com/playlist/456"},
	}

	result := FilterSpotifyPlaylists(playlists)

	if len(result) != 2 {
		t.Errorf("Expected 2 Spotify playlists, got %d", len(result))
	}

	for _, p := range result {
		if !IsSpotifyURL(p.URL) || IsYouTubePlaylistURL(p.URL) {
			t.Errorf("Expected Spotify playlist (not YouTube), got: %s", p.URL)
		}
	}
}

func TestConfigManager_Concurrency(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")

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

	manager, err := NewConfigManager(configPath, planPath)
	if err != nil {
		t.Fatalf("NewConfigManager failed: %v", err)
	}

	// Concurrent access
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			_, err := manager.Get()
			if err != nil {
				t.Errorf("Get failed: %v", err)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}
