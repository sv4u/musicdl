package audio

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// DefaultYouTubeCookieProbeURL is used when no YOUTUBE_PROBE_URL is set (cookie export script and API Tier B probe).
// Matches scripts/export-vivaldi-cookies.sh default.
const DefaultYouTubeCookieProbeURL = "https://www.youtube.com/watch?v=p4n8uCxhBTk"

// ProbeYouTubeCookieAuth runs yt-dlp with --skip-download using the same cookie and JS flags
// as normal downloads. It returns ok=true when yt-dlp exits zero (session can extract the probe video).
func ProbeYouTubeCookieAuth(ctx context.Context, cfg *Config, probeURL string) (ok bool, detail string) {
	if probeURL == "" {
		probeURL = DefaultYouTubeCookieProbeURL
	}
	args := []string{
		"--skip-download",
		"--no-warnings",
		"--quiet",
		"--age-limit", "99",
	}
	var err error
	args, err = prependYtDlpWritableCache(args)
	if err != nil {
		return false, fmt.Sprintf("yt-dlp cache directory: %v", err)
	}
	args = appendYtDlpYouTubeOpts(cfg, args)
	args = append(args, probeURL)
	cmd := exec.CommandContext(ctx, "yt-dlp", args...)
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text == "" {
			text = err.Error()
		}
		const maxLen = 800
		if len(text) > maxLen {
			text = text[:maxLen] + "…"
		}
		return false, text
	}
	if text != "" {
		return true, strings.TrimSpace(text)
	}
	return true, "yt-dlp probe succeeded"
}

// ResolveProbeURL returns YOUTUBE_PROBE_URL when set, otherwise DefaultYouTubeCookieProbeURL.
func ResolveProbeURL() string {
	if v := strings.TrimSpace(os.Getenv("YOUTUBE_PROBE_URL")); v != "" {
		return v
	}
	return DefaultYouTubeCookieProbeURL
}
