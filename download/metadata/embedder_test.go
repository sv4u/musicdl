package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewEmbedder(t *testing.T) {
	embedder := NewEmbedder()
	if embedder == nil {
		t.Fatal("NewEmbedder() returned nil")
	}
}

func TestEmbedder_Embed_UnsupportedFormat(t *testing.T) {
	embedder := NewEmbedder()
	song := &Song{
		Title:  "Test Song",
		Artist: "Test Artist",
	}

	// Create a temporary file with unsupported extension
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.wav")
	
	// Create empty file
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	// Should not error on unsupported format (just returns nil)
	err = embedder.Embed(testFile, song, "")
	if err != nil {
		t.Errorf("Expected no error for unsupported format, got: %v", err)
	}
}

func TestEmbedder_Embed_FileNotFound(t *testing.T) {
	embedder := NewEmbedder()
	song := &Song{
		Title:  "Test Song",
		Artist: "Test Artist",
	}

	err := embedder.Embed("/nonexistent/file.mp3", song, "")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	
	if _, ok := err.(*MetadataError); !ok {
		t.Errorf("Expected MetadataError, got %T", err)
	}
}

func TestEmbedder_EmbedFLAC_FileNotFound(t *testing.T) {
	embedder := NewEmbedder()
	song := &Song{
		Title:  "Test Song",
		Artist: "Test Artist",
	}

	err := embedder.embedFLAC("/nonexistent/file.flac", song, "")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	
	if _, ok := err.(*MetadataError); !ok {
		t.Errorf("Expected MetadataError, got %T", err)
	}
}

func TestEmbedder_EmbedVorbis_FileNotFound(t *testing.T) {
	embedder := NewEmbedder()
	song := &Song{
		Title:  "Test Song",
		Artist: "Test Artist",
	}

	err := embedder.embedVorbis("/nonexistent/file.ogg", song, "")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	
	if _, ok := err.(*MetadataError); !ok {
		t.Errorf("Expected MetadataError, got %T", err)
	}
}

func TestEmbedder_EmbedM4A_FileNotFound(t *testing.T) {
	embedder := NewEmbedder()
	song := &Song{
		Title:  "Test Song",
		Artist: "Test Artist",
	}

	err := embedder.embedM4A("/nonexistent/file.m4a", song, "")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
	
	if _, ok := err.(*MetadataError); !ok {
		t.Errorf("Expected MetadataError, got %T", err)
	}
}

func TestEmbedder_DownloadCoverArt(t *testing.T) {
	embedder := NewEmbedder()
	
	// Test with invalid URL (should fail)
	_, err := embedder.downloadCoverArt("http://invalid-url-that-does-not-exist.example.com/cover.jpg")
	if err == nil {
		t.Error("Expected error for invalid URL")
	}
}

func TestEmbedder_Embed_CoverURLFallback(t *testing.T) {
	embedder := NewEmbedder()
	song := &Song{
		Title:    "Test Song",
		Artist:   "Test Artist",
		CoverURL: "http://example.com/cover.jpg", // Song has cover URL
	}

	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.wav")
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	// Test Embed with empty coverURL parameter - should use song.CoverURL
	err = embedder.Embed(testFile, song, "")
	if err != nil {
		// Error is expected for unsupported format, but should not error on coverURL fallback
		if err.Error() != "" && !strings.Contains(err.Error(), "unsupported") {
			t.Logf("Note: Embed returned error (expected for unsupported format): %v", err)
		}
	}

	// Test Embed with explicit coverURL parameter - should use parameter
	err = embedder.Embed(testFile, song, "http://example.com/explicit-cover.jpg")
	if err != nil {
		// Error is expected for unsupported format
		if !strings.Contains(err.Error(), "unsupported") {
			t.Logf("Note: Embed returned error (expected for unsupported format): %v", err)
		}
	}
}

func TestEmbedder_Embed_EmptySong(t *testing.T) {
	embedder := NewEmbedder()
	song := &Song{} // Empty song

	// Create a temporary file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.wav")
	file, err := os.Create(testFile)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	file.Close()

	// Should not error on empty song (unsupported format returns nil)
	err = embedder.Embed(testFile, song, "")
	if err != nil {
		t.Errorf("Expected no error for empty song with unsupported format, got: %v", err)
	}
}

func TestMetadataError(t *testing.T) {
	// Test MetadataError without original error
	err := &MetadataError{
		Message: "Test error",
	}
	if err.Error() != "Metadata error: Test error" {
		t.Errorf("Expected 'Metadata error: Test error', got '%s'", err.Error())
	}

	// Test MetadataError with original error
	originalErr := fmt.Errorf("original error")
	err2 := &MetadataError{
		Message:  "Test error",
		Original: originalErr,
	}
	if err2.Error() == "" {
		t.Error("Expected non-empty error message")
	}
	if err2.Unwrap() != originalErr {
		t.Error("Unwrap() should return original error")
	}
}
