package config

import (
	"fmt"
	"strings"
)

// ConfigError represents a configuration error.
type ConfigError struct {
	Message string
}

func (e *ConfigError) Error() string {
	return e.Message
}

// OverwriteMode represents the file overwrite behavior.
type OverwriteMode string

const (
	OverwriteSkip      OverwriteMode = "skip"
	OverwriteOverwrite OverwriteMode = "overwrite"
	OverwriteMetadata  OverwriteMode = "metadata"
)

// DownloadSettings holds download configuration settings.
type DownloadSettings struct {
	// Spotify API credentials (required, from config only)
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`

	// Basic download settings
	Threads        int           `yaml:"threads"`
	MaxRetries     int           `yaml:"max_retries"`
	Format         string        `yaml:"format"`
	Bitrate        string        `yaml:"bitrate"`
	Output         string        `yaml:"output"`
	AudioProviders []string      `yaml:"audio_providers"`
	Overwrite      OverwriteMode `yaml:"overwrite"`

	// Cache settings
	CacheMaxSize              int `yaml:"cache_max_size"`
	CacheTTL                  int `yaml:"cache_ttl"`
	AudioSearchCacheMaxSize   int `yaml:"audio_search_cache_max_size"`
	AudioSearchCacheTTL       int `yaml:"audio_search_cache_ttl"`
	FileExistenceCacheMaxSize int `yaml:"file_existence_cache_max_size"`
	FileExistenceCacheTTL     int `yaml:"file_existence_cache_ttl"`

	// Spotify rate limiting settings
	SpotifyMaxRetries        int     `yaml:"spotify_max_retries"`
	SpotifyRetryBaseDelay    float64 `yaml:"spotify_retry_base_delay"`
	SpotifyRetryMaxDelay     float64 `yaml:"spotify_retry_max_delay"`
	SpotifyRateLimitEnabled  bool    `yaml:"spotify_rate_limit_enabled"`
	SpotifyRateLimitRequests int     `yaml:"spotify_rate_limit_requests"`
	SpotifyRateLimitWindow   float64 `yaml:"spotify_rate_limit_window"`

	// Download rate limiting settings
	DownloadRateLimitEnabled  bool    `yaml:"download_rate_limit_enabled"`
	DownloadRateLimitRequests int     `yaml:"download_rate_limit_requests"`
	DownloadRateLimitWindow   float64 `yaml:"download_rate_limit_window"`
	DownloadBandwidthLimit    *int    `yaml:"download_bandwidth_limit"` // nil = unlimited

	// Plan architecture feature flags
	PlanGenerationEnabled      bool `yaml:"plan_generation_enabled"`
	PlanOptimizationEnabled    bool `yaml:"plan_optimization_enabled"`
	PlanExecutionEnabled       bool `yaml:"plan_execution_enabled"`
	PlanPersistenceEnabled     bool `yaml:"plan_persistence_enabled"`
	PlanStatusReportingEnabled bool `yaml:"plan_status_reporting_enabled"`
}

// SetDefaults sets default values for DownloadSettings.
func (d *DownloadSettings) SetDefaults() {
	if d.Threads == 0 {
		d.Threads = 4
	}
	if d.MaxRetries == 0 {
		d.MaxRetries = 3
	}
	if d.Format == "" {
		d.Format = "mp3"
	}
	if d.Bitrate == "" {
		d.Bitrate = "128k"
	}
	if d.Output == "" {
		d.Output = "{artist}/{album}/{track-number} - {title}.{output-ext}"
	}
	if len(d.AudioProviders) == 0 {
		d.AudioProviders = []string{"youtube-music"}
	}
	if d.Overwrite == "" {
		d.Overwrite = OverwriteSkip
	}
	if d.CacheMaxSize == 0 {
		d.CacheMaxSize = 1000
	}
	if d.CacheTTL == 0 {
		d.CacheTTL = 3600
	}
	if d.AudioSearchCacheMaxSize == 0 {
		d.AudioSearchCacheMaxSize = 500
	}
	if d.AudioSearchCacheTTL == 0 {
		d.AudioSearchCacheTTL = 86400
	}
	if d.FileExistenceCacheMaxSize == 0 {
		d.FileExistenceCacheMaxSize = 10000
	}
	if d.FileExistenceCacheTTL == 0 {
		d.FileExistenceCacheTTL = 3600
	}
	if d.SpotifyMaxRetries == 0 {
		d.SpotifyMaxRetries = 3
	}
	if d.SpotifyRetryBaseDelay == 0 {
		d.SpotifyRetryBaseDelay = 1.0
	}
	if d.SpotifyRetryMaxDelay == 0 {
		d.SpotifyRetryMaxDelay = 120.0
	}
	if !d.SpotifyRateLimitEnabled && d.SpotifyRateLimitRequests == 0 {
		d.SpotifyRateLimitEnabled = true
	}
	if d.SpotifyRateLimitRequests == 0 {
		d.SpotifyRateLimitRequests = 10
	}
	if d.SpotifyRateLimitWindow == 0 {
		d.SpotifyRateLimitWindow = 1.0
	}
	if !d.DownloadRateLimitEnabled && d.DownloadRateLimitRequests == 0 {
		d.DownloadRateLimitEnabled = true
	}
	if d.DownloadRateLimitRequests == 0 {
		d.DownloadRateLimitRequests = 2
	}
	if d.DownloadRateLimitWindow == 0 {
		d.DownloadRateLimitWindow = 1.0
	}
	if d.DownloadBandwidthLimit == nil {
		limit := 1048576 // 1MB/sec
		d.DownloadBandwidthLimit = &limit
	}
	if !d.PlanGenerationEnabled && !d.PlanOptimizationEnabled && !d.PlanExecutionEnabled {
		d.PlanGenerationEnabled = true
		d.PlanOptimizationEnabled = true
		d.PlanExecutionEnabled = true
	}
	if !d.PlanPersistenceEnabled && !d.PlanStatusReportingEnabled {
		d.PlanPersistenceEnabled = true
		d.PlanStatusReportingEnabled = true
	}
}

// Validate validates DownloadSettings.
func (d *DownloadSettings) Validate() error {
	// Validate credentials (config-only, no environment variables)
	d.ClientID = strings.TrimSpace(d.ClientID)
	d.ClientSecret = strings.TrimSpace(d.ClientSecret)

	missing := []string{}
	if d.ClientID == "" {
		missing = append(missing, "client_id")
	}
	if d.ClientSecret == "" {
		missing = append(missing, "client_secret")
	}

	if len(missing) > 0 {
		missingStr := strings.Join(missing, " and ")
		return &ConfigError{
			Message: fmt.Sprintf(
				"Missing Spotify %s. Both client_id and client_secret must be provided in the configuration file: download.client_id and download.client_secret",
				missingStr,
			),
		}
	}

	// Validate threads (spec: 1-16)
	if d.Threads < 1 || d.Threads > 16 {
		return &ConfigError{
			Message: fmt.Sprintf("Invalid threads: %d. Must be between 1 and 16", d.Threads),
		}
	}

	// Validate output template contains {title} (spec requirement)
	if !strings.Contains(d.Output, "{title}") {
		return &ConfigError{
			Message: "download.output must contain the {title} placeholder",
		}
	}

	// Validate overwrite mode
	if d.Overwrite != OverwriteSkip && d.Overwrite != OverwriteOverwrite && d.Overwrite != OverwriteMetadata {
		return &ConfigError{
			Message: fmt.Sprintf("Invalid overwrite mode: %s. Must be one of: skip, overwrite, metadata", d.Overwrite),
		}
	}

	// Validate format
	validFormats := map[string]bool{
		"mp3":  true,
		"flac": true,
		"m4a":  true,
		"opus": true,
	}
	if !validFormats[d.Format] {
		return &ConfigError{
			Message: fmt.Sprintf("Invalid format: %s. Must be one of: mp3, flac, m4a, opus", d.Format),
		}
	}

	// Validate audio providers
	validProviders := map[string]bool{
		"youtube-music": true,
		"youtube":       true,
		"soundcloud":    true,
	}
	for _, provider := range d.AudioProviders {
		if !validProviders[provider] {
			return &ConfigError{
				Message: fmt.Sprintf("Invalid audio provider: %s. Must be one of: youtube-music, youtube, soundcloud", provider),
			}
		}
	}

	return nil
}

// MusicSource represents a music source entry.
type MusicSource struct {
	Name      string `yaml:"name"`
	URL       string `yaml:"url"`
	CreateM3U bool   `yaml:"create_m3u"`
}

// UISettings holds UI and history tracking configuration settings.
type UISettings struct {
	// History tracking settings
	HistoryPath      string `yaml:"history_path"`      // Path to history directory (default: plan_path/history)
	HistoryRetention int    `yaml:"history_retention"` // Number of runs to keep (0 = unlimited, default: 0)
	SnapshotInterval int    `yaml:"snapshot_interval"` // Progress snapshot interval in seconds (default: 10)

	// Log settings
	LogPath string `yaml:"log_path"` // Path to log file (configurable)
}

// SetDefaults sets default values for UISettings.
func (u *UISettings) SetDefaults(planPath string) {
	// Validate and set SnapshotInterval: must be positive, default to 10 if zero or negative
	if u.SnapshotInterval <= 0 {
		u.SnapshotInterval = 10 // Default: snapshot every 10 seconds
	}
	// HistoryRetention: 0 means unlimited, negative values are invalid but we allow 0
	if u.HistoryRetention < 0 {
		u.HistoryRetention = 0 // Treat negative as unlimited
	}
	// HistoryPath and LogPath are set by the caller based on planPath
}

// MusicDLConfig represents the main configuration model.
type MusicDLConfig struct {
	Version   string           `yaml:"version"`
	Download  DownloadSettings `yaml:"download"`
	UI        UISettings       `yaml:"ui"`
	Songs     []MusicSource    `yaml:"songs"`
	Artists   []MusicSource    `yaml:"artists"`
	Playlists []MusicSource    `yaml:"playlists"`
	Albums    []MusicSource    `yaml:"albums"`
}

// Validate validates MusicDLConfig.
func (c *MusicDLConfig) Validate() error {
	// Validate version
	if c.Version != "1.2" {
		return &ConfigError{
			Message: fmt.Sprintf("Invalid version: %s. Expected 1.2", c.Version),
		}
	}

	// Set defaults and validate download settings
	c.Download.SetDefaults()
	if err := c.Download.Validate(); err != nil {
		return err
	}

	// UI settings defaults are set by the caller (needs planPath)

	return nil
}
