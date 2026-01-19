package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/plan"
)

// ConfigManager manages configuration loading, saving, and updates.
type ConfigManager struct {
	configPath string
	planPath   string

	// Current config (cached)
	currentConfig *config.MusicDLConfig
	configMu      sync.RWMutex

	// Pending update queue
	pendingUpdate *config.MusicDLConfig
	pendingMu     sync.Mutex
}

// NewConfigManager creates a new configuration manager.
func NewConfigManager(configPath, planPath string) (*ConfigManager, error) {
	// Validate config file exists
	if _, err := os.Stat(configPath); err != nil {
		return nil, fmt.Errorf("config file not found: %w", err)
	}

	// Ensure plan directory exists
	if err := os.MkdirAll(planPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plan directory: %w", err)
	}

	manager := &ConfigManager{
		configPath: configPath,
		planPath:   planPath,
	}

	// Load initial config
	if _, err := manager.Load(); err != nil {
		return nil, fmt.Errorf("failed to load initial config: %w", err)
	}

	return manager, nil
}

// Load loads configuration from file.
func (m *ConfigManager) Load() (*config.MusicDLConfig, error) {
	cfg, err := config.LoadConfig(m.configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Set UI defaults
	cfg.UI.SetDefaults(m.planPath)

	m.configMu.Lock()
	m.currentConfig = cfg
	m.configMu.Unlock()

	return cfg, nil
}

// Save saves configuration to file.
func (m *ConfigManager) Save(cfg *config.MusicDLConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate before saving
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	// Set UI defaults
	cfg.UI.SetDefaults(m.planPath)

	// Marshal to YAML
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Create backup
	backupPath := m.configPath + ".backup"
	if _, err := os.Stat(m.configPath); err == nil {
		originalData, err := os.ReadFile(m.configPath)
		if err == nil {
			if err := os.WriteFile(backupPath, originalData, 0644); err != nil {
				// Log warning but continue - backup is optional
				// We can't use logger here as it might create circular dependency
			}
		}
	}

	// Write config file
	if err := os.WriteFile(m.configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	// Update cached config
	m.configMu.Lock()
	m.currentConfig = cfg
	m.configMu.Unlock()

	return nil
}

// Get returns the current configuration (cached or loaded).
func (m *ConfigManager) Get() (*config.MusicDLConfig, error) {
	m.configMu.RLock()
	cfg := m.currentConfig
	m.configMu.RUnlock()

	if cfg != nil {
		// Return a copy to avoid external modifications
		return m.copyConfig(cfg), nil
	}

	// Load if not cached
	return m.Load()
}

// Validate validates a configuration.
func (m *ConfigManager) Validate(cfg *config.MusicDLConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Set UI defaults for validation
	cfg.UI.SetDefaults(m.planPath)

	return cfg.Validate()
}

// GetDigest returns a configuration digest/summary for the dashboard.
func (m *ConfigManager) GetDigest() (string, error) {
	cfg, err := m.Get()
	if err != nil {
		return "", err
	}

	// Create a simple digest from key config values
	digest := fmt.Sprintf("version=%s, threads=%d, format=%s, songs=%d, artists=%d, playlists=%d, albums=%d",
		cfg.Version,
		cfg.Download.Threads,
		cfg.Download.Format,
		len(cfg.Songs),
		len(cfg.Artists),
		len(cfg.Playlists),
		len(cfg.Albums),
	)

	// Create hash for uniqueness
	hash := sha256.Sum256([]byte(digest))
	return hex.EncodeToString(hash[:8]), nil
}

// QueueUpdate queues a configuration update to apply after current download completes.
func (m *ConfigManager) QueueUpdate(cfg *config.MusicDLConfig) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate before queueing
	if err := m.Validate(cfg); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	m.pendingMu.Lock()
	m.pendingUpdate = cfg
	m.pendingMu.Unlock()

	return nil
}

// GetPendingUpdate returns the pending configuration update if any.
func (m *ConfigManager) GetPendingUpdate() (*config.MusicDLConfig, bool) {
	m.pendingMu.Lock()
	defer m.pendingMu.Unlock()

	if m.pendingUpdate == nil {
		return nil, false
	}

	// Return a copy
	return m.copyConfig(m.pendingUpdate), true
}

// ClearPendingUpdate clears the pending update.
func (m *ConfigManager) ClearPendingUpdate() {
	m.pendingMu.Lock()
	m.pendingUpdate = nil
	m.pendingMu.Unlock()
}

// ApplyPendingUpdate applies the pending update and saves it to file.
func (m *ConfigManager) ApplyPendingUpdate() error {
	m.pendingMu.Lock()
	pending := m.pendingUpdate
	m.pendingMu.Unlock()

	if pending == nil {
		return nil // No pending update
	}

	// Save to file
	if err := m.Save(pending); err != nil {
		return fmt.Errorf("failed to apply pending update: %w", err)
	}

	// Clear pending update
	m.ClearPendingUpdate()

	return nil
}

// copyConfig creates a deep copy of the configuration.
func (m *ConfigManager) copyConfig(cfg *config.MusicDLConfig) *config.MusicDLConfig {
	// Marshal and unmarshal to create a deep copy
	data, err := yaml.Marshal(cfg)
	if err != nil {
		// Fallback: return original if copy fails
		return cfg
	}

	var copy config.MusicDLConfig
	if err := yaml.Unmarshal(data, &copy); err != nil {
		// Fallback: return original if copy fails
		return cfg
	}

	return &copy
}

// ValidateSpotifyURL validates a Spotify URL for the given source type.
func ValidateSpotifyURL(url string, sourceType string) error {
	if url == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Basic Spotify URL validation
	if !IsSpotifyURL(url) {
		return fmt.Errorf("invalid Spotify URL format: %s", url)
	}

	// Validate URL format based on source type
	switch sourceType {
	case "song", "track":
		if !contains(url, "/track/") {
			return fmt.Errorf("invalid Spotify track URL: %s", url)
		}
	case "album":
		if !contains(url, "/album/") {
			return fmt.Errorf("invalid Spotify album URL: %s", url)
		}
	case "artist":
		if !contains(url, "/artist/") {
			return fmt.Errorf("invalid Spotify artist URL: %s", url)
		}
	case "playlist":
		if !contains(url, "/playlist/") {
			return fmt.Errorf("invalid Spotify playlist URL: %s", url)
		}
	default:
		// Generic validation - just check it's a Spotify URL
	}

	return nil
}

// ValidateYouTubeURL validates a YouTube URL (videos and playlists).
func ValidateYouTubeURL(url string) error {
	if url == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Basic YouTube URL validation
	if !contains(url, "youtube.com") && !contains(url, "youtu.be") {
		return fmt.Errorf("invalid YouTube URL format: %s", url)
	}

	return nil
}

// IsYouTubePlaylistURL checks if a URL is a YouTube playlist.
func IsYouTubePlaylistURL(url string) bool {
	return plan.IsYouTubePlaylist(url)
}

// IsSpotifyURL checks if a URL is a Spotify URL.
func IsSpotifyURL(url string) bool {
	return contains(url, "open.spotify.com") || contains(url, "spotify.com")
}

// FilterYouTubePlaylists filters YouTube playlists from a playlist array.
func FilterYouTubePlaylists(playlists []config.MusicSource) []config.MusicSource {
	result := make([]config.MusicSource, 0)
	for _, p := range playlists {
		if IsYouTubePlaylistURL(p.URL) {
			result = append(result, p)
		}
	}
	return result
}

// FilterSpotifyPlaylists filters Spotify playlists from a playlist array.
func FilterSpotifyPlaylists(playlists []config.MusicSource) []config.MusicSource {
	result := make([]config.MusicSource, 0)
	for _, p := range playlists {
		if IsSpotifyURL(p.URL) && !IsYouTubePlaylistURL(p.URL) {
			result = append(result, p)
		}
	}
	return result
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	// Simple case-insensitive contains
	sLower := toLower(s)
	substrLower := toLower(substr)
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// toLower converts a string to lowercase.
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}
