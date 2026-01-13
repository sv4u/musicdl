//go:build integration

package metadata

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEmbedder_EmbedVorbis_Integration(t *testing.T) {
	// This test requires an actual OGG/Opus file and mutagen
	// Skip if mutagen is not available
	if err := checkMutagenAvailable(); err != nil {
		t.Skipf("Mutagen not available: %v", err)
	}

	testOGG := os.Getenv("TEST_OGG_FILE")
	if testOGG == "" {
		t.Skip("TEST_OGG_FILE environment variable not set, skipping integration test")
	}

	if _, err := os.Stat(testOGG); err != nil {
		t.Skipf("Test OGG file not found: %s", testOGG)
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
	testFile := filepath.Join(tmpDir, "test_metadata.ogg")
	
	// Copy test file
	data, err := os.ReadFile(testOGG)
	if err != nil {
		t.Fatalf("Failed to read test OGG: %v", err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("Failed to write test OGG: %v", err)
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
