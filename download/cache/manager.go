package cache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// File names under cache dir (spec: .cache/spotify_cache.json, etc.)
const (
	SpotifyCacheFile  = "spotify_cache.json"
	YouTubeCacheFile  = "youtube_cache.json"
	DownloadCacheFile = "download_cache.json"
)

// Default TTLs in seconds (spec).
const (
	SpotifyTTLSeconds  = 3600  // 1 hour
	YouTubeTTLSeconds  = 86400 // 24 hours
	DownloadTTLSeconds = 0     // download cache has no TTL
)

// SpotifyEntry is one Spotify cache entry (spec: metadata, cached_at, ttl_seconds).
type SpotifyEntry struct {
	Metadata   interface{} `json:"metadata"`
	CachedAt   string      `json:"cached_at"`
	TTLSeconds int         `json:"ttl_seconds"`
}

// YouTubeEntry is one YouTube cache entry (spec: metadata, cached_at, ttl_seconds).
type YouTubeEntry struct {
	Metadata   interface{} `json:"metadata"`
	CachedAt   string      `json:"cached_at"`
	TTLSeconds int         `json:"ttl_seconds"`
}

// DownloadEntry is one download cache entry (spec: output_path, status, downloaded_at/last_attempt, file_size_bytes/error).
type DownloadEntry struct {
	OutputPath    string `json:"output_path"`
	Status        string `json:"status"`
	DownloadedAt  string `json:"downloaded_at,omitempty"`
	LastAttempt   string `json:"last_attempt,omitempty"`
	FileSizeBytes int64  `json:"file_size_bytes,omitempty"`
	Error         string `json:"error,omitempty"`
	Checksum      string `json:"checksum,omitempty"`
}

// Manager loads and saves cache JSON files under a cache directory, with TTL filtering on read.
// It is thread-safe.
type Manager struct {
	cacheDir string
	mu       sync.RWMutex
}

// NewManager returns a cache manager for the given cache directory.
func NewManager(cacheDir string) *Manager {
	return &Manager{cacheDir: cacheDir}
}

// path returns the full path for a cache file name.
func (m *Manager) path(filename string) string {
	return filepath.Join(m.cacheDir, filename)
}

// isExpired returns true if cachedAt + ttlSeconds is before now.
func isExpired(cachedAt string, ttlSeconds int) bool {
	if ttlSeconds <= 0 {
		return false
	}
	t, err := time.Parse(time.RFC3339, cachedAt)
	if err != nil {
		return true
	}
	return time.Now().After(t.Add(time.Duration(ttlSeconds) * time.Second))
}

// LoadSpotify loads the Spotify cache from disk and returns only non-expired entries.
func (m *Manager) LoadSpotify() (map[string]SpotifyEntry, error) {
	m.mu.RLock()
	path := m.path(SpotifyCacheFile)
	m.mu.RUnlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]SpotifyEntry), nil
		}
		return nil, err
	}

	var raw map[string]SpotifyEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	out := make(map[string]SpotifyEntry)
	for k, v := range raw {
		if !isExpired(v.CachedAt, v.TTLSeconds) {
			out[k] = v
		}
	}
	return out, nil
}

// SaveSpotify writes the Spotify cache to disk. Creates cache dir if needed.
func (m *Manager) SaveSpotify(entries map[string]SpotifyEntry) error {
	m.mu.Lock()
	cacheDir := m.cacheDir
	m.mu.Unlock()

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(cacheDir, SpotifyCacheFile)
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadYouTube loads the YouTube cache from disk and returns only non-expired entries.
func (m *Manager) LoadYouTube() (map[string]YouTubeEntry, error) {
	m.mu.RLock()
	path := m.path(YouTubeCacheFile)
	m.mu.RUnlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]YouTubeEntry), nil
		}
		return nil, err
	}

	var raw map[string]YouTubeEntry
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	out := make(map[string]YouTubeEntry)
	for k, v := range raw {
		if !isExpired(v.CachedAt, v.TTLSeconds) {
			out[k] = v
		}
	}
	return out, nil
}

// SaveYouTube writes the YouTube cache to disk. Creates cache dir if needed.
func (m *Manager) SaveYouTube(entries map[string]YouTubeEntry) error {
	m.mu.Lock()
	cacheDir := m.cacheDir
	m.mu.Unlock()

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(cacheDir, YouTubeCacheFile)
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadDownload loads the download cache from disk (no TTL).
func (m *Manager) LoadDownload() (map[string]DownloadEntry, error) {
	m.mu.RLock()
	path := m.path(DownloadCacheFile)
	m.mu.RUnlock()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]DownloadEntry), nil
		}
		return nil, err
	}

	var out map[string]DownloadEntry
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = make(map[string]DownloadEntry)
	}
	return out, nil
}

// SaveDownload writes the download cache to disk. Creates cache dir if needed.
func (m *Manager) SaveDownload(entries map[string]DownloadEntry) error {
	m.mu.Lock()
	cacheDir := m.cacheDir
	m.mu.Unlock()

	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(cacheDir, DownloadCacheFile)
	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// CacheDir returns the cache directory.
func (m *Manager) CacheDir() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cacheDir
}

// NewSpotifyEntry builds a SpotifyEntry with cached_at and TTL.
func NewSpotifyEntry(metadata interface{}) SpotifyEntry {
	return SpotifyEntry{
		Metadata:   metadata,
		CachedAt:   time.Now().UTC().Format(time.RFC3339),
		TTLSeconds: SpotifyTTLSeconds,
	}
}

// NewYouTubeEntry builds a YouTubeEntry with cached_at and TTL.
func NewYouTubeEntry(metadata interface{}) YouTubeEntry {
	return YouTubeEntry{
		Metadata:   metadata,
		CachedAt:   time.Now().UTC().Format(time.RFC3339),
		TTLSeconds: YouTubeTTLSeconds,
	}
}
