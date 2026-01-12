package download

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/spotify"
)

func TestNewDownloader(t *testing.T) {
	cfg := &config.DownloadSettings{
		Format:  "mp3",
		Bitrate: "128k",
		Output:  "{artist}/{album}/{title}.{output-ext}",
	}

	spotifyClient := &spotify.SpotifyClient{}
	audioProvider := &audio.Provider{}
	metadataEmbedder := metadata.NewEmbedder()

	downloader := NewDownloader(cfg, spotifyClient, audioProvider, metadataEmbedder)
	if downloader == nil {
		t.Fatal("NewDownloader() returned nil")
	}
}

func TestDownloader_GetOutputPath(t *testing.T) {
	cfg := &config.DownloadSettings{
		Format:  "mp3",
		Bitrate: "128k",
		Output:  "{artist}/{album}/{title}.{output-ext}",
	}

	downloader := NewDownloader(cfg, nil, nil, nil)

	song := &metadata.Song{
		Title:  "Test Song",
		Artist: "Test Artist",
		Album:  "Test Album",
	}

	outputPath := downloader.getOutputPath(song)
	expected := "Test Artist/Test Album/Test Song.mp3"
	if outputPath != expected {
		t.Errorf("Expected output path '%s', got '%s'", expected, outputPath)
	}
}

func TestDownloader_SanitizeFilename(t *testing.T) {
	cfg := &config.DownloadSettings{
		Output: "{title}.{output-ext}",
	}
	downloader := NewDownloader(cfg, nil, nil, nil)

	// Test that invalid characters are replaced
	song := &metadata.Song{
		Title:  "Song/With:Invalid*Chars",
		Artist: "Test",
		Album:  "Test",
	}
	result := downloader.getOutputPath(song)
	
	// Should not contain invalid characters
	if strings.Contains(result, "/") || strings.Contains(result, ":") || strings.Contains(result, "*") {
		t.Errorf("Output path should not contain invalid characters: %s", result)
	}
}

func TestDownloader_FileExistsCached(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp3")
	
	// Create test file
	os.WriteFile(testFile, []byte("test"), 0644)

	cfg := &config.DownloadSettings{}
	downloader := NewDownloader(cfg, nil, nil, nil)

	// Test file exists
	if !downloader.fileExistsCached(testFile) {
		t.Error("Expected file to exist")
	}

	// Test file doesn't exist
	nonexistent := filepath.Join(tmpDir, "nonexistent.mp3")
	if downloader.fileExistsCached(nonexistent) {
		t.Error("Expected file to not exist")
	}

	// Test caching
	if !downloader.fileExistsCached(testFile) {
		t.Error("Expected cached result to show file exists")
	}
}

func TestExtractYear(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"2024", 2024},
		{"2024-01", 2024},
		{"2024-01-15", 2024},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		result := extractYear(tt.input)
		if result != tt.expected {
			t.Errorf("extractYear(%q) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

func TestDownloader_GetOutputPath_WithTrackNumber(t *testing.T) {
	cfg := &config.DownloadSettings{
		Format:  "mp3",
		Bitrate: "128k",
		Output:  "{artist}/{album}/{track-number} - {title}.{output-ext}",
	}

	downloader := NewDownloader(cfg, nil, nil, nil)

	song := &metadata.Song{
		Title:       "Test Song",
		Artist:      "Test Artist",
		Album:       "Test Album",
		TrackNumber: 5,
	}

	outputPath := downloader.getOutputPath(song)
	expected := "Test Artist/Test Album/05 - Test Song.mp3"
	if outputPath != expected {
		t.Errorf("Expected output path '%s', got '%s'", expected, outputPath)
	}
}

func TestDownloader_GetOutputPath_DefaultTemplate(t *testing.T) {
	cfg := &config.DownloadSettings{
		Format:  "mp3",
		Bitrate: "128k",
		Output:  "", // Empty should use default
	}

	downloader := NewDownloader(cfg, nil, nil, nil)

	song := &metadata.Song{
		Title:       "Test Song",
		Artist:      "Test Artist",
		Album:       "Test Album",
		TrackNumber: 1,
	}

	outputPath := downloader.getOutputPath(song)
	// Default template is "{artist}/{album}/{track-number} - {title}.{output-ext}"
	expected := "Test Artist/Test Album/01 - Test Song.mp3"
	if outputPath != expected {
		t.Errorf("Expected output path '%s', got '%s'", expected, outputPath)
	}
}

func TestDownloader_GetOutputPath_DirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.DownloadSettings{
		Format:  "mp3",
		Bitrate: "128k",
		Output:  tmpDir + "/{artist}/{album}/{title}.{output-ext}",
	}

	downloader := NewDownloader(cfg, nil, nil, nil)

	song := &metadata.Song{
		Title:  "Test Song",
		Artist: "Test Artist",
		Album:  "Test Album",
	}

	outputPath := downloader.getOutputPath(song)
	expectedDir := filepath.Join(tmpDir, "Test Artist", "Test Album")
	
	// Directory should be created
	if _, err := os.Stat(expectedDir); err != nil {
		t.Errorf("Expected directory to be created: %v", err)
	}
	
	expectedPath := filepath.Join(expectedDir, "Test Song.mp3")
	if outputPath != expectedPath {
		t.Errorf("Expected output path '%s', got '%s'", expectedPath, outputPath)
	}
}

func TestDownloader_FileExistsCached_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp3")
	os.WriteFile(testFile, []byte("test"), 0644)

	cfg := &config.DownloadSettings{}
	downloader := NewDownloader(cfg, nil, nil, nil)

	// Test concurrent access (should be safe due to RWMutex)
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func() {
			downloader.fileExistsCached(testFile)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Should still work correctly
	if !downloader.fileExistsCached(testFile) {
		t.Error("Expected file to exist after concurrent access")
	}
}

func TestDownloader_FileExistsCached_CacheInvalidation(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp3")

	cfg := &config.DownloadSettings{}
	downloader := NewDownloader(cfg, nil, nil, nil)

	// File doesn't exist initially
	if downloader.fileExistsCached(testFile) {
		t.Error("Expected file to not exist")
	}

	// Create file
	os.WriteFile(testFile, []byte("test"), 0644)

	// Cache should still say it doesn't exist (cached)
	if downloader.fileExistsCached(testFile) {
		t.Error("Expected cached result to show file doesn't exist")
	}

	// Invalidate cache
	downloader.invalidateFileCache(testFile)

	// Now should detect file exists
	if !downloader.fileExistsCached(testFile) {
		t.Error("Expected file to exist after cache invalidation")
	}
}

func TestDownloader_FileExistsCached_SetFileExistsCached(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp3")

	cfg := &config.DownloadSettings{}
	downloader := NewDownloader(cfg, nil, nil, nil)

	// Set file as existing in cache (even though it doesn't exist on disk)
	downloader.setFileExistsCached(testFile, true)

	// Should return cached value
	if !downloader.fileExistsCached(testFile) {
		t.Error("Expected cached value to show file exists")
	}
}

func TestDownloader_SanitizeFilename_AllInvalidChars(t *testing.T) {
	cfg := &config.DownloadSettings{
		Output: "{title}.{output-ext}",
	}
	downloader := NewDownloader(cfg, nil, nil, nil)

	invalidChars := []string{"/", "\\", ":", "*", "?", "\"", "<", ">", "|"}
	
	for _, char := range invalidChars {
		song := &metadata.Song{
			Title:  "Song" + char + "Test",
			Artist: "Test",
			Album:  "Test",
		}
		result := downloader.getOutputPath(song)
		
		if strings.Contains(result, char) {
			t.Errorf("Output path should not contain invalid character '%s': %s", char, result)
		}
	}
}

func TestDownloader_SanitizeFilename_EmptyFields(t *testing.T) {
	cfg := &config.DownloadSettings{
		Output: "{artist}/{album}/{title}.{output-ext}",
	}
	downloader := NewDownloader(cfg, nil, nil, nil)

	song := &metadata.Song{
		Title:  "Test Song",
		Artist: "", // Empty artist
		Album:  "", // Empty album
	}

	outputPath := downloader.getOutputPath(song)
	// Should handle empty fields gracefully
	if outputPath == "" {
		t.Error("Expected non-empty output path even with empty fields")
	}
}

func TestDownloader_ExtractYear_EdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"2024", 2024},
		{"2024-01", 2024},
		{"2024-01-15", 2024},
		{"", 0},
		{"invalid", 0},
		{"2024-12-31", 2024},
		{"1999-01-01", 1999},
		{"2000", 2000},
		{"abc-2024", 0}, // Invalid format
		{"2024-abc", 2024}, // Partial match
	}

	for _, tt := range tests {
		result := extractYear(tt.input)
		if result != tt.expected {
			t.Errorf("extractYear(%q) = %d, expected %d", tt.input, result, tt.expected)
		}
	}
}

// Note: Full DownloadTrack behavioral tests with retries, API calls, etc.
// should be integration tests (see integration tests section).
// These unit tests focus on testable behaviors without external dependencies.
