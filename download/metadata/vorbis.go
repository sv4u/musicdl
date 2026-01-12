package metadata

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// embedVorbis embeds metadata in OGG/Opus files using mutagen subprocess.
// OGG/Opus use Vorbis comments similar to FLAC.
func (e *Embedder) embedVorbis(filePath string, song *Song, coverURL string) error {
	// Use mutagen subprocess for OGG/Opus metadata embedding
	// Mutagen supports OGG/Opus natively via Vorbis comments
	
	// Download cover art if provided
	var coverPath string
	if coverURL != "" {
		var err error
		coverPath, err = e.downloadCoverArt(coverURL)
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
	
	return e.embedVorbisWithMutagen(filePath, song, coverPath)
}

// embedVorbisWithMutagen embeds metadata in OGG/Opus file using mutagen Python library via subprocess.
func (e *Embedder) embedVorbisWithMutagen(filePath string, song *Song, coverPath string) error {
	// Create temporary Python script
	tmpDir := filepath.Dir(filePath)
	tmpScript := filepath.Join(tmpDir, fmt.Sprintf(".vorbis_metadata_%d.py", time.Now().UnixNano()))
	defer os.Remove(tmpScript)
	
	// Generate Python script content
	script := fmt.Sprintf(`#!/usr/bin/env python3
import sys
from mutagen.oggvorbis import OggVorbis
from mutagen.oggopus import OggOpus

try:
    # Try OggVorbis first (for .ogg files)
    try:
        audio = OggVorbis('%s')
    except:
        # Try OggOpus (for .opus files)
        audio = OggOpus('%s')
    
    # Clear existing tags
    audio.clear()
    
    # Set basic tags
    audio['TITLE'] = [%q]
    audio['ARTIST'] = [%q]
`, filePath, filePath, song.Title, song.Artist)
	
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
        audio['METADATA_BLOCK_PICTURE'] = f.read()
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
	
	// Execute Python script
	cmd := exec.Command("python3", tmpScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return &MetadataError{
			Message:  fmt.Sprintf("Failed to embed OGG/Opus metadata: %s", string(output)),
			Original: err,
		}
	}
	
	return nil
}
