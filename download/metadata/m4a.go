package metadata

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// embedM4A embeds metadata in M4A file using mutagen subprocess.
func (e *Embedder) embedM4A(ctx context.Context, filePath string, song *Song, coverURL string) error {
	// Use mutagen subprocess for M4A metadata embedding
	// Mutagen supports M4A/MP4 natively via MP4 tags
	
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
				os.Remove(coverPath)
			}
		}()
	}
	
	return e.embedM4AWithMutagen(ctx, filePath, song, coverPath)
}

// embedM4AWithMutagen embeds metadata in M4A file using mutagen Python library via subprocess.
func (e *Embedder) embedM4AWithMutagen(ctx context.Context, filePath string, song *Song, coverPath string) error {
	// Create temporary Python script
	tmpDir := filepath.Dir(filePath)
	tmpScript := filepath.Join(tmpDir, fmt.Sprintf(".m4a_metadata_%d.py", time.Now().UnixNano()))
	defer os.Remove(tmpScript)
	
	// Generate Python script content
	script := fmt.Sprintf(`#!/usr/bin/env python3
import sys
from mutagen.mp4 import MP4

try:
    audio = MP4('%s')
    
    # Clear existing tags
    audio.clear()
    
    # Set basic tags
    audio['\xa9nam'] = [%q]  # Title
    audio['\xa9ART'] = [%q]  # Artist
`, filePath, song.Title, song.Artist)
	
	if song.Album != "" {
		script += fmt.Sprintf("    audio['\\xa9alb'] = [%q]  # Album\n", song.Album)
	}
	if song.AlbumArtist != "" {
		script += fmt.Sprintf("    audio['aART'] = [%q]  # Album Artist\n", song.AlbumArtist)
	}
	if song.TrackNumber > 0 {
		if song.TracksCount > 0 {
			script += fmt.Sprintf("    audio['trkn'] = [(%d, %d)]  # Track number\n", song.TrackNumber, song.TracksCount)
		} else {
			script += fmt.Sprintf("    audio['trkn'] = [(%d, 0)]  # Track number\n", song.TrackNumber)
		}
	}
	if song.DiscNumber > 0 {
		if song.DiscCount > 0 {
			script += fmt.Sprintf("    audio['disk'] = [(%d, %d)]  # Disc number\n", song.DiscNumber, song.DiscCount)
		} else {
			script += fmt.Sprintf("    audio['disk'] = [(%d, 0)]  # Disc number\n", song.DiscNumber)
		}
	}
	if song.Date != "" {
		script += fmt.Sprintf("    audio['\\xa9day'] = [%q]  # Date\n", song.Date)
	} else if song.Year > 0 {
		script += fmt.Sprintf("    audio['\\xa9day'] = [%q]  # Date\n", fmt.Sprintf("%d", song.Year))
	}
	if song.Genre != "" {
		script += fmt.Sprintf("    audio['\\xa9gen'] = [%q]  # Genre\n", song.Genre)
	}
	if song.SpotifyURL != "" {
		script += fmt.Sprintf("    audio['\\xa9cmt'] = [%q]  # Comment\n", fmt.Sprintf("Spotify: %s", song.SpotifyURL))
	}
	
	// Add cover art if provided
	if coverPath != "" {
		script += fmt.Sprintf(`
    # Add cover art
    with open(%q, 'rb') as f:
        cover_data = f.read()
        audio['covr'] = [cover_data]
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
				Message:  fmt.Sprintf("Context cancelled during M4A metadata embedding: %v", ctx.Err()),
				Original: ctx.Err(),
			}
		}
		return &MetadataError{
			Message:  fmt.Sprintf("Failed to embed M4A metadata: %s", string(output)),
			Original: err,
		}
	}
	
	return nil
}
