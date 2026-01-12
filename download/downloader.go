package download

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/spotify"
	"github.com/sv4u/spotigo"
)

// Downloader downloads tracks using Spotify, audio provider, and metadata embedder.
type Downloader struct {
	config         *config.DownloadSettings
	spotifyClient  *spotify.SpotifyClient
	audioProvider  *audio.Provider
	metadataEmbedder *metadata.Embedder
	fileExistenceCache map[string]bool
	cacheMu        sync.RWMutex
}

// NewDownloader creates a new downloader.
func NewDownloader(cfg *config.DownloadSettings, spotifyClient *spotify.SpotifyClient, audioProvider *audio.Provider, metadataEmbedder *metadata.Embedder) *Downloader {
	return &Downloader{
		config:            cfg,
		spotifyClient:     spotifyClient,
		audioProvider:     audioProvider,
		metadataEmbedder:  metadataEmbedder,
		fileExistenceCache: make(map[string]bool),
	}
}

// DownloadTrack downloads a single track.
// Implements the plan.Downloader interface.
func (d *Downloader) DownloadTrack(ctx context.Context, spotifyURL string) (bool, string, error) {
	maxRetries := d.config.MaxRetries
	if maxRetries == 0 {
		maxRetries = 3
	}

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return false, "", err
		}

		success, filePath, err := d.downloadTrackAttempt(ctx, spotifyURL)
		if err == nil && success {
			return true, filePath, nil
		}

		lastErr = err
		if attempt < maxRetries {
			waitTime := time.Duration(1<<uint(attempt)) * time.Second
			log.Printf("INFO: retry attempt=%d max_retries=%d spotify_url=%s error=%v wait_seconds=%d", attempt, maxRetries, spotifyURL, err, int(waitTime.Seconds()))
			time.Sleep(waitTime)
		}
	}

	log.Printf("ERROR: download_failed spotify_url=%s attempts=%d error=%v", spotifyURL, maxRetries, lastErr)
	return false, "", fmt.Errorf("failed to download %s after %d attempts: %w", spotifyURL, maxRetries, lastErr)
}

// downloadTrackAttempt performs a single download attempt.
func (d *Downloader) downloadTrackAttempt(ctx context.Context, spotifyURL string) (bool, string, error) {
	// 1. Get metadata from Spotify
	track, err := d.spotifyClient.GetTrack(ctx, spotifyURL)
	if err != nil {
		return false, "", fmt.Errorf("failed to get track metadata: %w", err)
	}

	// Get album metadata
	albumID := ""
	if track.Album != nil {
		albumID = track.Album.ID
	}
	if albumID == "" {
		return false, "", fmt.Errorf("track has no album ID")
	}

	album, err := d.spotifyClient.GetAlbum(ctx, albumID)
	if err != nil {
		return false, "", fmt.Errorf("failed to get album metadata: %w", err)
	}

	// Convert to Song model
	song := spotifyTrackToSong(track, album)

	// Log download start
	log.Printf("INFO: download_start spotify_id=%s track=%s artist=%s album=%s", track.ID, song.Title, song.Artist, song.Album)

	// 2. Check if file already exists
	outputPath := d.getOutputPath(song)
	fileExists := d.fileExistsCached(outputPath)

	if fileExists && d.config.Overwrite == config.OverwriteSkip {
		// File exists and we should skip
		log.Printf("INFO: download_skipped reason=file_exists spotify_id=%s track=%s path=%s", track.ID, song.Title, outputPath)
		return true, outputPath, nil
	}

	if fileExists && d.config.Overwrite == config.OverwriteMetadata {
		// File exists, update metadata only
		log.Printf("INFO: metadata_update_start spotify_id=%s track=%s path=%s", track.ID, song.Title, outputPath)
		if err := d.metadataEmbedder.Embed(outputPath, song, song.CoverURL); err != nil {
			log.Printf("ERROR: metadata_update_failed spotify_id=%s track=%s path=%s error=%v", track.ID, song.Title, outputPath, err)
			return false, "", fmt.Errorf("failed to update metadata: %w", err)
		}
		log.Printf("INFO: metadata_update_complete spotify_id=%s track=%s path=%s", track.ID, song.Title, outputPath)
		return true, outputPath, nil
	}

	// 3. Search for audio using audio provider
	searchQuery := fmt.Sprintf("%s - %s", song.Artist, song.Title)
	audioURL, err := d.audioProvider.Search(ctx, searchQuery)
	if err != nil {
		return false, "", fmt.Errorf("failed to search for audio: %w", err)
	}
	if audioURL == "" {
		return false, "", fmt.Errorf("no audio found for: %s", searchQuery)
	}

	// 4. Download audio file
	downloadedPath, err := d.audioProvider.Download(ctx, audioURL, outputPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to download audio: %w", err)
	}

	// 5. Embed metadata
	if err := d.metadataEmbedder.Embed(downloadedPath, song, song.CoverURL); err != nil {
		// Log warning but don't fail - file is downloaded
		log.Printf("WARN: metadata_embed_failed spotify_id=%s track=%s path=%s error=%v", track.ID, song.Title, downloadedPath, err)
	} else {
		log.Printf("INFO: metadata_embed_complete spotify_id=%s track=%s path=%s", track.ID, song.Title, downloadedPath)
	}

	// Verify file exists
	if _, err := os.Stat(downloadedPath); err != nil {
		d.setFileExistsCached(downloadedPath, false)
		return false, "", fmt.Errorf("file not found after download: %w", err)
	}

	// Invalidate cache entry (file now exists)
	d.invalidateFileCache(downloadedPath)

	log.Printf("INFO: download_complete spotify_id=%s track=%s artist=%s path=%s", track.ID, song.Title, song.Artist, downloadedPath)
	return true, downloadedPath, nil
}

// spotifyTrackToSong converts Spotify track and album data to Song model.
func spotifyTrackToSong(track *spotigo.Track, album *spotigo.Album) *metadata.Song {
	song := &metadata.Song{
		Title:       track.Name,
		Artist:      "",
		Album:       "",
		TrackNumber: track.TrackNumber,
		Duration:    track.DurationMs / 1000, // Convert ms to seconds
		SpotifyURL:  "",
		DiscNumber:  track.DiscNumber,
		Explicit:    track.Explicit,
	}

	// Get artist name
	if len(track.Artists) > 0 {
		song.Artist = track.Artists[0].Name
	}

	// Get album info
	if album != nil {
		song.Album = album.Name
		if len(album.Artists) > 0 {
			song.AlbumArtist = album.Artists[0].Name
		}
		song.Year = extractYear(album.ReleaseDate)
		song.Date = album.ReleaseDate
	} else if track.Album != nil {
		song.Album = track.Album.Name
		if len(track.Album.Artists) > 0 {
			song.AlbumArtist = track.Album.Artists[0].Name
		}
		song.Year = extractYear(track.Album.ReleaseDate)
		song.Date = track.Album.ReleaseDate
	}

	// Get Spotify URL
	if track.ExternalURLs != nil {
		song.SpotifyURL = track.ExternalURLs.Spotify
	}

	// Get cover art URL
	if album != nil && len(album.Images) > 0 {
		// Use the largest image (usually first)
		song.CoverURL = album.Images[0].URL
	} else if track.Album != nil && len(track.Album.Images) > 0 {
		song.CoverURL = track.Album.Images[0].URL
	}

	// Get ISRC
	if track.ExternalIDs != nil && track.ExternalIDs.ISRC != nil && *track.ExternalIDs.ISRC != "" {
		song.ISRC = *track.ExternalIDs.ISRC
	}

	// Get total tracks from album
	if album != nil {
		song.TracksCount = album.TotalTracks
	} else if track.Album != nil {
		song.TracksCount = track.Album.TotalTracks
	}

	return song
}

// extractYear extracts year from release date string.
func extractYear(releaseDate string) int {
	if releaseDate == "" {
		return 0
	}
	// Try to parse YYYY-MM-DD or YYYY-MM or YYYY
	parts := strings.Split(releaseDate, "-")
	if len(parts) > 0 {
		var year int
		if _, err := fmt.Sscanf(parts[0], "%d", &year); err == nil {
			return year
		}
	}
	return 0
}

// getOutputPath generates output file path from song metadata and config template.
func (d *Downloader) getOutputPath(song *metadata.Song) string {
	template := d.config.Output
	if template == "" {
		template = "{artist}/{album}/{track-number} - {title}.{output-ext}"
	}

	// Replace template variables
	output := template
	output = strings.ReplaceAll(output, "{artist}", sanitizeFilename(song.Artist))
	output = strings.ReplaceAll(output, "{album}", sanitizeFilename(song.Album))
	output = strings.ReplaceAll(output, "{title}", sanitizeFilename(song.Title))
	output = strings.ReplaceAll(output, "{track-number}", fmt.Sprintf("%02d", song.TrackNumber))
	output = strings.ReplaceAll(output, "{output-ext}", d.config.Format)

	// Ensure directory exists
	dir := filepath.Dir(output)
	if err := os.MkdirAll(dir, 0755); err != nil {
		// If we can't create directory, use current directory
		output = filepath.Base(output)
	}

	return output
}

// sanitizeFilename sanitizes a filename by removing invalid characters.
func sanitizeFilename(name string) string {
	invalidChars := []rune{'/', '\\', ':', '*', '?', '"', '<', '>', '|'}
	result := []rune(name)
	for i, r := range result {
		for _, invalid := range invalidChars {
			if r == invalid {
				result[i] = '_'
				break
			}
		}
	}
	return string(result)
}

// fileExistsCached checks if file exists using cache.
func (d *Downloader) fileExistsCached(filePath string) bool {
	d.cacheMu.RLock()
	exists, ok := d.fileExistenceCache[filePath]
	d.cacheMu.RUnlock()

	if ok {
		return exists
	}

	// Check file system
	exists = false
	if _, err := os.Stat(filePath); err == nil {
		exists = true
	}

	// Cache result
	d.setFileExistsCached(filePath, exists)
	return exists
}

// setFileExistsCached sets file existence in cache.
func (d *Downloader) setFileExistsCached(filePath string, exists bool) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	d.fileExistenceCache[filePath] = exists
}

// invalidateFileCache invalidates file existence cache entry.
func (d *Downloader) invalidateFileCache(filePath string) {
	d.cacheMu.Lock()
	defer d.cacheMu.Unlock()
	delete(d.fileExistenceCache, filePath)
}

// GetFileExistenceCacheStats returns statistics for the file existence cache.
func (d *Downloader) GetFileExistenceCacheStats() map[string]interface{} {
	d.cacheMu.RLock()
	defer d.cacheMu.RUnlock()
	
	size := len(d.fileExistenceCache)
	maxSize := d.config.FileExistenceCacheMaxSize
	if maxSize == 0 {
		maxSize = 10000 // Default
	}
	
	return map[string]interface{}{
		"size":    size,
		"max_size": maxSize,
	}
}
