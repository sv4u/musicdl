//go:build integration

package audio

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestProvider_Search_Integration(t *testing.T) {
	// Check if yt-dlp is available
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skip("yt-dlp not found in PATH, skipping integration test")
	}

	config := &Config{
		OutputFormat:   "mp3",
		Bitrate:        "128k",
		AudioProviders: []string{"youtube-music"},
		CacheMaxSize:   100,
		CacheTTL:       3600,
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()

	// Test search with a known query
	query := "Rush YYZ"
	url, err := provider.Search(ctx, query)
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	if url == "" {
		t.Error("Search returned empty URL")
	}

	t.Logf("Found URL: %s", url)

	// Test cache - second search should hit cache
	url2, err := provider.Search(ctx, query)
	if err != nil {
		t.Fatalf("Search (cached) failed: %v", err)
	}

	if url2 != url {
		t.Errorf("Cached search returned different URL: %s != %s", url2, url)
	}

	stats := provider.GetCacheStats()
	if stats.Hits < 1 {
		t.Error("Expected at least 1 cache hit")
	}

	t.Logf("Cache stats: Hits=%d, Misses=%d", stats.Hits, stats.Misses)
}

func TestProvider_Download_Integration(t *testing.T) {
	// Check if yt-dlp is available
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		t.Skip("yt-dlp not found in PATH, skipping integration test")
	}

	// Check if ffmpeg is available (needed for format conversion)
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found in PATH, skipping download test")
	}

	config := &Config{
		OutputFormat:   "mp3",
		Bitrate:        "128k",
		AudioProviders: []string{"youtube"},
		CacheMaxSize:   100,
		CacheTTL:       3600,
	}

	provider, err := NewProvider(config)
	if err != nil {
		t.Fatalf("Failed to create provider: %v", err)
	}

	ctx := context.Background()

	// Create temp directory for download
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "test_audio.mp3")

	// Use a short test video URL (YouTube)
	testURL := "https://www.youtube.com/watch?v=dQw4w9WgXcQ" // Short test video

	// Download (this will take a while, so we'll skip if it takes too long)
	downloadedPath, err := provider.Download(ctx, testURL, outputPath)
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if downloadedPath == "" {
		t.Fatal("Download returned empty path")
	}

	// Verify file exists
	if _, err := os.Stat(downloadedPath); err != nil {
		t.Fatalf("Downloaded file not found: %v", err)
	}

	t.Logf("Downloaded file: %s", downloadedPath)

	// Cleanup
	os.Remove(downloadedPath)
}
