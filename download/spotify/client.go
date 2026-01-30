package spotify

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sv4u/spotigo"
)

// Config holds configuration for the Spotify client wrapper.
type Config struct {
	// Spotify API credentials
	ClientID     string
	ClientSecret string

	// Cache configuration
	CacheMaxSize         int
	CacheTTL             int
	CacheCleanupInterval time.Duration // 0 = disabled

	// Rate limiting configuration
	RateLimitEnabled  bool
	RateLimitRequests int
	RateLimitWindow   float64

	// Optional general rate limiter for network impact management
	GeneralRateLimiter interface {
		WaitForRequest(ctx context.Context) error
	}

	// Retry configuration (passed to spotigo, but documented here)
	MaxRetries     int
	RetryBaseDelay float64
	RetryMaxDelay  float64
}

// SpotifyClient is a wrapper around spotigo.Client that adds:
// - Proactive rate limiting
// - Response caching
// - Rate limit state tracking
type SpotifyClient struct {
	client             *spotigo.Client
	cache              *TTLCache
	rateLimiter        *RateLimiter
	rateLimitTracker   *RateLimitTracker
	generalRateLimiter interface {
		WaitForRequest(ctx context.Context) error
	}
	config *Config
}

// NewSpotifyClient creates a new Spotify client wrapper.
func NewSpotifyClient(config *Config) (*SpotifyClient, error) {
	// Create spotigo auth manager
	auth, err := spotigo.NewClientCredentials(config.ClientID, config.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth: %w", err)
	}

	// Create spotigo client
	spotigoClient, err := spotigo.NewClient(auth)
	if err != nil {
		return nil, fmt.Errorf("failed to create spotigo client: %w", err)
	}

	// Create wrapper components
	cache := NewTTLCache(config.CacheMaxSize, config.CacheTTL)
	if config.CacheCleanupInterval > 0 {
		cache.StartCleanup(config.CacheCleanupInterval)
	}

	rateLimiter := NewRateLimiter(
		config.RateLimitEnabled,
		config.RateLimitRequests,
		config.RateLimitWindow,
	)

	tracker := NewRateLimitTracker()

	return &SpotifyClient{
		client:             spotigoClient,
		cache:              cache,
		rateLimiter:        rateLimiter,
		rateLimitTracker:   tracker,
		generalRateLimiter: config.GeneralRateLimiter,
		config:             config,
	}, nil
}

// applyRateLimiting applies both general and Spotify-specific rate limiting.
func (c *SpotifyClient) applyRateLimiting(ctx context.Context) error {
	// Check context cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	// Apply general rate limiter if enabled
	if c.generalRateLimiter != nil {
		if err := c.generalRateLimiter.WaitForRequest(ctx); err != nil {
			return err
		}
	}

	// Apply Spotify-specific rate limiter
	if err := c.rateLimiter.WaitIfNeeded(ctx); err != nil {
		return err
	}

	return nil
}

// handleError processes spotigo errors and updates rate limit state.
func (c *SpotifyClient) handleError(err error) error {
	if err == nil {
		return nil
	}

	// Check if it's a rate limit error
	if c.isRateLimitError(err) {
		retryAfter := c.extractRetryAfter(err)
		c.rateLimitTracker.Update(retryAfter)
		return &RateLimitError{
			RetryAfter: retryAfter,
			Original:   err,
		}
	}

	// Wrap other errors
	return &SpotifyError{
		Message:  "Spotify API error",
		Original: err,
	}
}

// isRateLimitError checks if an error is a rate limit error (HTTP 429).
func (c *SpotifyClient) isRateLimitError(err error) bool {
	// Check for HTTP 429 status
	if httpErr, ok := err.(interface {
		StatusCode() int
	}); ok {
		return httpErr.StatusCode() == http.StatusTooManyRequests
	}

	// Check error message for rate limit indicators
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests")
}

// extractRetryAfter extracts Retry-After value from error.
func (c *SpotifyClient) extractRetryAfter(err error) int {
	// Default retry after 1 second if we can't determine
	defaultRetryAfter := 1

	// Try to extract from error if it has retry-after information
	// This is a simplified version - spotigo may provide this differently
	if httpErr, ok := err.(interface {
		RetryAfter() int
	}); ok {
		if retryAfter := httpErr.RetryAfter(); retryAfter > 0 {
			return retryAfter
		}
	}

	return defaultRetryAfter
}

// GetRateLimitInfo returns the current rate limit state.
func (c *SpotifyClient) GetRateLimitInfo() *RateLimitInfo {
	return c.rateLimitTracker.GetInfo()
}

// ClearCache clears the response cache.
func (c *SpotifyClient) ClearCache() {
	c.cache.Clear()
}

// GetCacheStats returns cache statistics.
func (c *SpotifyClient) GetCacheStats() CacheStats {
	return c.cache.Stats()
}

// Close gracefully shuts down the client.
func (c *SpotifyClient) Close() {
	c.cache.StopCleanup()
}

// GetTrack retrieves track metadata (cached).
func (c *SpotifyClient) GetTrack(ctx context.Context, trackIDOrURL string) (*spotigo.Track, error) {
	// Extract ID for cache key (spotigo handles URL/URI/ID parsing)
	trackID, err := spotigo.GetID(trackIDOrURL, "track")
	if err != nil {
		return nil, fmt.Errorf("invalid track ID/URL: %w", err)
	}

	// Check cache
	cacheKey := fmt.Sprintf("track:%s", trackID)
	if cached := c.cache.Get(cacheKey); cached != nil {
		if track, ok := cached.(*spotigo.Track); ok {
			return track, nil
		}
	}

	// Apply rate limiting
	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	// Call spotigo (it handles URL/URI/ID parsing)
	track, err := c.client.Track(ctx, trackIDOrURL)
	if err != nil {
		return nil, c.handleError(err)
	}

	// Cache result
	c.cache.Set(cacheKey, track)

	// Clear rate limit state on success
	c.rateLimitTracker.Clear()

	return track, nil
}

// GetAlbum retrieves album metadata (cached).
func (c *SpotifyClient) GetAlbum(ctx context.Context, albumIDOrURL string) (*spotigo.Album, error) {
	albumID, err := spotigo.GetID(albumIDOrURL, "album")
	if err != nil {
		return nil, fmt.Errorf("invalid album ID/URL: %w", err)
	}

	cacheKey := fmt.Sprintf("album:%s", albumID)
	if cached := c.cache.Get(cacheKey); cached != nil {
		if album, ok := cached.(*spotigo.Album); ok {
			return album, nil
		}
	}

	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	album, err := c.client.Album(ctx, albumIDOrURL)
	if err != nil {
		return nil, c.handleError(err)
	}

	c.cache.Set(cacheKey, album)
	c.rateLimitTracker.Clear()

	return album, nil
}

// GetPlaylist retrieves playlist metadata (cached).
func (c *SpotifyClient) GetPlaylist(ctx context.Context, playlistIDOrURL string) (*spotigo.Playlist, error) {
	playlistID, err := spotigo.GetID(playlistIDOrURL, "playlist")
	if err != nil {
		return nil, fmt.Errorf("invalid playlist ID/URL: %w", err)
	}

	cacheKey := fmt.Sprintf("playlist:%s", playlistID)
	if cached := c.cache.Get(cacheKey); cached != nil {
		if playlist, ok := cached.(*spotigo.Playlist); ok {
			return playlist, nil
		}
	}

	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	playlist, err := c.client.Playlist(ctx, playlistIDOrURL, nil)
	if err != nil {
		return nil, c.handleError(err)
	}

	c.cache.Set(cacheKey, playlist)
	c.rateLimitTracker.Clear()

	return playlist, nil
}

// GetArtist retrieves artist metadata (cached).
func (c *SpotifyClient) GetArtist(ctx context.Context, artistIDOrURL string) (*spotigo.Artist, error) {
	artistID, err := spotigo.GetID(artistIDOrURL, "artist")
	if err != nil {
		return nil, fmt.Errorf("invalid artist ID/URL: %w", err)
	}

	cacheKey := fmt.Sprintf("artist:%s", artistID)
	if cached := c.cache.Get(cacheKey); cached != nil {
		if artist, ok := cached.(*spotigo.Artist); ok {
			return artist, nil
		}
	}

	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	artist, err := c.client.Artist(ctx, artistIDOrURL)
	if err != nil {
		return nil, c.handleError(err)
	}

	c.cache.Set(cacheKey, artist)
	c.rateLimitTracker.Clear()

	return artist, nil
}

// GetArtistAlbums retrieves all albums and singles for an artist (cached).
// Excludes compilations and "Appears On" albums.
func (c *SpotifyClient) GetArtistAlbums(ctx context.Context, artistIDOrURL string) ([]spotigo.SimplifiedAlbum, error) {
	artistID, err := spotigo.GetID(artistIDOrURL, "artist")
	if err != nil {
		return nil, fmt.Errorf("invalid artist ID/URL: %w", err)
	}

	cacheKey := fmt.Sprintf("artist_albums:%s", artistID)
	if cached := c.cache.Get(cacheKey); cached != nil {
		if albums, ok := cached.([]spotigo.SimplifiedAlbum); ok {
			return albums, nil
		}
	}

	// Fetch first page with rate limiting
	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	// Get first page - filter to albums and singles only
	paging, err := c.client.ArtistAlbums(ctx, artistID, &spotigo.ArtistAlbumsOptions{
		IncludeGroups: []string{"album", "single"},
		Limit:         50,
	})
	if err != nil {
		return nil, c.handleError(err)
	}

	allAlbums := make([]spotigo.SimplifiedAlbum, 0, len(paging.Items))
	allAlbums = append(allAlbums, paging.Items...)

	// Paginate through remaining pages
	for paging.GetNext() != nil {
		// Check context before each page
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("context cancelled during pagination: %w", err)
		}

		// Get next page with rate limiting
		paging, err = c.NextWithRateLimit(ctx, paging)
		if err != nil {
			return nil, fmt.Errorf("failed to paginate artist albums: %w", err)
		}
		if paging == nil {
			break
		}

		allAlbums = append(allAlbums, paging.Items...)
	}

	// Cache complete result
	c.cache.Set(cacheKey, allAlbums)
	c.rateLimitTracker.Clear()

	return allAlbums, nil
}

// NextWithRateLimit gets the next page of results with rate limiting.
func (c *SpotifyClient) NextWithRateLimit(ctx context.Context, paging interface{ GetNext() *string }) (*spotigo.Paging[spotigo.SimplifiedAlbum], error) {
	// Apply rate limiting
	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	// Use spotigo's type-safe pagination
	return spotigo.NextGeneric[spotigo.SimplifiedAlbum](c.client, ctx, paging)
}

// NextAlbumTracks gets the next page of album tracks with rate limiting.
func (c *SpotifyClient) NextAlbumTracks(ctx context.Context, paging interface{ GetNext() *string }) (*spotigo.Paging[spotigo.SimplifiedTrack], error) {
	// Apply rate limiting
	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	// Use spotigo's type-safe pagination
	return spotigo.NextGeneric[spotigo.SimplifiedTrack](c.client, ctx, paging)
}

// NextPlaylistTracks gets the next page of playlist tracks with rate limiting.
func (c *SpotifyClient) NextPlaylistTracks(ctx context.Context, paging interface{ GetNext() *string }) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
	// Apply rate limiting
	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	// Use spotigo's type-safe pagination
	return spotigo.NextGeneric[spotigo.PlaylistTrack](c.client, ctx, paging)
}

// GetPlaylistTracks retrieves tracks from a playlist (cached).
func (c *SpotifyClient) GetPlaylistTracks(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
	// Check cache
	cacheKey := fmt.Sprintf("playlist_tracks:%s", playlistID)
	if cached := c.cache.Get(cacheKey); cached != nil {
		if tracks, ok := cached.(*spotigo.Paging[spotigo.PlaylistTrack]); ok {
			return tracks, nil
		}
	}

	// Apply rate limiting
	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	// Call spotigo
	tracks, err := c.client.PlaylistTracks(ctx, playlistID, opts)
	if err != nil {
		return nil, c.handleError(err)
	}

	// Cache result
	c.cache.Set(cacheKey, tracks)

	// Clear rate limit state on success
	c.rateLimitTracker.Clear()

	return tracks, nil
}

// Search searches for tracks, artists, albums, etc. on Spotify (cached).
// searchType should be one of: "track", "artist", "album", "playlist", "show", "episode", "audiobook"
// or a comma-separated list like "track,album"
func (c *SpotifyClient) Search(ctx context.Context, query, searchType string, opts *spotigo.SearchOptions) (*spotigo.SearchResponse, error) {
	// Create cache key from query and search type
	cacheKey := fmt.Sprintf("search:%s:%s", searchType, query)
	if cached := c.cache.Get(cacheKey); cached != nil {
		if response, ok := cached.(*spotigo.SearchResponse); ok {
			return response, nil
		}
	}

	// Apply rate limiting
	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	// Call spotigo
	response, err := c.client.Search(ctx, query, searchType, opts)
	if err != nil {
		return nil, c.handleError(err)
	}

	// Cache result
	c.cache.Set(cacheKey, response)

	// Clear rate limit state on success
	c.rateLimitTracker.Clear()

	return response, nil
}
