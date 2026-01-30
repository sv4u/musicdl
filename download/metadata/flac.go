package metadata

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// embedFLAC embeds metadata in FLAC file using mutagen subprocess.
func (e *Embedder) embedFLAC(ctx context.Context, filePath string, song *Song, coverURL string) error {
	// Use mutagen subprocess for FLAC metadata embedding
	// Mutagen supports FLAC natively via Vorbis comments

	// Download cover art if provided
	var coverPath string
	if coverURL != "" {
		var err error
		coverPath, err = e.downloadCoverArt(ctx, coverURL)
		if err != nil {
			log.Printf("WARN: metadata_embed_cover_failed file=%s cover_url=%s error=%v", filePath, coverURL, err)
			// Continue without cover art
		}
		defer func() {
			if coverPath != "" {
				_ = os.Remove(coverPath)
			}
		}()
	}

	// Use mutagen to set metadata
	// mutagen-inspect can read, but we need to use Python script with mutagen library
	// Create a temporary Python script to set metadata
	return e.embedFLACWithMutagen(ctx, filePath, song, coverPath)
}

// embedFLACWithMutagen embeds metadata in FLAC file using mutagen Python library via subprocess.
func (e *Embedder) embedFLACWithMutagen(ctx context.Context, filePath string, song *Song, coverPath string) error {
	// Create temporary Python script
	tmpDir := filepath.Dir(filePath)
	tmpScript := filepath.Join(tmpDir, fmt.Sprintf(".flac_metadata_%d.py", time.Now().UnixNano()))
	defer func() { _ = os.Remove(tmpScript) }()

	// Generate Python script content
	script := fmt.Sprintf(`#!/usr/bin/env python3
import sys
from mutagen.flac import FLAC

try:
    audio = FLAC('%s')
    
    # Clear existing tags
    audio.clear()
    
    # Set basic tags
    audio['TITLE'] = [%q]
    audio['ARTIST'] = [%q]
`, filePath, song.Title, song.Artist)

	if song.Album != "" {
		script += fmt.Sprintf("    audio['ALBUM'] = [%q]\n", song.Album)
	}
	if song.AlbumArtist != "" {
		script += fmt.Sprintf("    audio['ALBUMARTIST'] = [%q]\n", song.AlbumArtist)
	}
	if song.TrackNumber > 0 {
		if song.TracksCount > 0 {
			script += fmt.Sprintf("    audio['TRACKNUMBER'] = [%q]\n", fmt.Sprintf("%d/%d", song.TrackNumber, song.TracksCount))
		} else {
			script += fmt.Sprintf("    audio['TRACKNUMBER'] = [%q]\n", fmt.Sprintf("%d", song.TrackNumber))
		}
	}
	if song.Date != "" {
		script += fmt.Sprintf("    audio['DATE'] = [%q]\n", song.Date)
	} else if song.Year > 0 {
		script += fmt.Sprintf("    audio['DATE'] = [%q]\n", fmt.Sprintf("%d", song.Year))
	}
	if song.Genre != "" {
		script += fmt.Sprintf("    audio['GENRE'] = [%q]\n", song.Genre)
	}
	if song.SpotifyURL != "" {
		script += fmt.Sprintf("    audio['COMMENT'] = [%q]\n", fmt.Sprintf("Spotify: %s", song.SpotifyURL))
	}

	// Add cover art if provided
	if coverPath != "" {
		script += fmt.Sprintf(`
    # Add cover art
    with open(%q, 'rb') as f:
        audio['PICTURE'] = f.read()
`, coverPath)
	}

	script += `
    audio.save()
    sys.exit(0)
except Exception as e:
    print(f"Error: {e}", file=sys.stderr)
    sys.exit(1)
`

	// Write script to file
	if err := os.WriteFile(tmpScript, []byte(script), 0755); err != nil {
		return &MetadataError{
			Message:  fmt.Sprintf("Failed to create mutagen script: %s", filePath),
			Original: err,
		}
	}

	// Execute Python script with context support
	cmd := exec.CommandContext(ctx, "python3", tmpScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check if context was cancelled
		if ctx.Err() != nil {
			return &MetadataError{
				Message:  fmt.Sprintf("Context cancelled during FLAC metadata embedding: %v", ctx.Err()),
				Original: ctx.Err(),
			}
		}
		return &MetadataError{
			Message:  fmt.Sprintf("Failed to embed FLAC metadata: %s", string(output)),
			Original: err,
		}
	}

	return nil
}

// downloadCoverArt downloads cover art from URL to a temporary file.
func (e *Embedder) downloadCoverArt(ctx context.Context, coverURL string) (string, error) {
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, "GET", coverURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download cover art: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download cover art: status %d", resp.StatusCode)
	}

	// Create temporary file
	tmpFile, err := os.CreateTemp("", "cover_art_*.jpg")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}

	// Copy data
	if _, err := io.Copy(tmpFile, resp.Body); err != nil {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write cover art: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to close temp file: %w", err)
	}

	return tmpFile.Name(), nil
}
