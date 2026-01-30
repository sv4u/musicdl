package plan

import (
	"errors"
	"os"
	"path/filepath"
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
