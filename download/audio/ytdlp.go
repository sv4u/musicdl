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
	URL        string `json:"url,omitempty"`
	WebpageURL string `json:"webpage_url,omitempty"`
	ID         string `json:"id,omitempty"`
	Entries    []ytDlpSearchResult `json:"entries,omitempty"`
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
