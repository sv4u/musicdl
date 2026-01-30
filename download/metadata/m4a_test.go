//go:build integration

package metadata

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestEmbedder_EmbedM4A_Integration(t *testing.T) {
	// This test requires an actual M4A file and mutagen
	// Skip if mutagen is not available
	if err := checkMutagenAvailable(); err != nil {
		t.Skipf("Mutagen not available: %v", err)
	}

	testM4A := os.Getenv("TEST_M4A_FILE")
	if testM4A == "" {
		t.Skip("TEST_M4A_FILE environment variable not set, skipping integration test")
	}

	if _, err := os.Stat(testM4A); err != nil {
		t.Skipf("Test M4A file not found: %s", testM4A)
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
	testFile := filepath.Join(tmpDir, "test_metadata.m4a")

	// Copy test file
	data, err := os.ReadFile(testM4A)
	if err != nil {
		t.Fatalf("Failed to read test M4A: %v", err)
	}
	if err := os.WriteFile(testFile, data, 0644); err != nil {
		t.Fatalf("Failed to write test M4A: %v", err)
	}

	// Embed metadata
	err = embedder.Embed(context.Background(), testFile, song, "")
	if err != nil {
		t.Fatalf("Failed to embed metadata: %v", err)
	}

	// Verify file still exists
	if _, err := os.Stat(testFile); err != nil {
		t.Fatalf("File not found after embedding: %v", err)
	}

	t.Logf("Successfully embedded metadata in %s", testFile)
}
