//go:build integration

package metadata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbedder_EmbedMP3_Integration(t *testing.T) {
	// This test requires an actual MP3 file
	// For now, we'll skip if no test file is available
	testMP3 := os.Getenv("TEST_MP3_FILE")
	if testMP3 == "" {
		t.Skip("TEST_MP3_FILE environment variable not set, skipping integration test")
	}

	if _, err := os.Stat(testMP3); err != nil {
		t.Skipf("Test MP3 file not found: %s", testMP3)
	}

	embedder := NewEmbedder()
	song := &Song{
		Title:       "Test Song",
		Artist:      "Test Artist",
		Album:       "Test Album",
		AlbumArtist: "Test Album Artist",
		TrackNumber: 1,
		TracksCount: 10,
		Year:        2024,
		SpotifyURL:  "https://open.spotify.com/track/test",
		Genre:       "Rock",
		CoverURL:    "", // No cover for this test
	}

	// Create a copy of the test file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test_metadata.mp3")
	
	// Copy test file
	data, err := os.ReadFile(testMP3)
	if err != nil {
		t.Fatalf("Failed to read test MP3: %v", err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("Failed to write test MP3: %v", err)
	}

	// Embed metadata
	err = embedder.Embed(testFile, song, "")
	if err != nil {
		t.Fatalf("Failed to embed metadata: %v", err)
	}

	// Verify file still exists
	if _, err := os.Stat(testFile); err != nil {
		t.Fatalf("File not found after embedding: %v", err)
	}

	t.Logf("Successfully embedded metadata in %s", testFile)
}
