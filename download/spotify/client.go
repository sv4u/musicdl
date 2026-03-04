package spotify

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sv4u/spotigo/v2"
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
// - Rate limit state tracking via WithRateLimitCallback
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
	auth, err := spotigo.NewClientCredentials(config.ClientID, config.ClientSecret)
	if err != nil {
		return nil, fmt.Errorf("failed to create auth: %w", err)
	}

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

	spotigoClient, err := spotigo.NewClient(auth,
		spotigo.WithRateLimitCallback(func(retryAfter time.Duration) {
			if sec := int(retryAfter.Seconds()); sec > 0 {
				tracker.Update(sec)
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create spotigo client: %w", err)
	}

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
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	if c.generalRateLimiter != nil {
		if err := c.generalRateLimiter.WaitForRequest(ctx); err != nil {
			return err
		}
	}

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

	var spotifyErr *spotigo.SpotifyError
	if errors.As(err, &spotifyErr) && spotifyErr.StatusCode() == 429 {
		retryAfter := 1
		if duration, hasRetryAfter := spotifyErr.RetryAfter(); hasRetryAfter && duration > 0 {
			retryAfter = int(duration.Seconds())
		}
		c.rateLimitTracker.Update(retryAfter)
		return &RateLimitError{
			RetryAfter: retryAfter,
			Original:   err,
		}
	}

	return &SpotifyError{
		Message:  "Spotify API error",
		Original: err,
	}
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
	trackID, err := spotigo.GetID(trackIDOrURL, "track")
	if err != nil {
		return nil, fmt.Errorf("invalid track ID/URL: %w", err)
	}

	cacheKey := fmt.Sprintf("track:%s", trackID)
	if cached := c.cache.Get(cacheKey); cached != nil {
		if track, ok := cached.(*spotigo.Track); ok {
			return track, nil
		}
	}

	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	track, err := c.client.Track(ctx, trackIDOrURL)
	if err != nil {
		return nil, c.handleError(err)
	}

	c.cache.Set(cacheKey, track)
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

// AllArtistAlbums retrieves all albums and singles for an artist (cached).
// Delegates pagination to spotigo's AllArtistAlbums. Excludes compilations and "Appears On" albums.
func (c *SpotifyClient) AllArtistAlbums(ctx context.Context, artistIDOrURL string) ([]spotigo.SimplifiedAlbum, error) {
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

	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	allAlbums, err := c.client.AllArtistAlbums(ctx, artistID, &spotigo.ArtistAlbumsOptions{
		IncludeGroups: []string{"album", "single"},
		Limit:         50,
	})
	if err != nil {
		return nil, c.handleError(err)
	}

	c.cache.Set(cacheKey, allAlbums)
	c.rateLimitTracker.Clear()

	return allAlbums, nil
}

// AllAlbumTracks retrieves all tracks from an album (cached).
// Delegates pagination to spotigo's AllAlbumTracks.
func (c *SpotifyClient) AllAlbumTracks(ctx context.Context, albumIDOrURL string) ([]spotigo.SimplifiedTrack, error) {
	albumID, err := spotigo.GetID(albumIDOrURL, "album")
	if err != nil {
		return nil, fmt.Errorf("invalid album ID/URL: %w", err)
	}

	cacheKey := fmt.Sprintf("album_tracks:%s", albumID)
	if cached := c.cache.Get(cacheKey); cached != nil {
		if tracks, ok := cached.([]spotigo.SimplifiedTrack); ok {
			return tracks, nil
		}
	}

	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	allTracks, err := c.client.AllAlbumTracks(ctx, albumID, nil)
	if err != nil {
		return nil, c.handleError(err)
	}

	c.cache.Set(cacheKey, allTracks)
	c.rateLimitTracker.Clear()

	return allTracks, nil
}

// AllPlaylistTracks retrieves all tracks from a playlist (cached).
// Delegates pagination to spotigo's AllPlaylistTracks.
func (c *SpotifyClient) AllPlaylistTracks(ctx context.Context, playlistIDOrURL string) ([]spotigo.PlaylistTrack, error) {
	playlistID, err := spotigo.GetID(playlistIDOrURL, "playlist")
	if err != nil {
		return nil, fmt.Errorf("invalid playlist ID/URL: %w", err)
	}

	cacheKey := fmt.Sprintf("all_playlist_tracks:%s", playlistID)
	if cached := c.cache.Get(cacheKey); cached != nil {
		if tracks, ok := cached.([]spotigo.PlaylistTrack); ok {
			return tracks, nil
		}
	}

	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	allTracks, err := c.client.AllPlaylistTracks(ctx, playlistID, nil)
	if err != nil {
		return nil, c.handleError(err)
	}

	c.cache.Set(cacheKey, allTracks)
	c.rateLimitTracker.Clear()

	return allTracks, nil
}

// Search searches for tracks on Spotify (cached).
func (c *SpotifyClient) Search(ctx context.Context, query, searchType string, opts *spotigo.SearchOptions) (*spotigo.SearchResponse, error) {
	cacheKey := fmt.Sprintf("search:%s:%s", searchType, query)
	if cached := c.cache.Get(cacheKey); cached != nil {
		if response, ok := cached.(*spotigo.SearchResponse); ok {
			return response, nil
		}
	}

	if err := c.applyRateLimiting(ctx); err != nil {
		return nil, err
	}

	response, err := c.client.Search(ctx, query, searchType, opts)
	if err != nil {
		return nil, c.handleError(err)
	}

	c.cache.Set(cacheKey, response)
	c.rateLimitTracker.Clear()

	return response, nil
}