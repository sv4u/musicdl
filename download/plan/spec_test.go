package plan

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestPlanToSpecAndSpecToPlan_RoundTrip(t *testing.T) {
	plan := NewDownloadPlan(map[string]interface{}{"config_version": "1.2"})
	plan.AddItem(&PlanItem{
		ItemID:     "track-1",
		ItemType:   PlanItemTypeTrack,
		YouTubeURL: "https://www.youtube.com/watch?v=abc",
		SpotifyID:  "xyz",
		FilePath:   "Artist/Album/01 - Title.mp3",
		Status:     PlanItemStatusPending,
		Metadata: map[string]interface{}{
			"spotify_metadata": map[string]interface{}{"title": "Title"},
			"source_context":   map[string]interface{}{"type": "song"},
		},
		CreatedAt: time.Now(),
		Progress:  0,
	})
	plan.AddItem(&PlanItem{
		ItemID:     "playlist-1",
		ItemType:   PlanItemTypePlaylist,
		Name:       "My Playlist",
		SpotifyURL: "https://open.spotify.com/playlist/abc",
		ChildIDs:   []string{"track-1"},
		Status:     PlanItemStatusPending,
		Metadata:   map[string]interface{}{"create_m3u": true},
		CreatedAt:  time.Now(),
		Progress:   0,
	})

	configHash := "a1b2c3d4e5f67890"
	configFile := "musicdl-config.yml"
	generatedAt := time.Now().UTC()

	spec := PlanToSpec(plan, configHash, configFile, generatedAt)
	if spec.ConfigHash != configHash {
		t.Errorf("spec.ConfigHash = %q, want %q", spec.ConfigHash, configHash)
	}
	if spec.ConfigFile != configFile {
		t.Errorf("spec.ConfigFile = %q, want %q", spec.ConfigFile, configFile)
	}
	if spec.TotalTracks != 1 {
		t.Errorf("spec.TotalTracks = %d, want 1", spec.TotalTracks)
	}
	if len(spec.Downloads) != 1 {
		t.Fatalf("spec.Downloads len = %d, want 1", len(spec.Downloads))
	}
	if len(spec.Playlists) != 1 {
		t.Fatalf("spec.Playlists len = %d, want 1", len(spec.Playlists))
	}

	back, err := SpecToPlan(spec)
	if err != nil {
		t.Fatalf("SpecToPlan: %v", err)
	}
	if len(back.Items) != 2 {
		t.Errorf("after round-trip Items len = %d, want 2", len(back.Items))
	}
	var trackItem *PlanItem
	for _, it := range back.Items {
		if it.ItemType == PlanItemTypeTrack {
			trackItem = it
			break
		}
	}
	if trackItem == nil {
		t.Fatal("no track item after round-trip")
	}
	if trackItem.ItemID != "track-1" || trackItem.YouTubeURL != "https://www.youtube.com/watch?v=abc" {
		t.Errorf("track item: id=%q youtube=%q", trackItem.ItemID, trackItem.YouTubeURL)
	}
	if trackItem.FilePath != "Artist/Album/01 - Title.mp3" {
		t.Errorf("track item FilePath = %q", trackItem.FilePath)
	}
	var playlistItem *PlanItem
	for _, it := range back.Items {
		if it.ItemType == PlanItemTypePlaylist {
			playlistItem = it
			break
		}
	}
	if playlistItem == nil {
		t.Fatal("no playlist item after round-trip")
	}
	if playlistItem.ItemID != "playlist-1" {
		t.Errorf("playlist ItemID = %q, want playlist-1 (stable ID from spec)", playlistItem.ItemID)
	}
}

func TestPlanToSpecAndSpecToPlan_M3URoundTrip(t *testing.T) {
	plan := NewDownloadPlan(map[string]interface{}{"config_version": "1.2"})
	plan.AddItem(&PlanItem{
		ItemID:     "track:abc",
		ItemType:   PlanItemTypeTrack,
		SpotifyURL: "https://open.spotify.com/track/abc",
		FilePath:   "Artist/Album/01 - Title.mp3",
		Status:     PlanItemStatusPending,
		CreatedAt:  time.Now(),
		Progress:   0,
	})
	plan.AddItem(&PlanItem{
		ItemID:     "playlist:xyz",
		ItemType:   PlanItemTypePlaylist,
		Name:       "My Playlist",
		SpotifyURL: "https://open.spotify.com/playlist/xyz",
		ChildIDs:   []string{"track:abc", "m3u:xyz"},
		Status:     PlanItemStatusPending,
		Metadata:   map[string]interface{}{"create_m3u": true},
		CreatedAt:  time.Now(),
		Progress:   0,
	})
	plan.AddItem(&PlanItem{
		ItemID:     "m3u:xyz",
		ItemType:   PlanItemTypeM3U,
		ParentID:   "playlist:xyz",
		Name:       "My Playlist.m3u",
		Status:     PlanItemStatusPending,
		Metadata:   map[string]interface{}{"playlist_name": "My Playlist"},
		CreatedAt:  time.Now(),
		Progress:   0,
	})

	spec := PlanToSpec(plan, "hash1", "config.yml", time.Now().UTC())
	if len(spec.M3Us) != 1 {
		t.Fatalf("spec.M3Us len = %d, want 1", len(spec.M3Us))
	}
	if spec.M3Us[0].ID != "m3u:xyz" || spec.M3Us[0].ParentID != "playlist:xyz" || spec.M3Us[0].PlaylistName != "My Playlist" {
		t.Errorf("spec.M3Us[0] = id=%q parent_id=%q playlist_name=%q", spec.M3Us[0].ID, spec.M3Us[0].ParentID, spec.M3Us[0].PlaylistName)
	}
	if len(spec.Playlists) != 1 || spec.Playlists[0].ID != "playlist:xyz" {
		t.Errorf("spec.Playlists[0].ID = %q, want playlist:xyz", spec.Playlists[0].ID)
	}
	if len(spec.Playlists[0].TrackIDs) != 1 || spec.Playlists[0].TrackIDs[0] != "track:abc" {
		t.Errorf("playlist TrackIDs = %v, want [track:abc]", spec.Playlists[0].TrackIDs)
	}

	back, err := SpecToPlan(spec)
	if err != nil {
		t.Fatalf("SpecToPlan: %v", err)
	}
	m3uItems := back.GetItemsByType(PlanItemTypeM3U)
	if len(m3uItems) != 1 {
		t.Fatalf("after round-trip M3U items len = %d, want 1", len(m3uItems))
	}
	m3u := m3uItems[0]
	if m3u.ItemID != "m3u:xyz" || m3u.ParentID != "playlist:xyz" || m3u.Name != "My Playlist.m3u" {
		t.Errorf("M3U item: id=%q parent_id=%q name=%q", m3u.ItemID, m3u.ParentID, m3u.Name)
	}
	if back.GetItem(m3u.ParentID) == nil {
		t.Error("M3U ParentID should resolve to playlist item (processM3UFiles needs it)")
	}
	playlistName, _ := m3u.Metadata["playlist_name"].(string)
	if playlistName != "My Playlist" {
		t.Errorf("M3U Metadata playlist_name = %q, want My Playlist", playlistName)
	}
}

func TestPlanToSpec_AlbumM3URoundTrip(t *testing.T) {
	plan := NewDownloadPlan(nil)
	plan.AddItem(&PlanItem{
		ItemID:     "m3u:album:aid",
		ItemType:   PlanItemTypeM3U,
		ParentID:   "album:aid",
		Name:       "Album Name.m3u",
		Status:     PlanItemStatusPending,
		Metadata:   map[string]interface{}{"album_name": "Album Name"},
		CreatedAt:  time.Now(),
		Progress:   0,
	})
	spec := PlanToSpec(plan, "h", "c.yml", time.Now().UTC())
	if len(spec.M3Us) != 1 {
		t.Fatalf("expected 1 M3U (album M3U round-trip), got len(M3Us)=%d", len(spec.M3Us))
	}
	if spec.M3Us[0].AlbumName != "Album Name" {
		t.Errorf("expected AlbumName=Album Name, got %q", spec.M3Us[0].AlbumName)
	}
	if spec.M3Us[0].ParentID != "album:aid" {
		t.Errorf("expected ParentID=album:aid, got %q", spec.M3Us[0].ParentID)
	}
}

func TestLoadPlanByHash_FileNotFound(t *testing.T) {
	dir := t.TempDir()
	_, err := LoadPlanByHash(dir, "nonexistent_hash")
	if err == nil {
		t.Error("LoadPlanByHash: expected error when file does not exist")
	}
	if !errors.Is(err, ErrPlanNotFound) && !os.IsNotExist(err) {
		t.Errorf("LoadPlanByHash: expected ErrPlanNotFound or IsNotExist, got %v", err)
	}
}

func TestLoadPlanByHash_HashMismatch(t *testing.T) {
	dir := t.TempDir()
	path := GetPlanFilePath(dir, "abc123")
	spec := &SpecPlan{
		ConfigHash:  "wrong_hash",
		ConfigFile:  "config.yml",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		TotalTracks: 0,
		Downloads:   nil,
		Playlists:   nil,
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := SaveSpecPlan(spec, path); err != nil {
		t.Fatalf("SaveSpecPlan: %v", err)
	}

	_, err := LoadPlanByHash(dir, "abc123")
	if err == nil {
		t.Error("LoadPlanByHash: expected error when config_hash does not match")
	}
	if !errors.Is(err, ErrPlanHashMismatch) {
		t.Errorf("LoadPlanByHash: expected ErrPlanHashMismatch, got %v", err)
	}
}

func TestSavePlanByHashAndLoadPlanByHash(t *testing.T) {
	dir := t.TempDir()
	configHash := "deadbeef12345678"
	configFile := "musicdl-config.yml"

	plan := NewDownloadPlan(nil)
	plan.AddItem(&PlanItem{
		ItemID:     "t1",
		ItemType:   PlanItemTypeTrack,
		YouTubeURL: "https://youtube.com/watch?v=x",
		FilePath:   "out.mp3",
		Status:     PlanItemStatusPending,
		CreatedAt:  time.Now(),
		Progress:   0,
	})

	if err := SavePlanByHash(plan, dir, configHash, configFile); err != nil {
		t.Fatalf("SavePlanByHash: %v", err)
	}

	loaded, err := LoadPlanByHash(dir, configHash)
	if err != nil {
		t.Fatalf("LoadPlanByHash: %v", err)
	}
	if len(loaded.Items) != 1 {
		t.Errorf("loaded plan Items len = %d, want 1", len(loaded.Items))
	}
	if loaded.Items[0].ItemID != "t1" {
		t.Errorf("loaded ItemID = %q, want t1", loaded.Items[0].ItemID)
	}
}

func TestSpecToPlan_MetadataOnlyStatus(t *testing.T) {
	spec := &SpecPlan{
		ConfigHash:  "abc",
		ConfigFile:  "config.yml",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		TotalTracks: 1,
		Downloads: []SpecDownloadItem{
			{
				ID:         "id1",
				YouTubeURL: "https://youtube.com/watch?v=a",
				OutputPath: "x.mp3",
				Status:     "metadata_only",
			},
		},
		Playlists: nil,
	}
	plan, err := SpecToPlan(spec)
	if err != nil {
		t.Fatalf("SpecToPlan: %v", err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("Items len = %d, want 1", len(plan.Items))
	}
	if plan.Items[0].Status != PlanItemStatusSkipped {
		t.Errorf("metadata_only should map to skipped, got %q", plan.Items[0].Status)
	}
}

func TestPlanToSpec_ErrorAndRawOutput(t *testing.T) {
	plan := NewDownloadPlan(map[string]interface{}{"config_version": "1.2"})
	plan.AddItem(&PlanItem{
		ItemID:     "track-1",
		ItemType:   PlanItemTypeTrack,
		YouTubeURL: "https://www.youtube.com/watch?v=abc123",
		SpotifyID:  "xyz",
		FilePath:   "Artist/Album/01 - Title.mp3",
		Status:     PlanItemStatusFailed,
		Error:      "download failed: 429 rate limited",
		RawOutput:  "ERROR: [youtube] abc123: HTTP Error 429: Too Many Requests",
		Metadata:   nil,
		CreatedAt:  time.Now(),
		Progress:   0,
	})

	spec := PlanToSpec(plan, "hash1", "config.yml", time.Now().UTC())
	if len(spec.Downloads) != 1 {
		t.Fatalf("spec.Downloads len = %d, want 1", len(spec.Downloads))
	}
	di := spec.Downloads[0]
	if di.Error != "download failed: 429 rate limited" {
		t.Errorf("spec.Downloads[0].Error = %q, want %q", di.Error, "download failed: 429 rate limited")
	}
	if !strings.Contains(di.RawOutput, "HTTP Error 429") {
		t.Errorf("spec.Downloads[0].RawOutput = %q, want to contain %q", di.RawOutput, "HTTP Error 429")
	}
}

func TestSpecToPlan_ErrorAndRawOutput(t *testing.T) {
	spec := &SpecPlan{
		ConfigHash:  "abc",
		ConfigFile:  "config.yml",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		TotalTracks: 1,
		Downloads: []SpecDownloadItem{
			{
				ID:         "id1",
				YouTubeURL: "https://youtube.com/watch?v=a",
				OutputPath: "x.mp3",
				Status:     "failed",
				Error:      "download failed: 429 rate limited",
				RawOutput:  "ERROR: [youtube] a: HTTP Error 429: Too Many Requests",
			},
		},
		Playlists: nil,
	}
	plan, err := SpecToPlan(spec)
	if err != nil {
		t.Fatalf("SpecToPlan: %v", err)
	}
	if len(plan.Items) != 1 {
		t.Fatalf("Items len = %d, want 1", len(plan.Items))
	}
	item := plan.Items[0]
	if item.Error != "download failed: 429 rate limited" {
		t.Errorf("plan.Items[0].Error = %q, want %q", item.Error, "download failed: 429 rate limited")
	}
	if item.RawOutput != "ERROR: [youtube] a: HTTP Error 429: Too Many Requests" {
		t.Errorf("plan.Items[0].RawOutput = %q, want %q", item.RawOutput, "ERROR: [youtube] a: HTTP Error 429: Too Many Requests")
	}
}

func TestPlanToSpecAndSpecToPlan_RoundTrip_WithErrorAndRawOutput(t *testing.T) {
	plan := NewDownloadPlan(map[string]interface{}{"config_version": "1.2"})
	plan.AddItem(&PlanItem{
		ItemID:     "track-1",
		ItemType:   PlanItemTypeTrack,
		YouTubeURL: "https://www.youtube.com/watch?v=abc",
		FilePath:   "Artist/Album/01 - Title.mp3",
		Status:     PlanItemStatusFailed,
		Error:      "download failed: 429 rate limited",
		RawOutput:  "ERROR: [youtube] abc: HTTP Error 429: Too Many Requests",
		CreatedAt:  time.Now(),
		Progress:   0,
	})

	configHash := "roundtrip123"
	configFile := "config.yml"
	generatedAt := time.Now().UTC()

	spec := PlanToSpec(plan, configHash, configFile, generatedAt)
	back, err := SpecToPlan(spec)
	if err != nil {
		t.Fatalf("SpecToPlan: %v", err)
	}
	if len(back.Items) != 1 {
		t.Fatalf("after round-trip Items len = %d, want 1", len(back.Items))
	}
	item := back.Items[0]
	if item.Error != "download failed: 429 rate limited" {
		t.Errorf("after round-trip Error = %q, want %q", item.Error, "download failed: 429 rate limited")
	}
	if item.RawOutput != "ERROR: [youtube] abc: HTTP Error 429: Too Many Requests" {
		t.Errorf("after round-trip RawOutput = %q, want %q", item.RawOutput, "ERROR: [youtube] abc: HTTP Error 429: Too Many Requests")
	}

	dir := t.TempDir()
	if err := SavePlanByHash(plan, dir, configHash, configFile); err != nil {
		t.Fatalf("SavePlanByHash: %v", err)
	}
	loaded, err := LoadPlanByHash(dir, configHash)
	if err != nil {
		t.Fatalf("LoadPlanByHash: %v", err)
	}
	trackItems := loaded.GetItemsByType(PlanItemTypeTrack)
	if len(trackItems) != 1 {
		t.Fatalf("loaded plan track items len = %d, want 1", len(trackItems))
	}
	loadedItem := trackItems[0]
	if loadedItem.Error != "download failed: 429 rate limited" {
		t.Errorf("after disk load Error = %q, want %q", loadedItem.Error, "download failed: 429 rate limited")
	}
	if loadedItem.RawOutput != "ERROR: [youtube] abc: HTTP Error 429: Too Many Requests" {
		t.Errorf("after disk load RawOutput = %q, want %q", loadedItem.RawOutput, "ERROR: [youtube] abc: HTTP Error 429: Too Many Requests")
	}
}

func TestPlanCacheFile_FreshnessCheck(t *testing.T) {
	tmpDir := t.TempDir()
	configHash := "cache_test_hash"
	cacheTTL := 24 * time.Hour

	// Create and save a plan
	dp := NewDownloadPlan(nil)
	dp.AddItem(&PlanItem{
		ItemID:   "track:cache1",
		ItemType: PlanItemTypeTrack,
		Name:     "Cached Track",
		Status:   PlanItemStatusPending,
	})
	if err := SavePlanByHash(dp, tmpDir, configHash, "test.yml"); err != nil {
		t.Fatalf("SavePlanByHash failed: %v", err)
	}

	// Check freshness: file just created should be fresh
	planPath := GetPlanFilePath(tmpDir, configHash)
	info, err := os.Stat(planPath)
	if err != nil {
		t.Fatalf("os.Stat failed: %v", err)
	}
	if time.Since(info.ModTime()) >= cacheTTL {
		t.Error("Freshly created plan file should be within TTL")
	}

	// Load the cached plan
	loaded, err := LoadPlanByHash(tmpDir, configHash)
	if err != nil {
		t.Fatalf("LoadPlanByHash failed: %v", err)
	}

	// Verify the loaded plan matches
	trackCount := 0
	for _, item := range loaded.Items {
		if item.ItemType == PlanItemTypeTrack {
			trackCount++
		}
	}
	if trackCount != 1 {
		t.Errorf("Expected 1 track in loaded plan, got %d", trackCount)
	}
}

func TestPlanCacheFile_StaleCheck(t *testing.T) {
	tmpDir := t.TempDir()
	configHash := "stale_test_hash"

	// Create and save a plan
	dp := NewDownloadPlan(nil)
	dp.AddItem(&PlanItem{
		ItemID:   "track:stale1",
		ItemType: PlanItemTypeTrack,
		Name:     "Stale Track",
		Status:   PlanItemStatusPending,
	})
	if err := SavePlanByHash(dp, tmpDir, configHash, "test.yml"); err != nil {
		t.Fatalf("SavePlanByHash failed: %v", err)
	}

	// Backdate the file's modification time to simulate staleness
	planPath := GetPlanFilePath(tmpDir, configHash)
	oldTime := time.Now().Add(-25 * time.Hour) // 25 hours ago
	if err := os.Chtimes(planPath, oldTime, oldTime); err != nil {
		t.Fatalf("os.Chtimes failed: %v", err)
	}

	// Check freshness: file should now be stale
	info, err := os.Stat(planPath)
	if err != nil {
		t.Fatalf("os.Stat failed: %v", err)
	}
	cacheTTL := 24 * time.Hour
	if time.Since(info.ModTime()) < cacheTTL {
		t.Error("Backdated plan file should be outside TTL")
	}
}

// --- Bug 1: TrackMetadata round-trip preserves all metadata fields ---

func TestTrackMetadataRoundTrip(t *testing.T) {
	plan := NewDownloadPlan(nil)
	plan.AddItem(&PlanItem{
		ItemID:     "track:abc",
		ItemType:   PlanItemTypeTrack,
		SpotifyID:  "abc",
		SpotifyURL: "https://open.spotify.com/track/abc",
		YouTubeURL: "https://www.youtube.com/watch?v=xyz",
		Name:       "Test Track",
		FilePath:   "Artist/Album/01 - Test Track.mp3",
		Status:     PlanItemStatusPending,
		Metadata: map[string]interface{}{
			"artist":              "Test Artist",
			"title":               "Test Track",
			"album":               "Test Album",
			"track_number":        float64(1),
			"disc_number":         float64(1),
			"duration_ms":         float64(240000),
			"spotify_enhancement": true,
		},
	})

	spec := PlanToSpec(plan, "hash", "config.yml", time.Now().UTC())
	if len(spec.Downloads) != 1 {
		t.Fatalf("spec.Downloads len = %d, want 1", len(spec.Downloads))
	}
	di := spec.Downloads[0]
	if di.TrackMetadata == nil {
		t.Fatal("TrackMetadata should not be nil after PlanToSpec")
	}
	if di.TrackMetadata["artist"] != "Test Artist" {
		t.Errorf("TrackMetadata[artist] = %v, want Test Artist", di.TrackMetadata["artist"])
	}
	if di.TrackMetadata["album"] != "Test Album" {
		t.Errorf("TrackMetadata[album] = %v, want Test Album", di.TrackMetadata["album"])
	}

	back, err := SpecToPlan(spec)
	if err != nil {
		t.Fatalf("SpecToPlan: %v", err)
	}
	trackItems := back.GetItemsByType(PlanItemTypeTrack)
	if len(trackItems) != 1 {
		t.Fatalf("round-trip track items = %d, want 1", len(trackItems))
	}
	track := trackItems[0]
	if v, _ := track.Metadata["artist"].(string); v != "Test Artist" {
		t.Errorf("round-trip Metadata[artist] = %v, want Test Artist", track.Metadata["artist"])
	}
	if v, _ := track.Metadata["album"].(string); v != "Test Album" {
		t.Errorf("round-trip Metadata[album] = %v, want Test Album", track.Metadata["album"])
	}
	if v, _ := track.Metadata["track_number"].(float64); v != 1 {
		t.Errorf("round-trip Metadata[track_number] = %v, want 1", track.Metadata["track_number"])
	}
	if v, _ := track.Metadata["spotify_enhancement"].(bool); !v {
		t.Errorf("round-trip Metadata[spotify_enhancement] = %v, want true", track.Metadata["spotify_enhancement"])
	}
}

func TestStructMetadataSurvivesRoundTrip(t *testing.T) {
	type fakeYTMeta struct {
		VideoID  string `json:"video_id"`
		Title    string `json:"title"`
		Duration int    `json:"duration"`
	}
	plan := NewDownloadPlan(nil)
	plan.AddItem(&PlanItem{
		ItemID:     "track:yt1",
		ItemType:   PlanItemTypeTrack,
		YouTubeURL: "https://www.youtube.com/watch?v=vid1",
		Name:       "YT Track",
		Status:     PlanItemStatusPending,
		Metadata: map[string]interface{}{
			"youtube_metadata": &fakeYTMeta{VideoID: "vid1", Title: "YT Track", Duration: 300},
			"artist":           "Some Artist",
		},
	})

	spec := PlanToSpec(plan, "hash", "c.yml", time.Now().UTC())
	if spec.Downloads[0].TrackMetadata == nil {
		t.Fatal("TrackMetadata should not be nil for struct metadata")
	}
	ytm, ok := spec.Downloads[0].TrackMetadata["youtube_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("youtube_metadata should be a map after serialization, got %T", spec.Downloads[0].TrackMetadata["youtube_metadata"])
	}
	if ytm["video_id"] != "vid1" {
		t.Errorf("youtube_metadata.video_id = %v, want vid1", ytm["video_id"])
	}

	back, err := SpecToPlan(spec)
	if err != nil {
		t.Fatalf("SpecToPlan: %v", err)
	}
	track := back.GetItemsByType(PlanItemTypeTrack)[0]
	ytm2, ok := track.Metadata["youtube_metadata"].(map[string]interface{})
	if !ok {
		t.Fatalf("round-trip youtube_metadata type = %T, want map[string]interface{}", track.Metadata["youtube_metadata"])
	}
	if ytm2["video_id"] != "vid1" {
		t.Errorf("round-trip youtube_metadata.video_id = %v, want vid1", ytm2["video_id"])
	}
}

func TestLegacySpotifyMetadataFallback(t *testing.T) {
	spec := &SpecPlan{
		ConfigHash:  "abc",
		ConfigFile:  "config.yml",
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		TotalTracks: 1,
		Downloads: []SpecDownloadItem{
			{
				ID:         "track:legacy",
				YouTubeURL: "https://youtube.com/watch?v=old",
				OutputPath: "x.mp3",
				Status:     "pending",
				SpotifyMetadata: map[string]interface{}{
					"title":  "Legacy Title",
					"artist": "Legacy Artist",
				},
				YouTubeMetadata: map[string]interface{}{
					"video_id": "old",
				},
			},
		},
	}

	plan, err := SpecToPlan(spec)
	if err != nil {
		t.Fatalf("SpecToPlan: %v", err)
	}
	track := plan.Items[0]
	sm, ok := track.Metadata["spotify_metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("expected legacy spotify_metadata fallback")
	}
	if sm["title"] != "Legacy Title" {
		t.Errorf("legacy spotify_metadata.title = %v, want Legacy Title", sm["title"])
	}
	ym, ok := track.Metadata["youtube_metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("expected legacy youtube_metadata fallback")
	}
	if ym["video_id"] != "old" {
		t.Errorf("legacy youtube_metadata.video_id = %v, want old", ym["video_id"])
	}
}

// --- Bug 3: Album and Artist container items round-trip ---

func TestContainerRoundTrip(t *testing.T) {
	plan := NewDownloadPlan(nil)
	plan.AddItem(&PlanItem{
		ItemID:    "track:t1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "t1",
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
	})
	plan.AddItem(&PlanItem{
		ItemID:     "album:a1",
		ItemType:   PlanItemTypeAlbum,
		SpotifyURL: "https://open.spotify.com/album/a1",
		Name:       "Test Album",
		ChildIDs:   []string{"track:t1"},
		Status:     PlanItemStatusPending,
		Metadata:   map[string]interface{}{"release_date": "2024-01-01", "create_m3u": true},
	})
	plan.AddItem(&PlanItem{
		ItemID:     "artist:ar1",
		ItemType:   PlanItemTypeArtist,
		SpotifyURL: "https://open.spotify.com/artist/ar1",
		Name:       "Test Artist",
		ChildIDs:   []string{"album:a1"},
		Status:     PlanItemStatusPending,
		Metadata:   map[string]interface{}{"genres": "rock"},
	})

	spec := PlanToSpec(plan, "hash", "c.yml", time.Now().UTC())
	if len(spec.Containers) != 2 {
		t.Fatalf("spec.Containers len = %d, want 2", len(spec.Containers))
	}

	albumFound := false
	artistFound := false
	for _, c := range spec.Containers {
		switch c.Type {
		case "album":
			albumFound = true
			if c.Name != "Test Album" {
				t.Errorf("album container name = %q, want Test Album", c.Name)
			}
			if len(c.ChildIDs) != 1 || c.ChildIDs[0] != "track:t1" {
				t.Errorf("album container ChildIDs = %v, want [track:t1]", c.ChildIDs)
			}
		case "artist":
			artistFound = true
			if c.Name != "Test Artist" {
				t.Errorf("artist container name = %q, want Test Artist", c.Name)
			}
		}
	}
	if !albumFound {
		t.Error("album container not found in spec")
	}
	if !artistFound {
		t.Error("artist container not found in spec")
	}

	back, err := SpecToPlan(spec)
	if err != nil {
		t.Fatalf("SpecToPlan: %v", err)
	}

	albumItems := back.GetItemsByType(PlanItemTypeAlbum)
	if len(albumItems) != 1 {
		t.Fatalf("round-trip album items = %d, want 1", len(albumItems))
	}
	if albumItems[0].Name != "Test Album" {
		t.Errorf("round-trip album name = %q, want Test Album", albumItems[0].Name)
	}
	if len(albumItems[0].ChildIDs) != 1 {
		t.Errorf("round-trip album ChildIDs len = %d, want 1", len(albumItems[0].ChildIDs))
	}

	artistItems := back.GetItemsByType(PlanItemTypeArtist)
	if len(artistItems) != 1 {
		t.Fatalf("round-trip artist items = %d, want 1", len(artistItems))
	}
	if artistItems[0].Name != "Test Artist" {
		t.Errorf("round-trip artist name = %q, want Test Artist", artistItems[0].Name)
	}
}

func TestContainersDiskRoundTrip(t *testing.T) {
	dir := t.TempDir()
	configHash := "container_disk_hash"

	plan := NewDownloadPlan(nil)
	plan.AddItem(&PlanItem{
		ItemID:   "track:t1",
		ItemType: PlanItemTypeTrack,
		Name:     "Track 1",
		Status:   PlanItemStatusPending,
	})
	plan.AddItem(&PlanItem{
		ItemID:   "album:a1",
		ItemType: PlanItemTypeAlbum,
		Name:     "Disk Album",
		ChildIDs: []string{"track:t1"},
		Status:   PlanItemStatusPending,
	})

	if err := SavePlanByHash(plan, dir, configHash, "config.yml"); err != nil {
		t.Fatalf("SavePlanByHash: %v", err)
	}
	loaded, err := LoadPlanByHash(dir, configHash)
	if err != nil {
		t.Fatalf("LoadPlanByHash: %v", err)
	}

	albumItems := loaded.GetItemsByType(PlanItemTypeAlbum)
	if len(albumItems) != 1 {
		t.Fatalf("disk round-trip album items = %d, want 1", len(albumItems))
	}
	if albumItems[0].Name != "Disk Album" {
		t.Errorf("disk round-trip album name = %q, want Disk Album", albumItems[0].Name)
	}
	if len(albumItems[0].ChildIDs) != 1 || albumItems[0].ChildIDs[0] != "track:t1" {
		t.Errorf("disk round-trip album ChildIDs = %v, want [track:t1]", albumItems[0].ChildIDs)
	}
}

func TestSpecRoundTrip_DirectSourceItems(t *testing.T) {
	plan := NewDownloadPlan(map[string]interface{}{
		"config_version": "1.2",
	})

	plan.AddItem(&PlanItem{
		ItemID:    "track:soundcloud:artist/track",
		ItemType:  PlanItemTypeTrack,
		Name:      "SC Track",
		Source:    SourceTypeSoundCloud,
		SourceURL: "https://soundcloud.com/artist/track",
		Status:    PlanItemStatusPending,
		Metadata:  map[string]interface{}{"uploader": "SC Artist"},
	})
	plan.AddItem(&PlanItem{
		ItemID:    "track:bandcamp:artist/song",
		ItemType:  PlanItemTypeTrack,
		Name:      "BC Track",
		Source:    SourceTypeBandcamp,
		SourceURL: "https://artist.bandcamp.com/track/song",
		Status:    PlanItemStatusCompleted,
		FilePath:  "/music/artist/song.mp3",
		Metadata:  map[string]interface{}{"uploader": "BC Artist"},
	})
	plan.AddItem(&PlanItem{
		ItemID:    "track:audius:artist/track",
		ItemType:  PlanItemTypeTrack,
		Name:      "Audius Track",
		Source:    SourceTypeAudius,
		SourceURL: "https://audius.co/artist/track",
		Status:    PlanItemStatusPending,
	})
	plan.AddItem(&PlanItem{
		ItemID:    "playlist:soundcloud:artist/sets/my-set",
		ItemType:  PlanItemTypePlaylist,
		Name:      "SC Set",
		SourceURL: "https://soundcloud.com/artist/sets/my-set",
		ChildIDs:  []string{"track:soundcloud:artist/track"},
		Status:    PlanItemStatusPending,
		Metadata:  map[string]interface{}{"create_m3u": true},
	})

	spec := PlanToSpec(plan, "abc123", "config.yaml", time.Now())

	if len(spec.Downloads) != 3 {
		t.Fatalf("expected 3 downloads, got %d", len(spec.Downloads))
	}
	if spec.Downloads[0].Source != "soundcloud" {
		t.Errorf("expected source 'soundcloud', got %q", spec.Downloads[0].Source)
	}
	if spec.Downloads[0].SourceURL != "https://soundcloud.com/artist/track" {
		t.Errorf("expected SourceURL preserved, got %q", spec.Downloads[0].SourceURL)
	}
	if spec.Downloads[1].Source != "bandcamp" {
		t.Errorf("expected source 'bandcamp', got %q", spec.Downloads[1].Source)
	}

	restored, err := SpecToPlan(spec)
	if err != nil {
		t.Fatalf("SpecToPlan() error: %v", err)
	}

	tracks := restored.GetItemsByType(PlanItemTypeTrack)
	if len(tracks) != 3 {
		t.Fatalf("expected 3 tracks after restore, got %d", len(tracks))
	}

	for _, tr := range tracks {
		switch tr.ItemID {
		case "track:soundcloud:artist/track":
			if tr.Source != SourceTypeSoundCloud {
				t.Errorf("expected source %q, got %q", SourceTypeSoundCloud, tr.Source)
			}
			if tr.SourceURL != "https://soundcloud.com/artist/track" {
				t.Errorf("expected SourceURL preserved, got %q", tr.SourceURL)
			}
			if tr.DownloadURL() != "https://soundcloud.com/artist/track" {
				t.Errorf("expected DownloadURL() to return SourceURL, got %q", tr.DownloadURL())
			}
		case "track:bandcamp:artist/song":
			if tr.Source != SourceTypeBandcamp {
				t.Errorf("expected source %q, got %q", SourceTypeBandcamp, tr.Source)
			}
			if tr.Status != PlanItemStatusCompleted {
				t.Errorf("expected status completed, got %q", tr.Status)
			}
		case "track:audius:artist/track":
			if tr.Source != SourceTypeAudius {
				t.Errorf("expected source %q, got %q", SourceTypeAudius, tr.Source)
			}
		default:
			t.Errorf("unexpected track ID: %s", tr.ItemID)
		}
	}

	playlists := restored.GetItemsByType(PlanItemTypePlaylist)
	if len(playlists) != 1 {
		t.Fatalf("expected 1 playlist, got %d", len(playlists))
	}
	if playlists[0].SourceURL != "https://soundcloud.com/artist/sets/my-set" {
		t.Errorf("expected playlist SourceURL preserved, got %q", playlists[0].SourceURL)
	}
}
