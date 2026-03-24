package audio

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sv4u/musicdl/download/audius"
)

func TestNewProvider(t *testing.T) {
	config := &Config{
		OutputFormat:             "mp3",
		Bitrate:                  "128k",
		AudioProviders:           []string{"youtube-music", "youtube"},
		CacheMaxSize:             100,
		CacheTTL:                 3600,
		YouTubeRateLimitEnabled:  true,
		YouTubeRateLimitRequests: 2,
		YouTubeRateLimitWindow:   1.0,
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("NewProvider() failed: %v", err)
	}

	if provider == nil {
		t.Fatal("NewProvider() returned nil")
	}

	if provider.config.OutputFormat != "mp3" {
		t.Errorf("Expected output format 'mp3', got '%s'", provider.config.OutputFormat)
	}
}

func TestProvider_NormalizeQuery(t *testing.T) {
	config := &Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"youtube"},
	}
	provider, _ := NewProvider(config)

	query := "Artist - Song Name"
	normalized := provider.normalizeQuery(query)

	expected := "audio_search:artist - song name"
	if normalized != expected {
		t.Errorf("Expected normalized query '%s', got '%s'", expected, normalized)
	}
}

func TestProvider_DetectProvider(t *testing.T) {
	config := &Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"youtube"},
	}
	provider, _ := NewProvider(config)

	tests := []struct {
		url      string
		expected string
	}{
		{"https://www.youtube.com/watch?v=test", "youtube"},
		{"https://youtu.be/test", "youtube"},
		{"https://soundcloud.com/user/track", "soundcloud"},
		{"https://artist.bandcamp.com/track/song", "bandcamp"},
		{"https://artist.bandcamp.com/album/album-name", "bandcamp"},
		{"https://ARTIST.BANDCAMP.COM/track/song", "bandcamp"},
		{"https://audius.co/artist/track", "audius"},
		{"https://AUDIUS.CO/artist/track", "audius"},
		{"https://example.com/video", "youtube"}, // default
	}

	for _, tt := range tests {
		result := provider.detectProvider(tt.url)
		if result != tt.expected {
			t.Errorf("detectProvider(%q) = %q, expected %q", tt.url, result, tt.expected)
		}
	}
}

func TestProvider_Cache(t *testing.T) {
	config := &Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"youtube"},
		CacheMaxSize:   10,
		CacheTTL:       3600,
	}
	provider, _ := NewProvider(config)

	// Test cache operations
	provider.searchCache.Set("test:key", "test:value")
	value := provider.searchCache.Get("test:key")
	if value != "test:value" {
		t.Errorf("Expected cached value 'test:value', got %v", value)
	}

	stats := provider.GetCacheStats()
	if stats.Size != 1 {
		t.Errorf("Expected cache size 1, got %d", stats.Size)
	}

	provider.ClearCache()
	stats = provider.GetCacheStats()
	if stats.Size != 0 {
		t.Errorf("Expected cache size 0 after clear, got %d", stats.Size)
	}
}

func TestProvider_Search_CacheHit(t *testing.T) {
	config := &Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"youtube"},
		CacheMaxSize:   10,
		CacheTTL:       3600,
	}
	provider, _ := NewProvider(config)

	// Pre-populate cache
	query := "test query"
	cacheKey := provider.normalizeQuery(query)
	provider.searchCache.Set(cacheKey, "https://example.com/audio")

	// Search should hit cache
	ctx := context.Background()
	url, err := provider.Search(ctx, query)
	if err != nil {
		t.Fatalf("Search() error = %v", err)
	}
	if url != "https://example.com/audio" {
		t.Errorf("Expected cached URL, got %s", url)
	}

	stats := provider.GetCacheStats()
	if stats.Hits < 1 {
		t.Error("Expected at least 1 cache hit")
	}
}

func TestProvider_Search_CacheMiss_NoProviders(t *testing.T) {
	config := &Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{}, // No providers
		CacheMaxSize:   10,
		CacheTTL:       3600,
	}
	provider, _ := NewProvider(config)

	ctx := context.Background()
	_, err := provider.Search(ctx, "test query")
	if err == nil {
		t.Error("Search() should fail with no providers")
	}
}

func TestProvider_Search_ContextCancellation(t *testing.T) {
	config := &Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"youtube"},
		CacheMaxSize:   10,
		CacheTTL:       3600,
	}
	provider, _ := NewProvider(config)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := provider.Search(ctx, "test query")
	if err == nil {
		t.Error("Search() should fail with cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled, got %v", err)
	}
}

func TestProvider_SearchProvider(t *testing.T) {
	config := &Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"youtube"},
	}
	provider, _ := NewProvider(config)

	// Test that searchProvider builds correct query strings
	// Note: This test doesn't actually call yt-dlp, just verifies the function doesn't panic
	// Real search testing is done in integration tests
	tests := []struct {
		name     string
		provider string
		query    string
	}{
		{
			name:     "youtube-music",
			provider: "youtube-music",
			query:    "test",
		},
		{
			name:     "youtube",
			provider: "youtube",
			query:    "test",
		},
		{
			name:     "soundcloud",
			provider: "soundcloud",
			query:    "test",
		},
		{
			name:     "default",
			provider: "unknown",
			query:    "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			// This will attempt to call yt-dlp, which may or may not be available
			// We just verify the function doesn't panic
			_, err := provider.searchProviderWithCriteria(ctx, tt.provider, tt.query, nil)
			// Error is expected if yt-dlp is not available or query is invalid
			// Success is also valid if yt-dlp is available
			_ = err // Accept either outcome
		})
	}
}

func TestProvider_FindDownloadedFile(t *testing.T) {
	tmpDir := t.TempDir()
	basePath := filepath.Join(tmpDir, "test_audio")

	config := &Config{
		OutputFormat: "mp3",
	}
	provider, _ := NewProvider(config)

	// Test: file doesn't exist
	result := provider.findDownloadedFile(basePath + ".mp3")
	if result != "" {
		t.Errorf("Expected empty result for non-existent file, got %s", result)
	}

	// Test: file exists at expected path
	expectedPath := basePath + ".mp3"
	if err := os.WriteFile(expectedPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	result = provider.findDownloadedFile(expectedPath)
	if result != expectedPath {
		t.Errorf("Expected %s, got %s", expectedPath, result)
	}

	// Test: file exists with different extension
	_ = os.Remove(expectedPath)
	actualPath := basePath + ".m4a"
	if err := os.WriteFile(actualPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	result = provider.findDownloadedFile(basePath + ".mp3")
	if result != actualPath {
		t.Errorf("Expected %s, got %s", actualPath, result)
	}

	// Test: file exists with similar name
	_ = os.Remove(actualPath)
	similarPath := basePath + "_similar.mp3"
	if err := os.WriteFile(similarPath, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	result = provider.findDownloadedFile(basePath + ".mp3")
	if result != similarPath {
		t.Errorf("Expected %s, got %s", similarPath, result)
	}
}

func TestProvider_SearchAudius_Success(t *testing.T) {
	type searchResp struct {
		Data []audius.Track `json:"data"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/tracks/search" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResp{
			Data: []audius.Track{
				{
					ID:        "track-abc",
					Title:     "Best Match Song",
					Permalink: "best-match-song",
					User:      audius.User{Handle: "cool-artist", Name: "Cool Artist"},
					Duration:  240,
				},
			},
		})
	}))
	defer server.Close()

	provider, err := NewProvider(&Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"audius"},
		CacheMaxSize:   10,
		CacheTTL:       3600,
	})
	if err != nil {
		t.Fatalf("NewProvider() failed: %v", err)
	}
	provider.audiusClient = audius.NewClient(audius.WithBaseURL(server.URL))

	ctx := context.Background()
	url, err := provider.searchAudius(ctx, "Cool Artist Best Match Song")
	if err != nil {
		t.Fatalf("searchAudius() error: %v", err)
	}
	expected := "https://audius.co/cool-artist/best-match-song"
	if url != expected {
		t.Errorf("searchAudius() = %q, expected %q", url, expected)
	}
}

func TestProvider_SearchAudius_NoResults(t *testing.T) {
	type searchResp struct {
		Data []audius.Track `json:"data"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResp{Data: []audius.Track{}})
	}))
	defer server.Close()

	provider, _ := NewProvider(&Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"audius"},
	})
	provider.audiusClient = audius.NewClient(audius.WithBaseURL(server.URL))

	ctx := context.Background()
	_, err := provider.searchAudius(ctx, "nonexistent track xyz")
	if err == nil {
		t.Fatal("searchAudius() expected error for no results")
	}
	searchErr, ok := err.(*SearchError)
	if !ok {
		t.Fatalf("expected *SearchError, got %T", err)
	}
	if searchErr.Message != "No results from Audius search" {
		t.Errorf("unexpected error message: %q", searchErr.Message)
	}
}

func TestProvider_SearchAudius_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "service unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	provider, _ := NewProvider(&Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"audius"},
	})
	provider.audiusClient = audius.NewClient(audius.WithBaseURL(server.URL))

	ctx := context.Background()
	_, err := provider.searchAudius(ctx, "test query")
	if err == nil {
		t.Fatal("searchAudius() expected error for server error")
	}
	if _, ok := err.(*SearchError); !ok {
		t.Errorf("expected *SearchError, got %T", err)
	}
}

func TestProvider_SearchAudius_NilClient(t *testing.T) {
	provider, _ := NewProvider(&Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"youtube"},
	})
	// audiusClient is nil because "audius" is not in AudioProviders

	ctx := context.Background()
	_, err := provider.searchAudius(ctx, "test")
	if err == nil {
		t.Fatal("searchAudius() expected error when client is nil")
	}
	searchErr, ok := err.(*SearchError)
	if !ok {
		t.Fatalf("expected *SearchError, got %T", err)
	}
	if searchErr.Message != "Audius client not initialized" {
		t.Errorf("unexpected error message: %q", searchErr.Message)
	}
}

func TestProvider_SearchAudius_ViaSearchProvider(t *testing.T) {
	type searchResp struct {
		Data []audius.Track `json:"data"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResp{
			Data: []audius.Track{
				{
					ID:        "trk-1",
					Title:     "Found Track",
					Permalink: "found-track",
					User:      audius.User{Handle: "artist1", Name: "Artist One"},
				},
			},
		})
	}))
	defer server.Close()

	provider, _ := NewProvider(&Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"audius"},
		CacheMaxSize:   10,
		CacheTTL:       3600,
	})
	provider.audiusClient = audius.NewClient(audius.WithBaseURL(server.URL))

	ctx := context.Background()
	url, err := provider.searchProviderWithCriteria(ctx, "audius", "Artist One Found Track", nil)
	if err != nil {
		t.Fatalf("searchProvider(audius) error: %v", err)
	}
	expected := "https://audius.co/artist1/found-track"
	if url != expected {
		t.Errorf("searchProvider(audius) = %q, expected %q", url, expected)
	}
}

func TestProvider_Search_AudiusEndToEnd(t *testing.T) {
	type searchResp struct {
		Data []audius.Track `json:"data"`
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(searchResp{
			Data: []audius.Track{
				{
					ID:        "trk-e2e",
					Title:     "E2E Song",
					Permalink: "e2e-song",
					User:      audius.User{Handle: "e2e-artist"},
				},
			},
		})
	}))
	defer server.Close()

	provider, _ := NewProvider(&Config{
		OutputFormat:   "mp3",
		AudioProviders: []string{"audius"},
		CacheMaxSize:   10,
		CacheTTL:       3600,
	})
	provider.audiusClient = audius.NewClient(audius.WithBaseURL(server.URL))

	ctx := context.Background()
	url, err := provider.Search(ctx, "E2E Song")
	if err != nil {
		t.Fatalf("Search() with audius provider error: %v", err)
	}
	expected := "https://audius.co/e2e-artist/e2e-song"
	if url != expected {
		t.Errorf("Search() = %q, expected %q", url, expected)
	}

	// Second call should hit cache
	url2, err := provider.Search(ctx, "E2E Song")
	if err != nil {
		t.Fatalf("Search() cache hit error: %v", err)
	}
	if url2 != expected {
		t.Errorf("cached Search() = %q, expected %q", url2, expected)
	}
	stats := provider.GetCacheStats()
	if stats.Hits < 1 {
		t.Error("expected at least 1 cache hit")
	}
}

func TestDownload_ReturnsThreeValues(t *testing.T) {
	// This test verifies that Download returns (path, rawOutput, error)
	// by calling it with an invalid URL that will fail
	config := &Config{
		OutputFormat:   "mp3",
		Bitrate:        "128k",
		AudioProviders: []string{"youtube"},
	}
	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("NewProvider() failed: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use a clearly invalid URL that yt-dlp will reject quickly
	_, rawOutput, err := provider.Download(ctx, "not-a-valid-url", t.TempDir()+"/test.mp3")

	// We expect an error since the URL is invalid
	if err == nil {
		t.Fatal("Download() should fail with invalid URL")
	}

	// rawOutput may be empty or contain yt-dlp's error output
	// The key assertion is that three values are returned (compile-time check)
	_ = rawOutput
}
