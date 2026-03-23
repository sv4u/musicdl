package audio

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sv4u/musicdl/download/audius"
	"github.com/sv4u/musicdl/download/spotify"
)

// Config holds configuration for the audio provider.
type Config struct {
	// Output settings
	OutputFormat string
	Bitrate      string

	// Provider settings
	AudioProviders []string

	// yt-dlp settings
	CookiesFromBrowser string // e.g. "chrome", "firefox" — passed as --cookies-from-browser
	Cookies            string // path to Netscape-format cookies.txt — passed as --cookies
	JSRuntimes         string // JS runtime for yt-dlp — passed as --js-runtimes (e.g. "node")

	// Cache settings
	CacheMaxSize int
	CacheTTL     int

	// Rate limiting settings (per provider)
	YouTubeRateLimitEnabled  bool
	YouTubeRateLimitRequests int
	YouTubeRateLimitWindow   float64

	YouTubeMusicRateLimitEnabled  bool
	YouTubeMusicRateLimitRequests int
	YouTubeMusicRateLimitWindow   float64

	SoundCloudRateLimitEnabled  bool
	SoundCloudRateLimitRequests int
	SoundCloudRateLimitWindow   float64

	BandcampRateLimitEnabled  bool
	BandcampRateLimitRequests int
	BandcampRateLimitWindow   float64

	AudiusRateLimitEnabled  bool
	AudiusRateLimitRequests int
	AudiusRateLimitWindow   float64

	// Optional general rate limiter for network impact management
	GeneralRateLimiter interface {
		WaitForRequest(ctx context.Context) error
	}
}

// Provider represents an audio provider that uses yt-dlp.
type Provider struct {
	config             *Config
	searchCache        *spotify.TTLCache
	rateLimiters       map[string]*spotify.RateLimiter
	generalRateLimiter interface {
		WaitForRequest(ctx context.Context) error
	}
	audiusClient *audius.Client
	tempDir      string
}

// NewProvider creates a new audio provider.
func NewProvider(config *Config) (*Provider, error) {
	// Create search cache
	searchCache := spotify.NewTTLCache(config.CacheMaxSize, config.CacheTTL)

	// Create per-provider rate limiters
	rateLimiters := make(map[string]*spotify.RateLimiter)

	if config.YouTubeRateLimitEnabled {
		rateLimiters["youtube"] = spotify.NewRateLimiter(
			config.YouTubeRateLimitEnabled,
			config.YouTubeRateLimitRequests,
			config.YouTubeRateLimitWindow,
		)
	}

	if config.YouTubeMusicRateLimitEnabled {
		rateLimiters["youtube-music"] = spotify.NewRateLimiter(
			config.YouTubeMusicRateLimitEnabled,
			config.YouTubeMusicRateLimitRequests,
			config.YouTubeMusicRateLimitWindow,
		)
	}

	if config.SoundCloudRateLimitEnabled {
		rateLimiters["soundcloud"] = spotify.NewRateLimiter(
			config.SoundCloudRateLimitEnabled,
			config.SoundCloudRateLimitRequests,
			config.SoundCloudRateLimitWindow,
		)
	}

	if config.BandcampRateLimitEnabled {
		rateLimiters["bandcamp"] = spotify.NewRateLimiter(
			config.BandcampRateLimitEnabled,
			config.BandcampRateLimitRequests,
			config.BandcampRateLimitWindow,
		)
	}

	if config.AudiusRateLimitEnabled {
		rateLimiters["audius"] = spotify.NewRateLimiter(
			config.AudiusRateLimitEnabled,
			config.AudiusRateLimitRequests,
			config.AudiusRateLimitWindow,
		)
	}

	// Create temp directory
	tempDir := filepath.Join(os.TempDir(), "musicdl")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create Audius client if audius is in the provider list
	var audiusClient *audius.Client
	for _, p := range config.AudioProviders {
		if p == "audius" {
			audiusClient = audius.NewClient()
			break
		}
	}

	return &Provider{
		config:             config,
		searchCache:        searchCache,
		rateLimiters:       rateLimiters,
		generalRateLimiter: config.GeneralRateLimiter,
		audiusClient:       audiusClient,
		tempDir:            tempDir,
	}, nil
}

// Search searches for audio URL matching query (cached).
func (p *Provider) Search(ctx context.Context, query string) (string, error) {
	// Normalize query for cache key
	cacheKey := p.normalizeQuery(query)

	// Check cache first
	if cached := p.searchCache.Get(cacheKey); cached != nil {
		if url, ok := cached.(string); ok {
			return url, nil
		}
		// Cached as "not found" (empty string)
		if urlStr, ok := cached.(string); ok && urlStr == "" {
			return "", &SearchError{Message: "No audio found (cached)"}
		}
	}

	// Apply general rate limiting if enabled
	if p.generalRateLimiter != nil {
		if err := p.generalRateLimiter.WaitForRequest(ctx); err != nil {
			return "", fmt.Errorf("general rate limit: %w", err)
		}
	}

	// Try each provider in order
	var audioURL string
	var lastErr error

	for _, provider := range p.config.AudioProviders {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return "", err
		}

		// Apply provider-specific rate limiting
		if limiter, ok := p.rateLimiters[provider]; ok {
			if err := limiter.WaitIfNeeded(ctx); err != nil {
				return "", fmt.Errorf("rate limit for %s: %w", provider, err)
			}
		}

		// Search using provider
		url, err := p.searchProvider(ctx, provider, query)
		if err == nil && url != "" {
			audioURL = url
			break
		}
		lastErr = err
	}

	// Cache result (even if empty, to avoid repeated failed searches)
	if audioURL == "" {
		p.searchCache.Set(cacheKey, "")
		if lastErr != nil {
			return "", fmt.Errorf("search failed: %w", lastErr)
		}
		return "", &SearchError{Message: "No audio found"}
	}

	p.searchCache.Set(cacheKey, audioURL)
	return audioURL, nil
}

// normalizeQuery normalizes a query string for cache key.
func (p *Provider) normalizeQuery(query string) string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	return fmt.Sprintf("audio_search:%s", normalized)
}

// searchProvider searches using a specific provider.
func (p *Provider) searchProvider(ctx context.Context, provider, query string) (string, error) {
	// Audius uses its own REST API for search, not yt-dlp search prefixes
	if provider == "audius" {
		return p.searchAudius(ctx, query)
	}

	// All other providers use yt-dlp search prefixes
	var searchQuery string
	switch provider {
	case "youtube-music":
		searchQuery = fmt.Sprintf("ytsearch1:%s", query)
	case "youtube":
		searchQuery = fmt.Sprintf("ytsearch:%s", query)
	case "soundcloud":
		searchQuery = fmt.Sprintf("scsearch:%s", query)
	default:
		searchQuery = fmt.Sprintf("ytsearch:%s", query)
	}

	return p.runYtDlpSearch(ctx, searchQuery)
}

// searchAudius searches for a track on Audius via the REST API.
// Returns an Audius track URL that yt-dlp can then download.
func (p *Provider) searchAudius(ctx context.Context, query string) (string, error) {
	if p.audiusClient == nil {
		return "", &SearchError{Message: "Audius client not initialized"}
	}
	trackURL, err := p.audiusClient.SearchBestMatch(ctx, query)
	if err != nil {
		return "", &SearchError{
			Message:  fmt.Sprintf("Audius search failed: %v", err),
			Original: err,
		}
	}
	if trackURL == "" {
		return "", &SearchError{Message: "No results from Audius search"}
	}
	return trackURL, nil
}

// Download downloads audio to output path.
// Returns (downloadedFilePath, rawYtDlpOutput, error).
func (p *Provider) Download(ctx context.Context, url, outputPath string) (string, string, error) {
	// Apply general rate limiting if enabled
	if p.generalRateLimiter != nil {
		if err := p.generalRateLimiter.WaitForRequest(ctx); err != nil {
			return "", "", fmt.Errorf("general rate limit: %w", err)
		}
	}

	// Determine provider from URL for provider-specific rate limiting
	provider := p.detectProvider(url)
	if limiter, ok := p.rateLimiters[provider]; ok {
		if err := limiter.WaitIfNeeded(ctx); err != nil {
			return "", "", fmt.Errorf("rate limit for %s: %w", provider, err)
		}
	}

	// Download using yt-dlp
	return p.runYtDlpDownload(ctx, url, outputPath)
}

// detectProvider detects the provider from a URL.
func (p *Provider) detectProvider(url string) string {
	urlLower := strings.ToLower(url)
	if strings.Contains(urlLower, "youtube.com") || strings.Contains(urlLower, "youtu.be") {
		return "youtube"
	}
	if strings.Contains(urlLower, "soundcloud.com") {
		return "soundcloud"
	}
	if strings.Contains(urlLower, "bandcamp.com") {
		return "bandcamp"
	}
	if strings.Contains(urlLower, "audius.co") {
		return "audius"
	}
	return "youtube"
}

// GetCacheStats returns cache statistics.
func (p *Provider) GetCacheStats() spotify.CacheStats {
	return p.searchCache.Stats()
}

// ClearCache clears the search cache.
func (p *Provider) ClearCache() {
	p.searchCache.Clear()
}
