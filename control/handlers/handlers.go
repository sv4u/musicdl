package handlers

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sv4u/musicdl/download"
	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/spotify"
)

// Handlers holds all HTTP handlers for the control platform.
type Handlers struct {
	configPath string
	planPath   string
	logPath    string
	startTime  time.Time

	// Download service (lazy initialization)
	service     *download.Service
	serviceMu   sync.RWMutex
	serviceInit sync.Once
}

// NewHandlers creates a new handlers instance.
func NewHandlers(configPath, planPath, logPath string, startTime time.Time) (*Handlers, error) {
	// Validate paths exist or can be created
	if err := validatePaths(configPath, planPath, logPath); err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	return &Handlers{
		configPath: configPath,
		planPath:   planPath,
		logPath:    logPath,
		startTime:  startTime,
	}, nil
}

// getService returns the download service, initializing it if necessary.
func (h *Handlers) getService() (*download.Service, error) {
	var initErr error
	h.serviceInit.Do(func() {
		// Load config
		cfg, err := config.LoadConfig(h.configPath)
		if err != nil {
			initErr = fmt.Errorf("failed to load config: %w", err)
			return
		}

		// Create Spotify client
		spotifyConfig := &spotify.Config{
			ClientID:          cfg.Download.ClientID,
			ClientSecret:      cfg.Download.ClientSecret,
			CacheMaxSize:      cfg.Download.CacheMaxSize,
			CacheTTL:          cfg.Download.CacheTTL,
			RateLimitEnabled:  cfg.Download.SpotifyRateLimitEnabled,
			RateLimitRequests: cfg.Download.SpotifyRateLimitRequests,
			RateLimitWindow:   cfg.Download.SpotifyRateLimitWindow,
			MaxRetries:        cfg.Download.SpotifyMaxRetries,
			RetryBaseDelay:    cfg.Download.SpotifyRetryBaseDelay,
			RetryMaxDelay:     cfg.Download.SpotifyRetryMaxDelay,
		}
		spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
		if err != nil {
			initErr = fmt.Errorf("failed to create Spotify client: %w", err)
			return
		}

		// Create audio provider
		audioConfig := &audio.Config{
			OutputFormat:   cfg.Download.Format,
			Bitrate:        cfg.Download.Bitrate,
			AudioProviders: cfg.Download.AudioProviders,
			CacheMaxSize:   cfg.Download.AudioSearchCacheMaxSize,
			CacheTTL:       cfg.Download.AudioSearchCacheTTL,
		}
		audioProvider, err := audio.NewProvider(audioConfig)
		if err != nil {
			initErr = fmt.Errorf("failed to create audio provider: %w", err)
			return
		}

		// Create metadata embedder
		metadataEmbedder := metadata.NewEmbedder()

		// Create download service
		service, err := download.NewService(cfg, spotifyClient, audioProvider, metadataEmbedder, h.planPath)
		if err != nil {
			initErr = fmt.Errorf("failed to create download service: %w", err)
			return
		}

		h.serviceMu.Lock()
		h.service = service
		h.serviceMu.Unlock()
	})

	if initErr != nil {
		return nil, initErr
	}

	h.serviceMu.RLock()
	defer h.serviceMu.RUnlock()
	return h.service, nil
}

// validatePaths ensures required directories exist.
func validatePaths(configPath, planPath, logPath string) error {
	// Check config file exists
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("config file not found: %s", configPath)
	}

	// Ensure plan directory exists
	if err := os.MkdirAll(planPath, 0755); err != nil {
		return fmt.Errorf("failed to create plan directory: %w", err)
	}

	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	return nil
}

// logError logs an error with context.
func (h *Handlers) logError(operation string, err error) {
	log.Printf("Error in %s: %v", operation, err)
}
