package download

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/plan"
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
	// Reduce memory usage when running with race detector
	numGoroutines := 10
	if testing.RaceEnabled() {
		numGoroutines = 5
	}
	done := make(chan bool, numGoroutines)
	for i := 0; i < numGoroutines; i++ {
		go func() {
			downloader.fileExistsCached(testFile)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
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

func TestYoutubeMetadataToSong(t *testing.T) {
	ytMetadata := &audio.YouTubeVideoMetadata{
		VideoID:   "dQw4w9WgXcQ",
		Title:     "Test Video Title",
		Uploader:  "Test Artist",
		Duration:  200,
		UploadDate: "2024-01-15",
	}

	item := &plan.PlanItem{
		ItemID:     "track:youtube:dQw4w9WgXcQ",
		ItemType:   plan.PlanItemTypeTrack,
		YouTubeURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		Name:       "Test Video",
		Metadata:   make(map[string]interface{}),
	}

	song := youtubeMetadataToSong(ytMetadata, item)

	if song.Title != "Test Video Title" {
		t.Errorf("Expected Title to be 'Test Video Title', got '%s'", song.Title)
	}
	if song.Artist != "Test Artist" {
		t.Errorf("Expected Artist to be 'Test Artist', got '%s'", song.Artist)
	}
	if song.Album != "YouTube" {
		t.Errorf("Expected Album to be 'YouTube', got '%s'", song.Album)
	}
	if song.Duration != 200 {
		t.Errorf("Expected Duration to be 200, got %d", song.Duration)
	}
	if song.Year != 2024 {
		t.Errorf("Expected Year to be 2024, got %d", song.Year)
	}
	if song.Date != "2024-01-15" {
		t.Errorf("Expected Date to be '2024-01-15', got '%s'", song.Date)
	}
}

func TestYoutubeMetadataToSong_WithItemNameFallback(t *testing.T) {
	ytMetadata := &audio.YouTubeVideoMetadata{
		VideoID:  "dQw4w9WgXcQ",
		Title:    "", // Empty title
		Uploader: "Test Artist",
		Duration: 200,
	}

	item := &plan.PlanItem{
		ItemID:     "track:youtube:dQw4w9WgXcQ",
		ItemType:   plan.PlanItemTypeTrack,
		YouTubeURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		Name:       "Test Video Name", // Should be used as fallback
		Metadata:   make(map[string]interface{}),
	}

	song := youtubeMetadataToSong(ytMetadata, item)

	if song.Title != "Test Video Name" {
		t.Errorf("Expected Title to fallback to item name 'Test Video Name', got '%s'", song.Title)
	}
}

func TestYoutubeMetadataToSong_WithMetadataArtist(t *testing.T) {
	ytMetadata := &audio.YouTubeVideoMetadata{
		VideoID:  "dQw4w9WgXcQ",
		Title:    "Test Video",
		Uploader: "Uploader Name",
		Duration: 200,
	}

	item := &plan.PlanItem{
		ItemID:     "track:youtube:dQw4w9WgXcQ",
		ItemType:   plan.PlanItemTypeTrack,
		YouTubeURL: "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
		Name:       "Test Video",
		Metadata: map[string]interface{}{
			"artist": "Metadata Artist", // Should override uploader
		},
	}

	song := youtubeMetadataToSong(ytMetadata, item)

	if song.Artist != "Metadata Artist" {
		t.Errorf("Expected Artist to be 'Metadata Artist' from metadata, got '%s'", song.Artist)
	}
}

func TestApplySpotifyEnhancement(t *testing.T) {
	song := &metadata.Song{
		Title:    "Test Song",
		Artist:   "YouTube Artist",
		Album:    "YouTube",
		Duration: 200,
	}

	item := &plan.PlanItem{
		Metadata: map[string]interface{}{
			"spotify_enhancement": map[string]interface{}{
				"album":        "Enhanced Album",
				"artist":       "Enhanced Artist",
				"track_number": 5,
				"disc_number":  1,
				"year":         2023,
				"date":         "2023-06-15",
				"isrc":         "USRC12345678",
				"cover_url":    "https://example.com/cover.jpg",
				"spotify_url":  "https://open.spotify.com/track/123",
				"explicit":     true,
				"tracks_count": 12,
			},
		},
	}

	applySpotifyEnhancement(song, item)

	if song.Album != "Enhanced Album" {
		t.Errorf("Expected Album to be 'Enhanced Album', got '%s'", song.Album)
	}
	if song.Artist != "Enhanced Artist" {
		t.Errorf("Expected Artist to be 'Enhanced Artist', got '%s'", song.Artist)
	}
	if song.TrackNumber != 5 {
		t.Errorf("Expected TrackNumber to be 5, got %d", song.TrackNumber)
	}
	if song.DiscNumber != 1 {
		t.Errorf("Expected DiscNumber to be 1, got %d", song.DiscNumber)
	}
	if song.Year != 2023 {
		t.Errorf("Expected Year to be 2023, got %d", song.Year)
	}
	if song.Date != "2023-06-15" {
		t.Errorf("Expected Date to be '2023-06-15', got '%s'", song.Date)
	}
	if song.ISRC != "USRC12345678" {
		t.Errorf("Expected ISRC to be 'USRC12345678', got '%s'", song.ISRC)
	}
	if song.CoverURL != "https://example.com/cover.jpg" {
		t.Errorf("Expected CoverURL to be 'https://example.com/cover.jpg', got '%s'", song.CoverURL)
	}
	if song.SpotifyURL != "https://open.spotify.com/track/123" {
		t.Errorf("Expected SpotifyURL to be 'https://open.spotify.com/track/123', got '%s'", song.SpotifyURL)
	}
	if !song.Explicit {
		t.Error("Expected Explicit to be true")
	}
	if song.TracksCount != 12 {
		t.Errorf("Expected TracksCount to be 12, got %d", song.TracksCount)
	}
}

func TestApplySpotifyEnhancement_NoEnhancement(t *testing.T) {
	song := &metadata.Song{
		Title:    "Test Song",
		Artist:   "YouTube Artist",
		Album:    "YouTube",
		Duration: 200,
	}

	item := &plan.PlanItem{
		Metadata: make(map[string]interface{}), // No enhancement
	}

	applySpotifyEnhancement(song, item)

	// Song should remain unchanged
	if song.Album != "YouTube" {
		t.Errorf("Expected Album to remain 'YouTube', got '%s'", song.Album)
	}
	if song.Artist != "YouTube Artist" {
		t.Errorf("Expected Artist to remain 'YouTube Artist', got '%s'", song.Artist)
	}
}

// Note: Unit tests for downloadYouTubeTrack are limited because Downloader uses concrete types.
// The helper functions (youtubeMetadataToSong, applySpotifyEnhancement) are already tested above.
// Full download flow tests are integration tests in youtube_integration_test.go.
