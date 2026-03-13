package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoadConfig_PlaylistsWithM3U(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Test extended playlist format with create_m3u
	configYAML := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
playlists:
  - name: "Playlist 1"
    url: "https://open.spotify.com/playlist/1"
    create_m3u: true
  - name: "Playlist 2"
    url: "https://open.spotify.com/playlist/2"
songs: []
artists: []
albums: []
`

	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if len(config.Playlists) != 2 {
		t.Fatalf("Expected 2 playlists, got %d", len(config.Playlists))
	}

	if !config.Playlists[0].CreateM3U {
		t.Error("Expected first playlist create_m3u to be true")
	}

	if config.Playlists[1].CreateM3U {
		t.Error("Expected second playlist create_m3u to be false")
	}

	if config.Playlists[0].Name != "Playlist 1" {
		t.Errorf("Expected first playlist name 'Playlist 1', got '%s'", config.Playlists[0].Name)
	}

	if config.Playlists[1].Name != "Playlist 2" {
		t.Errorf("Expected second playlist name 'Playlist 2', got '%s'", config.Playlists[1].Name)
	}
}

func TestLoadConfig_SpecLayoutSpotifyAndThreads(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Spec layout: top-level spotify and threads
	configYAML := `version: "1.2"
threads: 8
spotify:
  client_id: "spec_client_id"
  client_secret: "spec_client_secret"
download:
  format: "mp3"
  output: "{artist}/{album}/{title}.{output-ext}"
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

	if config.Download.ClientID != "spec_client_id" {
		t.Errorf("Expected client_id from spotify, got %q", config.Download.ClientID)
	}
	if config.Download.ClientSecret != "spec_client_secret" {
		t.Errorf("Expected client_secret from spotify, got %q", config.Download.ClientSecret)
	}
	if config.Download.Threads != 8 {
		t.Errorf("Expected threads 8 from top-level, got %d", config.Download.Threads)
	}
}

func TestLoadConfig_SpecLayoutLegacyCredentialsTakePrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Legacy download.client_id/client_secret present; spotify also present
	configYAML := `version: "1.2"
spotify:
  client_id: "spotify_id"
  client_secret: "spotify_secret"
download:
  client_id: "legacy_id"
  client_secret: "legacy_secret"
  output: "{title}.{output-ext}"
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

	if config.Download.ClientID != "legacy_id" || config.Download.ClientSecret != "legacy_secret" {
		t.Errorf("Expected legacy credentials to take precedence, got client_id=%q client_secret=%q",
			config.Download.ClientID, config.Download.ClientSecret)
	}
}

func TestLoadConfig_SpecLayoutRateLimits(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configYAML := `version: "1.2"
spotify:
  client_id: "id"
  client_secret: "sec"
rate_limits:
  spotify_retries: 5
  youtube_retries: 4
  youtube_bandwidth: 2097152
download:
  output: "{artist}/{title}.{output-ext}"
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

	if config.Download.SpotifyMaxRetries != 5 {
		t.Errorf("Expected spotify_max_retries 5 from rate_limits.spotify_retries, got %d", config.Download.SpotifyMaxRetries)
	}
	if config.Download.MaxRetries != 4 {
		t.Errorf("Expected max_retries 4 from rate_limits.youtube_retries, got %d", config.Download.MaxRetries)
	}
	if config.Download.DownloadBandwidthLimit == nil || *config.Download.DownloadBandwidthLimit != 2097152 {
		t.Errorf("Expected download_bandwidth_limit 2097152 from rate_limits.youtube_bandwidth, got %v", config.Download.DownloadBandwidthLimit)
	}
}

func TestLoadConfig_InvalidThreads(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configYAML := `version: "1.2"
download:
  client_id: "id"
  client_secret: "sec"
  threads: 20
  output: "{artist}/{album}/{track-number} - {title}.{output-ext}"
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
		t.Error("LoadConfig() should fail when threads is outside 1-16")
	}
}

func TestLoadConfig_OutputMissingTitlePlaceholder(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configYAML := `version: "1.2"
download:
  client_id: "id"
  client_secret: "sec"
  output: "{artist}/{album}.{output-ext}"
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
		t.Error("LoadConfig() should fail when output does not contain {title}")
	}
}

func TestLoadConfig_EmptySourceURLRejected(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configYAML := `version: "1.2"
download:
  client_id: "id"
  client_secret: "sec"
  output: "{artist}/{title}.{output-ext}"
songs: []
artists:
  - name: Some Artist
    url: ""
playlists: []
albums: []
`

	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Error("LoadConfig() should fail when any source has empty URL")
	}
	if _, ok := err.(*ConfigError); !ok {
		t.Errorf("Expected ConfigError, got %T", err)
	}
}

// TestLoadConfig_UserConfigStructure verifies that a config matching the user's
// structure (spec layout: spotify, download with threads/format/output/overwrite,
// rate_limits, artists, playlists including YouTube URL) loads without error.
func TestLoadConfig_UserConfigStructure(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configYAML := `version: "1.2"

spotify:
  client_id: test_client_id
  client_secret: test_client_secret

download:
  threads: 1
  max_retries: 2
  format: mp3
  bitrate: 128k
  output: "{artist}/{album}/{disc-number}{track-number} - {title}.{output-ext}"
  audio_providers:
    - youtube-music
    - youtube
  overwrite: metadata

rate_limits:
  spotify_retries: 2
  youtube_retries: 2
  youtube_bandwidth: 1048576

songs: []

artists:
  - name: ArtistOne
    url: https://open.spotify.com/artist/4kqFrZkeqDfOIEqTWqbOOV
  - name: "8485"
    url: https://open.spotify.com/artist/3LwiPwIJNshV4ItekGcIMo

playlists:
  - name: "dog relaxation"
    url: https://open.spotify.com/playlist/3300BQPneawOkHUGOOUhMK
    create_m3u: true
  - name: "upbeat study music"
    url: https://www.youtube.com/playlist?list=PLveg0IEcZWN7eQvidQOrkxiBvH0Skewwv
    create_m3u: true

albums: []
`

	if err := os.WriteFile(configPath, []byte(configYAML), 0644); err != nil {
		t.Fatalf("Failed to write config file: %v", err)
	}

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() failed: %v", err)
	}

	if cfg.Download.ClientID != "test_client_id" || cfg.Download.ClientSecret != "test_client_secret" {
		t.Errorf("Expected spotify credentials in download, got client_id=%q", cfg.Download.ClientID)
	}
	if cfg.Download.Threads != 1 {
		t.Errorf("Expected threads 1, got %d", cfg.Download.Threads)
	}
	if cfg.Download.MaxRetries != 2 {
		t.Errorf("Expected max_retries 2, got %d", cfg.Download.MaxRetries)
	}
	if cfg.Download.SpotifyMaxRetries != 2 {
		t.Errorf("Expected spotify_max_retries 2, got %d", cfg.Download.SpotifyMaxRetries)
	}
	if cfg.Download.DownloadBandwidthLimit == nil || *cfg.Download.DownloadBandwidthLimit != 1048576 {
		t.Errorf("Expected download_bandwidth_limit 1048576, got %v", cfg.Download.DownloadBandwidthLimit)
	}
	if cfg.Download.Overwrite != OverwriteMetadata {
		t.Errorf("Expected overwrite metadata, got %q", cfg.Download.Overwrite)
	}
	if len(cfg.Artists) != 2 {
		t.Errorf("Expected 2 artists, got %d", len(cfg.Artists))
	}
	if len(cfg.Playlists) != 2 {
		t.Errorf("Expected 2 playlists, got %d", len(cfg.Playlists))
	}
	// First playlist Spotify, second YouTube
	if !strings.Contains(cfg.Playlists[0].URL, "spotify.com") {
		t.Errorf("Expected first playlist to be Spotify, got %q", cfg.Playlists[0].URL)
	}
	if !strings.Contains(cfg.Playlists[1].URL, "youtube.com") {
		t.Errorf("Expected second playlist to be YouTube, got %q", cfg.Playlists[1].URL)
	}
}

// --- Bug 7: SetDefaults should not re-enable rate limiting when user configures requests/window ---

func TestSetDefaults_RateLimitAutoEnable(t *testing.T) {
	d := &DownloadSettings{}
	d.SetDefaults()

	if !d.SpotifyRateLimitEnabled {
		t.Error("SpotifyRateLimitEnabled should be true when nothing is configured")
	}
	if d.SpotifyRateLimitRequests != 10 {
		t.Errorf("SpotifyRateLimitRequests = %d, want 10", d.SpotifyRateLimitRequests)
	}
	if !d.DownloadRateLimitEnabled {
		t.Error("DownloadRateLimitEnabled should be true when nothing is configured")
	}
	if d.DownloadRateLimitRequests != 2 {
		t.Errorf("DownloadRateLimitRequests = %d, want 2", d.DownloadRateLimitRequests)
	}
}

func TestSetDefaults_RateLimitExplicitRequests(t *testing.T) {
	d := &DownloadSettings{
		SpotifyRateLimitRequests: 20,
	}
	d.SetDefaults()

	if d.SpotifyRateLimitRequests != 20 {
		t.Errorf("SpotifyRateLimitRequests = %d, want 20 (user-configured)", d.SpotifyRateLimitRequests)
	}
}

func TestSetDefaults_RateLimitExplicitWindow(t *testing.T) {
	d := &DownloadSettings{
		DownloadRateLimitWindow: 5.0,
	}
	d.SetDefaults()

	if d.DownloadRateLimitWindow != 5.0 {
		t.Errorf("DownloadRateLimitWindow = %f, want 5.0 (user-configured)", d.DownloadRateLimitWindow)
	}
	if d.DownloadRateLimitRequests != 2 {
		t.Errorf("DownloadRateLimitRequests = %d, want 2 (default)", d.DownloadRateLimitRequests)
	}
}
