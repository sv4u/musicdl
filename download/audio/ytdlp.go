package audio

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ytDlpSearchResult represents the result from yt-dlp search.
type ytDlpSearchResult struct {
	URL        string              `json:"url,omitempty"`
	WebpageURL string              `json:"webpage_url,omitempty"`
	ID         string              `json:"id,omitempty"`
	Entries    []ytDlpSearchResult `json:"entries,omitempty"`
}

// YouTubeVideoMetadata represents structured metadata for a YouTube video.
type YouTubeVideoMetadata struct {
	VideoID     string   `json:"video_id"`
	Title       string   `json:"title"`
	Description string   `json:"description,omitempty"`
	Duration    int      `json:"duration"` // Duration in seconds
	Uploader    string   `json:"uploader,omitempty"`
	UploadDate  string   `json:"upload_date,omitempty"`
	ViewCount   int64    `json:"view_count,omitempty"`
	Thumbnail   string   `json:"thumbnail,omitempty"`
	WebpageURL  string   `json:"webpage_url,omitempty"`
	Categories  []string `json:"categories,omitempty"`
	Tags        []string `json:"tags,omitempty"`
}

// YouTubePlaylistInfo represents structured metadata for a YouTube playlist.
type YouTubePlaylistInfo struct {
	PlaylistID  string                 `json:"playlist_id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Uploader    string                 `json:"uploader,omitempty"`
	VideoCount  int                    `json:"video_count,omitempty"`
	WebpageURL  string                 `json:"webpage_url,omitempty"`
	Thumbnail   string                 `json:"thumbnail,omitempty"`
	Entries     []YouTubeVideoMetadata `json:"entries,omitempty"`
}

// runYtDlpSearch runs yt-dlp to search for audio.
func (p *Provider) runYtDlpSearch(ctx context.Context, searchQuery string) (string, error) {
	// Build yt-dlp command for search
	args := []string{
		"--quiet",
		"--no-warnings",
		"--flat-playlist",
		"--default-search", "extract",
		"--dump-json",
		searchQuery,
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		// Check if it's a rate limit error
		if strings.Contains(outputStr, "429") ||
			strings.Contains(outputStr, "rate limit") ||
			strings.Contains(outputStr, "HTTP Error 429") {
			return "", &SearchError{
				Message:  "Rate limited by provider",
				Original: err,
			}
		}
		// Return error with output for debugging
		return "", &SearchError{
			Message:  fmt.Sprintf("yt-dlp search failed: %v (output: %s)", err, outputStr),
			Original: err,
		}
	}

	// Parse JSON output
	// yt-dlp may return multiple results (one per line)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 0 {
		return "", &SearchError{Message: "No results from yt-dlp"}
	}

	// Parse first result
	var result ytDlpSearchResult
	if err := json.Unmarshal([]byte(lines[0]), &result); err != nil {
		return "", &SearchError{
			Message:  "Failed to parse yt-dlp output",
			Original: err,
		}
	}

	// Extract URL
	url := result.URL
	if url == "" {
		url = result.WebpageURL
	}
	if url == "" && result.ID != "" {
		// Construct URL from ID
		if strings.HasPrefix(searchQuery, "ytsearch") {
			url = fmt.Sprintf("https://www.youtube.com/watch?v=%s", result.ID)
		} else if strings.HasPrefix(searchQuery, "scsearch") {
			url = fmt.Sprintf("https://soundcloud.com/%s", result.ID)
		}
	}

	// Handle entries (playlist results)
	if len(result.Entries) > 0 && url == "" {
		firstEntry := result.Entries[0]
		url = firstEntry.URL
		if url == "" {
			url = firstEntry.WebpageURL
		}
		if url == "" && firstEntry.ID != "" {
			if strings.HasPrefix(searchQuery, "ytsearch") {
				url = fmt.Sprintf("https://www.youtube.com/watch?v=%s", firstEntry.ID)
			}
		}
	}

	if url == "" {
		return "", &SearchError{Message: "No URL found in yt-dlp result"}
	}

	return url, nil
}

// runYtDlpDownload runs yt-dlp to download audio.
func (p *Provider) runYtDlpDownload(ctx context.Context, url, outputPath string) (string, error) {
	// Ensure output directory exists
	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", &DownloadError{
			Message:  fmt.Sprintf("Failed to create output directory: %s", outputDir),
			Original: err,
		}
	}

	// Determine format string for yt-dlp
	var formatStr string
	switch p.config.OutputFormat {
	case "m4a":
		formatStr = "bestaudio[ext=m4a]/bestaudio/best"
	case "opus":
		formatStr = "bestaudio[ext=webm]/bestaudio/best"
	default:
		formatStr = "bestaudio"
	}

	// Build output template (yt-dlp will add extension)
	outputTemplate := outputPath
	if filepath.Ext(outputTemplate) != "" {
		// Remove extension, yt-dlp will add it
		outputTemplate = strings.TrimSuffix(outputTemplate, filepath.Ext(outputTemplate))
	}
	outputTemplate = fmt.Sprintf("%s.%%(ext)s", outputTemplate)

	// Build yt-dlp command
	args := []string{
		"--format", formatStr,
		"--quiet",
		"--no-warnings",
		"--encoding", "UTF-8",
		"--output", outputTemplate,
		url,
	}

	// Add postprocessor for format conversion if needed
	if p.config.OutputFormat != "" && p.config.Bitrate != "disable" {
		args = append(args,
			"--postprocessor-args", fmt.Sprintf("ffmpeg:-b:a %s", p.config.Bitrate),
		)

		// Add format-specific postprocessor
		switch p.config.OutputFormat {
		case "mp3":
			args = append(args, "--extract-audio", "--audio-format", "mp3", "--audio-quality", p.config.Bitrate)
		case "flac":
			args = append(args, "--extract-audio", "--audio-format", "flac")
		case "m4a":
			args = append(args, "--extract-audio", "--audio-format", "m4a", "--audio-quality", p.config.Bitrate)
		case "opus":
			args = append(args, "--extract-audio", "--audio-format", "opus", "--audio-quality", p.config.Bitrate)
		}
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	if err := cmd.Run(); err != nil {
		// Check if it's a rate limit error
		if exitErr, ok := err.(*exec.ExitError); ok {
			stderr := string(exitErr.Stderr)
			if strings.Contains(stderr, "429") ||
				strings.Contains(stderr, "rate limit") ||
				strings.Contains(stderr, "HTTP Error 429") {
				return "", &DownloadError{
					Message:  "Rate limited by provider",
					Original: err,
				}
			}
		}
		return "", &DownloadError{
			Message:  fmt.Sprintf("yt-dlp download failed: %v", err),
			Original: err,
		}
	}

	// Find the actual downloaded file (yt-dlp may change extension)
	downloadedPath := p.findDownloadedFile(outputPath)
	if downloadedPath == "" {
		return "", &DownloadError{
			Message: fmt.Sprintf("Downloaded file not found at %s", outputPath),
		}
	}

	return downloadedPath, nil
}

// findDownloadedFile finds the actual downloaded file.
func (p *Provider) findDownloadedFile(expectedPath string) string {
	// Try expected path first
	if _, err := os.Stat(expectedPath); err == nil {
		return expectedPath
	}

	// Try with different extensions
	basePath := strings.TrimSuffix(expectedPath, filepath.Ext(expectedPath))
	extensions := []string{
		p.config.OutputFormat,
		"m4a", "webm", "opus", "mp3", "flac",
	}

	for _, ext := range extensions {
		candidate := fmt.Sprintf("%s.%s", basePath, ext)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Try to find any file with similar name in directory
	dir := filepath.Dir(expectedPath)
	baseName := filepath.Base(basePath)
	entries, err := os.ReadDir(dir)
	if err == nil {
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), baseName) {
				return filepath.Join(dir, entry.Name())
			}
		}
	}

	return ""
}

// GetVideoMetadata extracts metadata for a YouTube video using yt-dlp.
func (p *Provider) GetVideoMetadata(ctx context.Context, videoURL string) (*YouTubeVideoMetadata, error) {
	// Build yt-dlp command to extract metadata
	args := []string{
		"--quiet",
		"--no-warnings",
		"--dump-json",
		"--no-playlist", // Only get video, not playlist if URL contains playlist param
		videoURL,
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		return nil, &SearchError{
			Message:  fmt.Sprintf("yt-dlp metadata extraction failed: %v (output: %s)", err, outputStr),
			Original: err,
		}
	}

	// Parse JSON output
	var rawData map[string]interface{}
	if err := json.Unmarshal(output, &rawData); err != nil {
		return nil, &SearchError{
			Message:  "Failed to parse yt-dlp metadata output",
			Original: err,
		}
	}

	// Extract structured metadata
	metadata := &YouTubeVideoMetadata{}

	// Extract video ID
	if id, ok := rawData["id"].(string); ok {
		metadata.VideoID = id
	}

	// Extract title
	if title, ok := rawData["title"].(string); ok {
		metadata.Title = title
	}

	// Extract description
	if desc, ok := rawData["description"].(string); ok {
		metadata.Description = desc
	}

	// Extract duration (can be int or float)
	if duration, ok := rawData["duration"].(float64); ok {
		metadata.Duration = int(duration)
	} else if duration, ok := rawData["duration"].(int); ok {
		metadata.Duration = duration
	}

	// Extract uploader
	if uploader, ok := rawData["uploader"].(string); ok {
		metadata.Uploader = uploader
	} else if channel, ok := rawData["channel"].(string); ok {
		metadata.Uploader = channel
	}

	// Extract upload date
	if uploadDate, ok := rawData["upload_date"].(string); ok {
		metadata.UploadDate = uploadDate
	}

	// Extract view count
	if viewCount, ok := rawData["view_count"].(float64); ok {
		metadata.ViewCount = int64(viewCount)
	}

	// Extract thumbnail (prefer highest quality)
	// yt-dlp returns thumbnails sorted from lowest to highest quality
	if thumbnails, ok := rawData["thumbnails"].([]interface{}); ok && len(thumbnails) > 0 {
		// Get the last thumbnail (highest quality)
		lastThumb := thumbnails[len(thumbnails)-1]
		if thumbMap, ok := lastThumb.(map[string]interface{}); ok {
			if url, ok := thumbMap["url"].(string); ok {
				metadata.Thumbnail = url
			}
		}
	} else if thumbnail, ok := rawData["thumbnail"].(string); ok {
		metadata.Thumbnail = thumbnail
	}

	// Extract webpage URL
	if webpageURL, ok := rawData["webpage_url"].(string); ok {
		metadata.WebpageURL = webpageURL
	} else if url, ok := rawData["url"].(string); ok {
		metadata.WebpageURL = url
	}

	// Extract categories
	if categories, ok := rawData["categories"].([]interface{}); ok {
		metadata.Categories = make([]string, 0, len(categories))
		for _, cat := range categories {
			if catStr, ok := cat.(string); ok {
				metadata.Categories = append(metadata.Categories, catStr)
			}
		}
	}

	// Extract tags
	if tags, ok := rawData["tags"].([]interface{}); ok {
		metadata.Tags = make([]string, 0, len(tags))
		for _, tag := range tags {
			if tagStr, ok := tag.(string); ok {
				metadata.Tags = append(metadata.Tags, tagStr)
			}
		}
	}

	return metadata, nil
}

// GetPlaylistInfo extracts metadata for a YouTube playlist using yt-dlp.
// With --flat-playlist --dump-json, yt-dlp outputs one JSON object per line: the first
// line is playlist metadata, and each following line is a playlist entry (video).
func (p *Provider) GetPlaylistInfo(ctx context.Context, playlistURL string) (*YouTubePlaylistInfo, error) {
	args := []string{
		"--quiet",
		"--no-warnings",
		"--dump-json",
		"--flat-playlist",
		playlistURL,
	}

	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		outputStr := string(output)
		return nil, &SearchError{
			Message:  fmt.Sprintf("yt-dlp playlist metadata extraction failed: %v (output: %s)", err, outputStr),
			Original: err,
		}
	}
	return parsePlaylistOutput(string(output))
}

// parsePlaylistOutput parses yt-dlp --flat-playlist --dump-json output.
// First line is playlist metadata; each following line is a playlist entry (video).
// Used by GetPlaylistInfo and by tests.
func parsePlaylistOutput(output string) (*YouTubePlaylistInfo, error) {
	trimmed := strings.TrimSpace(output)
	if trimmed == "" {
		return nil, &SearchError{Message: "No playlist metadata from yt-dlp"}
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) == 0 {
		return nil, &SearchError{Message: "No playlist metadata from yt-dlp"}
	}

	info := &YouTubePlaylistInfo{}
	info.Entries = make([]YouTubeVideoMetadata, 0)

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var raw map[string]interface{}
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		if i == 0 {
			// First line is playlist-level metadata
			if id, ok := raw["id"].(string); ok {
				info.PlaylistID = id
			}
			if title, ok := raw["title"].(string); ok {
				info.Title = title
			}
			if desc, ok := raw["description"].(string); ok {
				info.Description = desc
			}
			if uploader, ok := raw["uploader"].(string); ok {
				info.Uploader = uploader
			} else if channel, ok := raw["channel"].(string); ok {
				info.Uploader = channel
			}
			if count, ok := raw["playlist_count"].(float64); ok {
				info.VideoCount = int(count)
			}
			if webpageURL, ok := raw["webpage_url"].(string); ok {
				info.WebpageURL = webpageURL
			}
			if thumbnails, ok := raw["thumbnails"].([]interface{}); ok && len(thumbnails) > 0 {
				lastThumb := thumbnails[len(thumbnails)-1]
				if thumbMap, ok := lastThumb.(map[string]interface{}); ok {
					if url, ok := thumbMap["url"].(string); ok {
						info.Thumbnail = url
					}
				}
			} else if thumbnail, ok := raw["thumbnail"].(string); ok {
				info.Thumbnail = thumbnail
			}
			// Inline entries if present (some yt-dlp versions)
			if entries, ok := raw["entries"].([]interface{}); ok {
				for _, e := range entries {
					if entryMap, ok := e.(map[string]interface{}); ok {
						if meta := parsePlaylistEntry(entryMap); meta != nil {
							info.Entries = append(info.Entries, *meta)
						}
					}
				}
			}
			continue
		}
		// Subsequent lines: one JSON object per video with --flat-playlist
		if meta := parsePlaylistEntry(raw); meta != nil {
			info.Entries = append(info.Entries, *meta)
		}
	}

	return info, nil
}

// parsePlaylistEntry extracts YouTubeVideoMetadata from a yt-dlp JSON object (playlist entry).
// Returns nil if the object does not look like a video entry (e.g. no id).
func parsePlaylistEntry(entryMap map[string]interface{}) *YouTubeVideoMetadata {
	videoID, ok := entryMap["id"].(string)
	if !ok || videoID == "" {
		return nil
	}
	// Skip playlist container entries that might appear in nested "entries"
	if entryType, _ := entryMap["_type"].(string); entryType == "playlist" {
		return nil
	}
	meta := &YouTubeVideoMetadata{VideoID: videoID}
	if title, ok := entryMap["title"].(string); ok {
		meta.Title = title
	}
	if duration, ok := entryMap["duration"].(float64); ok {
		meta.Duration = int(duration)
	} else if duration, ok := entryMap["duration"].(int); ok {
		meta.Duration = duration
	}
	if uploader, ok := entryMap["uploader"].(string); ok {
		meta.Uploader = uploader
	} else if channel, ok := entryMap["channel"].(string); ok {
		meta.Uploader = channel
	}
	if url, ok := entryMap["url"].(string); ok {
		meta.WebpageURL = url
	} else if webpageURL, ok := entryMap["webpage_url"].(string); ok {
		meta.WebpageURL = webpageURL
	}
	return meta
}
