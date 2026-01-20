package metadata

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/bogem/id3v2/v2"
)

// embedMP3 embeds metadata in MP3 file.
func (e *Embedder) embedMP3(ctx context.Context, filePath string, song *Song, coverURL string) error {
	// Check context cancellation
	if err := ctx.Err(); err != nil {
		return &MetadataError{
			Message:  fmt.Sprintf("Context cancelled: %v", err),
			Original: err,
		}
	}

	// Open or create ID3 tag
	tag, err := id3v2.Open(filePath, id3v2.Options{Parse: true})
	if err != nil {
		// If file doesn't have ID3 tag, create new one
		tag, err = id3v2.Open(filePath, id3v2.Options{Parse: false})
		if err != nil {
			return &MetadataError{
				Message:  fmt.Sprintf("Failed to open MP3 file: %s", filePath),
				Original: err,
			}
		}
	}
	defer tag.Close()

	// Set encoding to UTF-8
	tag.SetDefaultEncoding(id3v2.EncodingUTF8)

	// Basic tags
	tag.SetTitle(song.Title)
	tag.SetArtist(song.Artist)
	if song.Album != "" {
		tag.SetAlbum(song.Album)
	}
	if song.AlbumArtist != "" {
		tag.AddTextFrame(tag.CommonID("TPE2"), id3v2.EncodingUTF8, song.AlbumArtist)
	}

	// Track number
	if song.TrackNumber > 0 {
		trackStr := fmt.Sprintf("%d", song.TrackNumber)
		if song.TracksCount > 0 {
			trackStr = fmt.Sprintf("%d/%d", song.TrackNumber, song.TracksCount)
		}
		tag.AddTextFrame(tag.CommonID("TRCK"), id3v2.EncodingUTF8, trackStr)
	}

	// Date/Year
	if song.Date != "" {
		tag.AddTextFrame(tag.CommonID("TDRC"), id3v2.EncodingUTF8, song.Date)
	} else if song.Year > 0 {
		tag.AddTextFrame(tag.CommonID("TYER"), id3v2.EncodingUTF8, fmt.Sprintf("%d", song.Year))
	}

	// Spotify URL
	if song.SpotifyURL != "" {
		tag.AddTextFrame(tag.CommonID("WOAS"), id3v2.EncodingUTF8, song.SpotifyURL)
	}

	// Genre
	if song.Genre != "" {
		tag.SetGenre(song.Genre)
	}

	// Cover art
	if coverURL != "" {
		if err := e.embedCoverMP3(ctx, tag, coverURL); err != nil {
			log.Printf("WARN: cover_art_download_failed file=%s cover_url=%s error=%v", filePath, coverURL, err)
		}
	}

	// Save
	if err := tag.Save(); err != nil {
		log.Printf("ERROR: metadata_save_failed file=%s error=%v", filePath, err)
		return &MetadataError{
			Message:  "Failed to save MP3 metadata",
			Original: err,
		}
	}

	return nil
}

// embedCoverMP3 embeds cover art in MP3 file.
func (e *Embedder) embedCoverMP3(ctx context.Context, tag *id3v2.Tag, coverURL string) error {
	// Download cover art
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	req, err := http.NewRequestWithContext(ctx, "GET", coverURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download cover art: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download cover art: status %d", resp.StatusCode)
	}

	coverData, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read cover art: %w", err)
	}

	// Determine MIME type
	mimeType := "image/jpeg"
	if len(coverData) > 4 {
		// Check for PNG signature
		if coverData[0] == 0x89 && coverData[1] == 0x50 && coverData[2] == 0x4E && coverData[3] == 0x47 {
			mimeType = "image/png"
		}
	}

	// Remove existing cover art
	tag.DeleteFrames(tag.CommonID("APIC"))

	// Add new cover art
	pic := id3v2.PictureFrame{
		Encoding:    id3v2.EncodingUTF8,
		MimeType:    mimeType,
		PictureType: id3v2.PTFrontCover,
		Description: "Cover",
		Picture:     coverData,
	}
	tag.AddAttachedPicture(pic)

	return nil
}
