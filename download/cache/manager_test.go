package cache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager(".cache")
	if m.CacheDir() != ".cache" {
		t.Errorf("CacheDir() = %q, want .cache", m.CacheDir())
	}
}

func TestManager_LoadSpotify_MissingFile(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	entries, err := m.LoadSpotify()
	if err != nil {
		t.Fatalf("LoadSpotify: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("LoadSpotify: got %d entries, want 0", len(entries))
	}
}

func TestManager_SaveSpotify_LoadSpotify_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	entries := map[string]SpotifyEntry{
		"spotify:track:abc": NewSpotifyEntry(map[string]string{"title": "Song"}),
	}
	if err := m.SaveSpotify(entries); err != nil {
		t.Fatalf("SaveSpotify: %v", err)
	}
	loaded, err := m.LoadSpotify()
	if err != nil {
		t.Fatalf("LoadSpotify: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("LoadSpotify: got %d entries, want 1", len(loaded))
	}
	e, ok := loaded["spotify:track:abc"]
	if !ok {
		t.Fatal("key spotify:track:abc not found")
	}
	if e.TTLSeconds != SpotifyTTLSeconds {
		t.Errorf("TTLSeconds = %d, want %d", e.TTLSeconds, SpotifyTTLSeconds)
	}
	meta, ok := e.Metadata.(map[string]interface{})
	if !ok {
		t.Errorf("Metadata type = %T", e.Metadata)
	} else if meta["title"] != "Song" {
		t.Errorf("Metadata title = %v", meta["title"])
	}
}

func TestManager_LoadSpotify_FiltersExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, SpotifyCacheFile)
	// One valid entry (cached_at now + 1h TTL), one expired (cached_at in the past)
	validAt := time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)
	payload := `{
  "spotify:track:valid": {
    "metadata": {"title": "Valid"},
    "cached_at": "` + validAt + `",
    "ttl_seconds": 3600
  },
  "spotify:track:expired": {
    "metadata": {"title": "Expired"},
    "cached_at": "2020-01-01T12:00:00Z",
    "ttl_seconds": 1
  }
}`
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(payload), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m := NewManager(dir)
	loaded, err := m.LoadSpotify()
	if err != nil {
		t.Fatalf("LoadSpotify: %v", err)
	}
	if len(loaded) != 1 {
		t.Errorf("LoadSpotify: got %d entries (expected 1 after TTL filter), want 1", len(loaded))
	}
	if _, ok := loaded["spotify:track:valid"]; !ok {
		t.Error("valid entry should be present")
	}
	if _, ok := loaded["spotify:track:expired"]; ok {
		t.Error("expired entry should be filtered out")
	}
}

func TestManager_LoadYouTube_SaveYouTube_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	entries := map[string]YouTubeEntry{
		"https://www.youtube.com/watch?v=xyz": NewYouTubeEntry(map[string]interface{}{
			"title": "Video", "duration": 180,
		}),
	}
	if err := m.SaveYouTube(entries); err != nil {
		t.Fatalf("SaveYouTube: %v", err)
	}
	loaded, err := m.LoadYouTube()
	if err != nil {
		t.Fatalf("LoadYouTube: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("LoadYouTube: got %d entries, want 1", len(loaded))
	}
	e := loaded["https://www.youtube.com/watch?v=xyz"]
	if e.TTLSeconds != YouTubeTTLSeconds {
		t.Errorf("TTLSeconds = %d, want %d", e.TTLSeconds, YouTubeTTLSeconds)
	}
}

func TestManager_LoadYouTube_FiltersExpired(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, YouTubeCacheFile)
	payload := `{
  "https://youtube.com/watch?v=old": {
    "metadata": {},
    "cached_at": "2020-01-01T00:00:00Z",
    "ttl_seconds": 86400
  }
}`
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := os.WriteFile(path, []byte(payload), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	m := NewManager(dir)
	loaded, err := m.LoadYouTube()
	if err != nil {
		t.Fatalf("LoadYouTube: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("LoadYouTube: got %d entries (expired should be filtered), want 0", len(loaded))
	}
}

func TestManager_LoadDownload_SaveDownload_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	entries := map[string]DownloadEntry{
		"track-1": {
			OutputPath:    "Artist/Album/01 - Title.mp3",
			Status:        "completed",
			DownloadedAt:  time.Now().UTC().Format(time.RFC3339),
			FileSizeBytes: 1024,
		},
	}
	if err := m.SaveDownload(entries); err != nil {
		t.Fatalf("SaveDownload: %v", err)
	}
	loaded, err := m.LoadDownload()
	if err != nil {
		t.Fatalf("LoadDownload: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("LoadDownload: got %d entries, want 1", len(loaded))
	}
	e := loaded["track-1"]
	if e.OutputPath != "Artist/Album/01 - Title.mp3" || e.Status != "completed" {
		t.Errorf("OutputPath=%q Status=%q", e.OutputPath, e.Status)
	}
	if e.FileSizeBytes != 1024 {
		t.Errorf("FileSizeBytes = %d, want 1024", e.FileSizeBytes)
	}
}

func TestManager_LoadDownload_MissingFile(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	entries, err := m.LoadDownload()
	if err != nil {
		t.Fatalf("LoadDownload: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("LoadDownload: got %d entries, want 0", len(entries))
	}
}

func TestIsExpired(t *testing.T) {
	now := time.Now()
	future := now.Add(2 * time.Hour).Format(time.RFC3339)
	past := now.Add(-2 * time.Hour).Format(time.RFC3339)
	if isExpired(future, 3600) {
		t.Error("future cached_at with 3600 TTL should not be expired")
	}
	if !isExpired(past, 3600) {
		t.Error("past cached_at with 3600 TTL should be expired")
	}
	if isExpired(past, 0) {
		t.Error("TTL 0 should never be expired")
	}
}

func TestNewSpotifyEntry_NewYouTubeEntry(t *testing.T) {
	se := NewSpotifyEntry(map[string]string{"id": "x"})
	if se.TTLSeconds != SpotifyTTLSeconds {
		t.Errorf("NewSpotifyEntry TTL = %d", se.TTLSeconds)
	}
	if se.CachedAt == "" {
		t.Error("CachedAt should be set")
	}
	ye := NewYouTubeEntry(map[string]string{"id": "y"})
	if ye.TTLSeconds != YouTubeTTLSeconds {
		t.Errorf("NewYouTubeEntry TTL = %d", ye.TTLSeconds)
	}
}
