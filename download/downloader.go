package download

import (
	"context"
	"errors"
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
	"github.com/sv4u/musicdl/download/plan"
	"github.com/sv4u/musicdl/download/spotify"
	"github.com/sv4u/spotigo"
)

// Downloader downloads tracks using Spotify, audio provider, and metadata embedder.
type Downloader struct {
	config             *config.DownloadSettings
	spotifyClient      *spotify.SpotifyClient
	audioProvider      *audio.Provider
	metadataEmbedder   *metadata.Embedder
	fileExistenceCache map[string]bool
	cacheMu            sync.RWMutex
}

// NewDownloader creates a new downloader.
func NewDownloader(cfg *config.DownloadSettings, spotifyClient *spotify.SpotifyClient, audioProvider *audio.Provider, metadataEmbedder *metadata.Embedder) *Downloader {
	return &Downloader{
		config:             cfg,
		spotifyClient:      spotifyClient,
		audioProvider:      audioProvider,
		metadataEmbedder:   metadataEmbedder,
		fileExistenceCache: make(map[string]bool),
	}
}

// DownloadTrack downloads a single track.
// Implements the plan.Downloader interface.
func (d *Downloader) DownloadTrack(ctx context.Context, item *plan.PlanItem) (bool, string, error) {
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

		success, filePath, err := d.downloadTrackAttempt(ctx, item)
		if err == nil && success {
			return true, filePath, nil
		}

		lastErr = err
		if attempt < maxRetries {
			waitTime := time.Duration(1<<uint(attempt)) * time.Second
			var rateLimitErr *spotify.RateLimitError
			if errors.As(err, &rateLimitErr) && rateLimitErr.RetryAfter > 0 {
				waitTime = time.Duration(rateLimitErr.RetryAfter+10) * time.Second
			}
			url := item.SpotifyURL
			if url == "" {
				url = item.YouTubeURL
			}
			log.Printf("INFO: retry attempt=%d max_retries=%d url=%s error=%v wait_seconds=%d", attempt, maxRetries, url, err, int(waitTime.Seconds()))
			time.Sleep(waitTime)
		}
	}

	url := item.SpotifyURL
	if url == "" {
		url = item.YouTubeURL
	}
	log.Printf("ERROR: download_failed url=%s attempts=%d error=%v", url, maxRetries, lastErr)
	return false, "", fmt.Errorf("failed to download %s after %d attempts: %w", url, maxRetries, lastErr)
}

// downloadTrackAttempt performs a single download attempt.
func (d *Downloader) downloadTrackAttempt(ctx context.Context, item *plan.PlanItem) (bool, string, error) {
	// Route to appropriate handler based on URL type
	if item.YouTubeURL != "" {
		return d.downloadYouTubeTrack(ctx, item)
	}
	if item.SpotifyURL != "" {
		return d.downloadSpotifyTrack(ctx, item)
	}
	return false, "", fmt.Errorf("no Spotify URL or YouTube URL provided in plan item")
}

// downloadSpotifyTrack downloads a track from Spotify.
func (d *Downloader) downloadSpotifyTrack(ctx context.Context, item *plan.PlanItem) (bool, string, error) {
	spotifyURL := item.SpotifyURL

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
		if err := d.metadataEmbedder.Embed(ctx, outputPath, song, song.CoverURL); err != nil {
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
	if err := d.metadataEmbedder.Embed(ctx, downloadedPath, song, song.CoverURL); err != nil {
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

// downloadYouTubeTrack downloads a track from YouTube.
func (d *Downloader) downloadYouTubeTrack(ctx context.Context, item *plan.PlanItem) (bool, string, error) {
	// 1. Extract YouTube metadata from PlanItem
	ytMetadata, err := extractYouTubeMetadata(item)
	if err != nil {
		// Log warning and try to fetch fresh metadata if extraction fails
		log.Printf("WARN: failed_to_extract_youtube_metadata youtube_url=%s error=%v, attempting fresh fetch", item.YouTubeURL, err)
		if d.audioProvider == nil {
			return false, "", fmt.Errorf("audioProvider is required for YouTube downloads")
		}
		ytMetadata, err = d.audioProvider.GetVideoMetadata(ctx, item.YouTubeURL)
		if err != nil {
			return false, "", fmt.Errorf("failed to get YouTube metadata: %w", err)
		}
	}

	// 2. Convert YouTube metadata to Song model
	song := youtubeMetadataToSong(ytMetadata, item)

	// 3. Apply Spotify enhancement if available
	applySpotifyEnhancement(song, item)

	// Log download start
	videoID := ytMetadata.VideoID
	if videoID == "" {
		videoID = "unknown"
	}
	log.Printf("INFO: download_start youtube_id=%s track=%s artist=%s", videoID, song.Title, song.Artist)

	// 4. Check if file already exists
	outputPath := d.getOutputPath(song)
	fileExists := d.fileExistsCached(outputPath)

	if fileExists && d.config.Overwrite == config.OverwriteSkip {
		// File exists and we should skip
		log.Printf("INFO: download_skipped reason=file_exists youtube_id=%s track=%s path=%s", videoID, song.Title, outputPath)
		return true, outputPath, nil
	}

	if fileExists && d.config.Overwrite == config.OverwriteMetadata {
		// File exists, update metadata only
		log.Printf("INFO: metadata_update_start youtube_id=%s track=%s path=%s", videoID, song.Title, outputPath)
		if err := d.metadataEmbedder.Embed(ctx, outputPath, song, song.CoverURL); err != nil {
			log.Printf("ERROR: metadata_update_failed youtube_id=%s track=%s path=%s error=%v", videoID, song.Title, outputPath, err)
			return false, "", fmt.Errorf("failed to update metadata: %w", err)
		}
		log.Printf("INFO: metadata_update_complete youtube_id=%s track=%s path=%s", videoID, song.Title, outputPath)
		return true, outputPath, nil
	}

	// 5. Download directly from YouTube URL (no search needed)
	// Check audioProvider is available before attempting download
	if d.audioProvider == nil {
		return false, "", fmt.Errorf("audioProvider is required for YouTube downloads")
	}
	downloadedPath, err := d.audioProvider.Download(ctx, item.YouTubeURL, outputPath)
	if err != nil {
		return false, "", fmt.Errorf("failed to download from YouTube: %w", err)
	}

	// 6. Embed metadata
	if err := d.metadataEmbedder.Embed(ctx, downloadedPath, song, song.CoverURL); err != nil {
		// Log warning but don't fail - file is downloaded
		log.Printf("WARN: metadata_embed_failed youtube_id=%s track=%s path=%s error=%v", videoID, song.Title, downloadedPath, err)
	} else {
		log.Printf("INFO: metadata_embed_complete youtube_id=%s track=%s path=%s", videoID, song.Title, downloadedPath)
	}

	// Verify file exists
	if _, err := os.Stat(downloadedPath); err != nil {
		d.setFileExistsCached(downloadedPath, false)
		return false, "", fmt.Errorf("file not found after download: %w", err)
	}

	// Invalidate cache entry (file now exists)
	d.invalidateFileCache(downloadedPath)

	log.Printf("INFO: download_complete youtube_id=%s track=%s artist=%s path=%s", videoID, song.Title, song.Artist, downloadedPath)
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

// extractYouTubeMetadata extracts YouTube metadata from PlanItem.Metadata.
// Returns nil if metadata is not found or cannot be extracted.
func extractYouTubeMetadata(item *plan.PlanItem) (*audio.YouTubeVideoMetadata, error) {
	if item.Metadata == nil {
		return nil, fmt.Errorf("item metadata is nil")
	}

	// Try to extract from metadata map
	ytMetaRaw, ok := item.Metadata["youtube_metadata"]
	if !ok {
		return nil, fmt.Errorf("youtube_metadata not found in item metadata")
	}

	// Type assertion - could be map[string]interface{} or already YouTubeVideoMetadata
	switch v := ytMetaRaw.(type) {
	case *audio.YouTubeVideoMetadata:
		return v, nil
	case audio.YouTubeVideoMetadata:
		return &v, nil
	case map[string]interface{}:
		// Convert map to structured type
		meta := &audio.YouTubeVideoMetadata{}

		if id, ok := v["video_id"].(string); ok {
			meta.VideoID = id
		}
		if title, ok := v["title"].(string); ok {
			meta.Title = title
		}
		if desc, ok := v["description"].(string); ok {
			meta.Description = desc
		}
		if duration, ok := v["duration"].(float64); ok {
			meta.Duration = int(duration)
		} else if duration, ok := v["duration"].(int); ok {
			meta.Duration = duration
		}
		if uploader, ok := v["uploader"].(string); ok {
			meta.Uploader = uploader
		}
		if uploadDate, ok := v["upload_date"].(string); ok {
			meta.UploadDate = uploadDate
		}
		if viewCount, ok := v["view_count"].(float64); ok {
			meta.ViewCount = int64(viewCount)
		}
		if thumbnail, ok := v["thumbnail"].(string); ok {
			meta.Thumbnail = thumbnail
		}
		if webpageURL, ok := v["webpage_url"].(string); ok {
			meta.WebpageURL = webpageURL
		}
		if categories, ok := v["categories"].([]interface{}); ok {
			meta.Categories = make([]string, 0, len(categories))
			for _, cat := range categories {
				if catStr, ok := cat.(string); ok {
					meta.Categories = append(meta.Categories, catStr)
				}
			}
		}
		if tags, ok := v["tags"].([]interface{}); ok {
			meta.Tags = make([]string, 0, len(tags))
			for _, tag := range tags {
				if tagStr, ok := tag.(string); ok {
					meta.Tags = append(meta.Tags, tagStr)
				}
			}
		}

		return meta, nil
	default:
		return nil, fmt.Errorf("youtube_metadata has unexpected type: %T", v)
	}
}

// extractSpotifyEnhancement extracts Spotify enhancement metadata from PlanItem.Metadata.
// Returns nil map if not found.
func extractSpotifyEnhancement(item *plan.PlanItem) map[string]interface{} {
	if item.Metadata == nil {
		return nil
	}

	enhancementRaw, ok := item.Metadata["spotify_enhancement"]
	if !ok {
		return nil
	}

	// Type assertion to map[string]interface{}
	if enhancement, ok := enhancementRaw.(map[string]interface{}); ok {
		return enhancement
	}

	return nil
}

// youtubeMetadataToSong converts YouTube video metadata to Song model.
func youtubeMetadataToSong(ytMetadata *audio.YouTubeVideoMetadata, item *plan.PlanItem) *metadata.Song {
	song := &metadata.Song{
		Title:    ytMetadata.Title,
		Artist:   ytMetadata.Uploader,
		Album:    "YouTube", // Default album name
		Duration: ytMetadata.Duration,
		CoverURL: ytMetadata.Thumbnail, // Set cover URL from YouTube thumbnail
	}

	// Use item name as fallback if title is empty
	if song.Title == "" {
		song.Title = item.Name
	}

	// Use metadata artist if available
	if artist, ok := item.Metadata["artist"].(string); ok && artist != "" {
		song.Artist = artist
	}

	// Extract year from upload date if available
	if ytMetadata.UploadDate != "" {
		song.Year = extractYear(ytMetadata.UploadDate)
		song.Date = ytMetadata.UploadDate
	}

	return song
}

// applySpotifyEnhancement applies Spotify enhancement metadata to a Song if available.
func applySpotifyEnhancement(song *metadata.Song, item *plan.PlanItem) {
	enhancement := extractSpotifyEnhancement(item)
	if enhancement == nil {
		return
	}

	// Apply enhancement fields (only if not already set or if enhancement provides better data)
	if album, ok := enhancement["album"].(string); ok && album != "" && song.Album == "YouTube" {
		song.Album = album
	}

	if albumArtist, ok := enhancement["album_artist"].(string); ok && albumArtist != "" {
		song.AlbumArtist = albumArtist
	}

	if artist, ok := enhancement["artist"].(string); ok && artist != "" {
		// Prefer Spotify artist if available
		song.Artist = artist
	}

	if trackNumber, ok := enhancement["track_number"].(float64); ok {
		song.TrackNumber = int(trackNumber)
	} else if trackNumber, ok := enhancement["track_number"].(int); ok {
		song.TrackNumber = trackNumber
	}

	if discNumber, ok := enhancement["disc_number"].(float64); ok {
		song.DiscNumber = int(discNumber)
	} else if discNumber, ok := enhancement["disc_number"].(int); ok {
		song.DiscNumber = discNumber
	}

	if year, ok := enhancement["year"].(float64); ok {
		song.Year = int(year)
	} else if year, ok := enhancement["year"].(int); ok {
		song.Year = year
	}

	if date, ok := enhancement["date"].(string); ok && date != "" {
		song.Date = date
	}

	if isrc, ok := enhancement["isrc"].(string); ok && isrc != "" {
		song.ISRC = isrc
	}

	if coverURL, ok := enhancement["cover_url"].(string); ok && coverURL != "" {
		song.CoverURL = coverURL
	}

	if spotifyURL, ok := enhancement["spotify_url"].(string); ok && spotifyURL != "" {
		song.SpotifyURL = spotifyURL
	}

	if explicit, ok := enhancement["explicit"].(bool); ok {
		song.Explicit = explicit
	}

	if tracksCount, ok := enhancement["tracks_count"].(float64); ok {
		song.TracksCount = int(tracksCount)
	} else if tracksCount, ok := enhancement["tracks_count"].(int); ok {
		song.TracksCount = tracksCount
	}
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
	output = strings.ReplaceAll(output, "{disc-number}", fmt.Sprintf("%02d", song.DiscNumber))
	output = strings.ReplaceAll(output, "{output-ext}", d.config.Format)

	// Clean path to prevent directory traversal attacks
	output = filepath.Clean(output)

	// Ensure directory exists
	dir := filepath.Dir(output)
	if err := os.MkdirAll(dir, 0755); err != nil {
		// If we can't create directory, use current directory
		log.Printf("WARN: failed_to_create_output_directory dir=%s error=%v, using current directory", dir, err)
		output = filepath.Base(output)
	}

	return output
}

// sanitizeFilename sanitizes a filename by removing invalid characters.
// Also removes directory traversal sequences for additional security.
func sanitizeFilename(name string) string {
	if name == "" {
		return "_"
	}

	// Limit length to prevent filesystem issues
	maxLen := 255
	if len(name) > maxLen {
		name = name[:maxLen]
	}

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

	// Remove directory traversal sequences
	sanitized := string(result)
	sanitized = strings.ReplaceAll(sanitized, "..", "_")

	// Remove leading/trailing dots and spaces (Windows issue)
	sanitized = strings.Trim(sanitized, ". ")
	if sanitized == "" {
		return "_"
	}

	return sanitized
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

	maxSize := d.config.FileExistenceCacheMaxSize
	if maxSize == 0 {
		maxSize = 10000 // Default
	}

	// Evict oldest entries if at capacity
	if len(d.fileExistenceCache) >= maxSize {
		// Simple eviction: remove first 10% of entries
		evictCount := maxSize / 10
		if evictCount == 0 {
			evictCount = 1 // Ensure at least one entry is evicted
		}
		count := 0
		for k := range d.fileExistenceCache {
			if count >= evictCount {
				break
			}
			delete(d.fileExistenceCache, k)
			count++
		}
	}

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
		"size":     size,
		"max_size": maxSize,
	}
}
