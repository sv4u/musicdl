package main

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
)

const cookiesStatusCacheTTL = 5 * time.Minute
const cookiesProbeTimeout = 25 * time.Second

// cookiesStatusHandler returns Tier A (file / config heuristics) and Tier B (yt-dlp probe).
// Results are cached for cookiesStatusCacheTTL unless query force=1 or force=true.
// @Summary YouTube cookie status
// @Tags system
// @Produce json
// @Router /api/cookies-status [get]
func (s *APIServer) cookiesStatusHandler(w http.ResponseWriter, r *http.Request) {
	force := r.URL.Query().Get("force") == "1" || strings.EqualFold(r.URL.Query().Get("force"), "true")

	s.cookiesStatusMu.Lock()
	if !force && len(s.cookiesStatusCacheJSON) > 0 && time.Since(s.cookiesStatusCachedAt) < cookiesStatusCacheTTL {
		data := s.cookiesStatusCacheJSON
		s.cookiesStatusMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(data)
		return
	}
	s.cookiesStatusMu.Unlock()

	configPath := getDefaultConfigPath()
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "config.yaml not found"})
		return
	}

	cfg, err := config.LoadConfig(configPath)
	resp := map[string]interface{}{
		"source":             "none",
		"cookiesPath":        "",
		"cookiesPathRaw":     "",
		"cookiesFromBrowser": "",
		"tierA":              map[string]interface{}{},
		"tierB":              map[string]interface{}{},
	}
	if err != nil {
		resp["configError"] = err.Error()
		resp["tierA"] = map[string]interface{}{
			"ready": false,
			"issues": []string{"Fix config.yaml so it validates; cookie paths cannot be read until then."},
		}
		resp["tierB"] = map[string]interface{}{
			"skipped":       true,
			"valid":         nil,
			"message":       "Config did not load; yt-dlp probe not run.",
			"probeUrl":      "",
			"checkedAtUnix": time.Now().Unix(),
		}
	} else {
		buildCookiesStatusResponse(cfg, resp)
	}

	payload, errEnc := json.Marshal(resp)
	if errEnc != nil {
		jsonError(w, "failed to encode cookies status", http.StatusInternalServerError)
		return
	}

	s.cookiesStatusMu.Lock()
	s.cookiesStatusCacheJSON = payload
	s.cookiesStatusCachedAt = time.Now()
	s.cookiesStatusMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(payload)
}

func resolveCookiesPath(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if filepath.IsAbs(raw) {
		return filepath.Clean(raw)
	}
	dir := os.Getenv("MUSICDL_WORK_DIR")
	if dir == "" {
		dir = "."
	}
	return filepath.Clean(filepath.Join(dir, raw))
}

const maxCookieJarScan = 256 * 1024

// netscapeJarHasYouTubeOrGoogle returns true if a line looks like a Netscape jar row for YouTube/Google.
func netscapeJarHasYouTubeOrGoogle(jar []byte) bool {
	lines := strings.Split(string(jar), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) < 7 {
			continue
		}
		d := strings.ToLower(strings.TrimSpace(parts[0]))
		if strings.Contains(d, "youtube") || strings.Contains(d, "google") {
			return true
		}
	}
	return false
}

func buildCookiesStatusResponse(cfg *config.MusicDLConfig, resp map[string]interface{}) {
	d := &cfg.Download
	cookiesRaw := strings.TrimSpace(d.Cookies)
	browser := strings.TrimSpace(d.CookiesFromBrowser)

	source := "none"
	if cookiesRaw != "" {
		source = "file"
	} else if browser != "" {
		source = "browser"
	}
	resp["source"] = source
	resp["cookiesPathRaw"] = cookiesRaw

	resolved := ""
	if cookiesRaw != "" {
		resolved = resolveCookiesPath(cookiesRaw)
		resp["cookiesPath"] = resolved
	}
	resp["cookiesFromBrowser"] = browser

	issues := make([]string, 0, 4)
	tierA := map[string]interface{}{
		"ready":                     false,
		"fileExists":                false,
		"fileReadable":              false,
		"fileSizeBytes":             int64(0),
		"hasYoutubeOrGoogleJarLine": false,
	}

	switch source {
	case "file":
		if resolved == "" {
			issues = append(issues, "cookies path is empty after resolve")
			break
		}
		st, err := os.Stat(resolved)
		if err != nil {
			if os.IsNotExist(err) {
				issues = append(issues, "cookies file does not exist at resolved path")
			} else {
				issues = append(issues, "cannot stat cookies file: "+err.Error())
			}
			break
		}
		if st.IsDir() {
			issues = append(issues, "cookies path is a directory, not a file")
			break
		}
		tierA["fileExists"] = true
		tierA["fileSizeBytes"] = st.Size()
		if st.Size() == 0 {
			issues = append(issues, "cookies file is empty")
			break
		}
		data, err := os.ReadFile(resolved)
		if err != nil {
			issues = append(issues, "cannot read cookies file: "+err.Error())
			break
		}
		tierA["fileReadable"] = true
		if len(data) > maxCookieJarScan {
			data = data[:maxCookieJarScan]
		}
		if !netscapeJarHasYouTubeOrGoogle(data) {
			issues = append(issues, "no YouTube/Google domain rows detected in cookie jar (export may be incomplete)")
		} else {
			tierA["hasYoutubeOrGoogleJarLine"] = true
		}
		if len(issues) == 0 {
			tierA["ready"] = true
		}
	case "browser":
		tierA["ready"] = true
	case "none":
		issues = append(issues, "no download.cookies or download.cookies_from_browser set")
	}
	tierA["issues"] = issues
	resp["tierA"] = tierA

	probeURL := audio.ResolveProbeURL()
	tierB := map[string]interface{}{
		"skipped":       false,
		"valid":         nil,
		"message":       "",
		"probeUrl":      probeURL,
		"checkedAtUnix": time.Now().Unix(),
	}

	if source == "none" {
		tierB["skipped"] = true
		tierB["valid"] = nil
		tierB["message"] = "No cookies configured; yt-dlp probe skipped (not required for non-YouTube providers)."
		resp["tierB"] = tierB
		return
	}

	acfg := &audio.Config{
		OutputFormat:       d.Format,
		Bitrate:            d.Bitrate,
		AudioProviders:     append([]string(nil), d.AudioProviders...),
		CookiesFromBrowser: "",
		Cookies:            "",
		JSRuntimes:         d.JSRuntimes,
		RemoteComponents:   d.RemoteComponents,
	}
	if source == "file" {
		acfg.Cookies = resolved
	} else {
		acfg.CookiesFromBrowser = browser
	}

	ctx, cancel := context.WithTimeout(context.Background(), cookiesProbeTimeout)
	defer cancel()
	ok, detail := audio.ProbeYouTubeCookieAuth(ctx, acfg, probeURL)
	tierB["valid"] = ok
	tierB["message"] = detail
	tierB["checkedAtUnix"] = time.Now().Unix()
	resp["tierB"] = tierB
}
