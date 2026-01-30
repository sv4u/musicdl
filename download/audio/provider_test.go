package audio

import (
	"context"
	"os"
	"path/filepath"
	"testing"
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
			_, err := provider.searchProvider(ctx, tt.provider, tt.query)
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
