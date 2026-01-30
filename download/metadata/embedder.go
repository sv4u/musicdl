package metadata

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// Embedder embeds metadata into audio files.
type Embedder struct{}

// NewEmbedder creates a new metadata embedder.
func NewEmbedder() *Embedder {
	return &Embedder{}
}

// Embed embeds metadata into an audio file.
func (e *Embedder) Embed(ctx context.Context, filePath string, song *Song, coverURL string) error {
	log.Printf("INFO: metadata_embed_start file=%s track=%s artist=%s", filePath, song.Title, song.Artist)

	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return &MetadataError{
			Message:  fmt.Sprintf("Context cancelled: %v", err),
			Original: err,
		}
	}

	// Use coverURL from parameter, fallback to song.CoverURL
	if coverURL == "" {
		coverURL = song.CoverURL
	}

	// Check if file exists
	if _, err := os.Stat(filePath); err != nil {
		log.Printf("ERROR: metadata_embed_failed file=%s error=file_not_found: %v", filePath, err)
		return &MetadataError{
			Message:  fmt.Sprintf("File not found: %s", filePath),
			Original: err,
		}
	}

	// Get file extension
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))

	// Embed based on format
	var err error
	switch ext {
	case "mp3":
		err = e.embedMP3(ctx, filePath, song, coverURL)
	case "flac":
		err = e.embedFLAC(ctx, filePath, song, coverURL)
	case "m4a":
		err = e.embedM4A(ctx, filePath, song, coverURL)
	case "ogg", "opus":
		err = e.embedVorbis(ctx, filePath, song, coverURL)
	default:
		// Unsupported format - log warning but don't error
		log.Printf("WARN: metadata_embed_unsupported_format file=%s format=%s", filePath, ext)
		return nil
	}

	if err != nil {
		log.Printf("ERROR: metadata_embed_failed file=%s track=%s error=%v", filePath, song.Title, err)
		return err
	}

	log.Printf("INFO: metadata_embed_complete file=%s track=%s artist=%s", filePath, song.Title, song.Artist)
	return nil
}
