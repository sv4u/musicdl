//go:build integration

package spotify

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/joho/godotenv"
)

func TestSpotifyClient_Integration(t *testing.T) {
	// Try to load .env file (ignore error if it doesn't exist)
	_ = godotenv.Load()

	// Load credentials from environment (try both SPOTIFY_ and SPOTIGO_ prefixes)
	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	if clientID == "" {
		clientID = os.Getenv("SPOTIGO_CLIENT_ID")
	}
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("SPOTIGO_CLIENT_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		t.Skip("SPOTIFY_CLIENT_ID/SPOTIGO_CLIENT_ID and SPOTIFY_CLIENT_SECRET/SPOTIGO_CLIENT_SECRET required for integration tests")
	}

	// Create client
	config := &Config{
		ClientID:             clientID,
		ClientSecret:         clientSecret,
		CacheMaxSize:         100,
		CacheTTL:             3600,
		RateLimitEnabled:     true,
		RateLimitRequests:    10,
		RateLimitWindow:      1.0,
		CacheCleanupInterval: 5 * time.Minute,
	}

	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test GetTrack with a known track (Rush - YYZ)
	trackURL := "https://open.spotify.com/track/4iV5W9uYEdYUVa79Axb7Rh"
	track, err := client.GetTrack(ctx, trackURL)
	if err != nil {
		t.Fatalf("GetTrack failed: %v", err)
	}

	if track == nil {
		t.Fatal("GetTrack returned nil")
	}

	if track.Name == "" {
		t.Error("Track name is empty")
	}

	t.Logf("Track: %s by %s", track.Name, track.Artists[0].Name)

	// Test cache - second call should hit cache
	start := time.Now()
	track2, err := client.GetTrack(ctx, trackURL)
	cacheDuration := time.Since(start)

	if err != nil {
		t.Fatalf("GetTrack (cached) failed: %v", err)
	}

	if track2 == nil {
		t.Fatal("GetTrack (cached) returned nil")
	}

	// Cache hit should be very fast (< 10ms)
	if cacheDuration > 10*time.Millisecond {
		t.Logf("Cache hit took %v (expected < 10ms)", cacheDuration)
	}

	// Verify cache stats
	stats := client.GetCacheStats()
	if stats.Hits < 1 {
		t.Error("Expected at least 1 cache hit")
	}

	t.Logf("Cache stats: Hits=%d, Misses=%d, HitRate=%.2f%%", stats.Hits, stats.Misses, stats.HitRate*100)
}

func TestSpotifyClient_GetArtistAlbums_Integration(t *testing.T) {
	// Try to load .env file
	_ = godotenv.Load()

	clientID := os.Getenv("SPOTIFY_CLIENT_ID")
	if clientID == "" {
		clientID = os.Getenv("SPOTIGO_CLIENT_ID")
	}
	clientSecret := os.Getenv("SPOTIFY_CLIENT_SECRET")
	if clientSecret == "" {
		clientSecret = os.Getenv("SPOTIGO_CLIENT_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		t.Skip("SPOTIFY_CLIENT_ID/SPOTIGO_CLIENT_ID and SPOTIFY_CLIENT_SECRET/SPOTIGO_CLIENT_SECRET required")
	}

	config := &Config{
		ClientID:          clientID,
		ClientSecret:      clientSecret,
		CacheMaxSize:      100,
		CacheTTL:          3600,
		RateLimitEnabled:  true,
		RateLimitRequests: 10,
		RateLimitWindow:   1.0,
	}

	client, err := NewSpotifyClient(config)
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// Test GetArtistAlbums with a known artist
	artistURL := "https://open.spotify.com/artist/2YZyLoL8N0Wb9xBt1NhZWg" // Kendrick Lamar
	albums, err := client.GetArtistAlbums(ctx, artistURL)
	if err != nil {
		t.Fatalf("GetArtistAlbums failed: %v", err)
	}

	if len(albums) == 0 {
		t.Error("Expected at least one album")
	}

	t.Logf("Found %d albums for artist", len(albums))

	// Verify albums are albums or singles only (not compilations)
	for _, album := range albums {
		if album.AlbumType != "album" && album.AlbumType != "single" {
			t.Errorf("Unexpected album type: %s (expected album or single)", album.AlbumType)
		}
	}
}
