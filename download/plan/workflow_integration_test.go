package plan

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/spotigo"
)

func TestPlanWorkflow_GenerateOptimizeExecute_SingleSong(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
		},
		Songs: []config.MusicSource{
			{Name: "Test Song", URL: "https://open.spotify.com/track/track123"},
		},
	}

	mockClient := newMockSpotifyClient()
	mockClient.tracks["track123"] = createMockTrack("track123", "Test Song", "Test Artist")

	playlistTracksFunc := func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
		return nil, nil
	}

	generator := NewGenerator(cfg, mockClient, playlistTracksFunc, nil)

	// Step 1: Generate plan
	plan, err := generator.GeneratePlan(context.Background())
	if err != nil {
		t.Fatalf("GeneratePlan() failed: %v", err)
	}
	if plan == nil {
		t.Fatal("GeneratePlan() returned nil plan")
	}

	// Verify plan has track
	if len(plan.Items) != 1 {
		t.Fatalf("Expected 1 item in plan, got %d", len(plan.Items))
	}
	if plan.Items[0].ItemType != PlanItemTypeTrack {
		t.Errorf("Expected track item, got %s", plan.Items[0].ItemType)
	}

	// Step 2: Optimize plan
	optimizer := NewOptimizer(true)
	optimizer.Optimize(plan)

	// Plan should still have track (no duplicates to remove)
	if len(plan.Items) != 1 {
		t.Errorf("Expected 1 item after optimization, got %d", len(plan.Items))
	}

	// Step 3: Execute plan (with mock downloader)
	mockDownloader := &mockWorkflowDownloader{
		downloadResults: make(map[string]mockDownloadResult),
	}
	executor := NewExecutor(mockDownloader, 1)

	// Set up mock to succeed
	mockDownloader.downloadResults["https://open.spotify.com/track/track123"] = mockDownloadResult{
		success:  true,
		filePath: "/tmp/test.mp3",
		err:      nil,
	}

	stats, err := executor.Execute(context.Background(), plan, func(item *PlanItem) {
		// Progress callback
	})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	// Verify execution stats
	if stats["completed"] != 1 {
		t.Errorf("Expected 1 completed track, got %d", stats["completed"])
	}

	// Verify track status
	if plan.Items[0].Status != PlanItemStatusCompleted {
		t.Errorf("Expected track status COMPLETED, got %s", plan.Items[0].Status)
	}
}

func TestPlanWorkflow_GenerateOptimizeExecute_WithDuplicates(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
		},
		Songs: []config.MusicSource{
			{Name: "Test Song", URL: "https://open.spotify.com/track/track123"},
			{Name: "Test Song Duplicate", URL: "https://open.spotify.com/track/track123"}, // Same track
		},
	}

	mockClient := newMockSpotifyClient()
	mockClient.tracks["track123"] = createMockTrack("track123", "Test Song", "Test Artist")

	playlistTracksFunc := func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
		return nil, nil
	}

	generator := NewGenerator(cfg, mockClient, playlistTracksFunc, nil)

	// Step 1: Generate plan
	plan, err := generator.GeneratePlan(context.Background())
	if err != nil {
		t.Fatalf("GeneratePlan() failed: %v", err)
	}

	// Should have only 1 track (duplicate skipped during generation)
	if len(plan.Items) != 1 {
		t.Fatalf("Expected 1 item (duplicate skipped), got %d", len(plan.Items))
	}

	// Step 2: Optimize plan (should still have 1 track)
	optimizer := NewOptimizer(true)
	optimizer.Optimize(plan)

	if len(plan.Items) != 1 {
		t.Errorf("Expected 1 item after optimization, got %d", len(plan.Items))
	}

	// Step 3: Execute plan
	mockDownloader := &mockWorkflowDownloader{
		downloadResults: make(map[string]mockDownloadResult),
	}
	executor := NewExecutor(mockDownloader, 1)

	mockDownloader.downloadResults["https://open.spotify.com/track/track123"] = mockDownloadResult{
		success:  true,
		filePath: "/tmp/test.mp3",
		err:      nil,
	}

	stats, err := executor.Execute(context.Background(), plan, func(item *PlanItem) {
		// Progress callback
	})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	if stats["completed"] != 1 {
		t.Errorf("Expected 1 completed track, got %d", stats["completed"])
	}
}

func TestPlanWorkflow_GenerateOptimizeExecute_WithArtist(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.2",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
		},
		Artists: []config.MusicSource{
			{Name: "Test Artist", URL: "https://open.spotify.com/artist/artist1"},
		},
	}

	mockClient := newMockSpotifyClient()
	mockClient.artists["artist1"] = createMockArtist("artist1", "Test Artist")
	mockClient.artistAlbums["artist1"] = []spotigo.SimplifiedAlbum{
		createMockSimplifiedAlbum("album1", "Test Album", "Test Artist"),
	}

	album := createMockAlbum("album1", "Test Album", "Test Artist", 2)
	album.Tracks = &spotigo.Paging[spotigo.SimplifiedTrack]{
		Items: []spotigo.SimplifiedTrack{
			createMockSimplifiedTrack("track1", "Track 1", "Test Artist"),
			createMockSimplifiedTrack("track2", "Track 2", "Test Artist"),
		},
	}
	mockClient.albums["album1"] = album

	playlistTracksFunc := func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
		return nil, nil
	}

	generator := NewGenerator(cfg, mockClient, playlistTracksFunc, nil)

	// Step 1: Generate plan
	plan, err := generator.GeneratePlan(context.Background())
	if err != nil {
		t.Fatalf("GeneratePlan() failed: %v", err)
	}

	// Should have: artist + album + 2 tracks = 4 items
	if len(plan.Items) < 4 {
		t.Fatalf("Expected at least 4 items (artist + album + 2 tracks), got %d", len(plan.Items))
	}

	// Step 2: Optimize plan
	optimizer := NewOptimizer(true)
	optimizer.Optimize(plan)

	// Should still have same items (no duplicates)
	if len(plan.Items) < 4 {
		t.Errorf("Expected at least 4 items after optimization, got %d", len(plan.Items))
	}

	// Step 3: Execute plan
	mockDownloader := &mockWorkflowDownloader{
		downloadResults: make(map[string]mockDownloadResult),
	}
	executor := NewExecutor(mockDownloader, 2)

	// Set up mocks for both tracks
	mockDownloader.downloadResults["https://open.spotify.com/track/track1"] = mockDownloadResult{
		success:  true,
		filePath: "/tmp/track1.mp3",
		err:      nil,
	}
	mockDownloader.downloadResults["https://open.spotify.com/track/track2"] = mockDownloadResult{
		success:  true,
		filePath: "/tmp/track2.mp3",
		err:      nil,
	}

	stats, err := executor.Execute(context.Background(), plan, func(item *PlanItem) {
		// Progress callback
	})

	if err != nil {
		t.Fatalf("Execute() failed: %v", err)
	}

	// Should have completed 2 tracks
	if stats["completed"] != 2 {
		t.Errorf("Expected 2 completed tracks, got %d", stats["completed"])
	}
}

func TestPlanWorkflow_PlanPersistence_SaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "download_plan_progress.json")

	// Create a plan
	plan := NewDownloadPlan(map[string]interface{}{
		"test": "value",
	})

	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123",
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track1)

	track2 := &PlanItem{
		ItemID:    "track:2",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_456",
		Name:      "Track 2",
		Status:    PlanItemStatusCompleted,
	}
	plan.AddItem(track2)

	// Save plan
	if err := plan.Save(planPath); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(planPath); err != nil {
		t.Fatalf("Plan file was not created: %v", err)
	}

	// Load plan
	loadedPlan, err := LoadPlan(planPath)
	if err != nil {
		t.Fatalf("LoadPlan() failed: %v", err)
	}

	if loadedPlan == nil {
		t.Fatal("LoadPlan() returned nil plan")
	}

	// Verify plan structure
	if len(loadedPlan.Items) != 2 {
		t.Errorf("Expected 2 items in loaded plan, got %d", len(loadedPlan.Items))
	}

	// Verify metadata
	if loadedPlan.Metadata["test"] != "value" {
		t.Errorf("Expected metadata 'test'='value', got '%v'", loadedPlan.Metadata["test"])
	}

	// Verify items
	var track1Item, track2Item *PlanItem
	for _, item := range loadedPlan.Items {
		if item.ItemID == "track:1" {
			track1Item = item
		}
		if item.ItemID == "track:2" {
			track2Item = item
		}
	}

	if track1Item == nil {
		t.Fatal("Expected track:1 in loaded plan")
	}
	if track1Item.Status != PlanItemStatusPending {
		t.Errorf("Expected track:1 status PENDING, got %s", track1Item.Status)
	}

	if track2Item == nil {
		t.Fatal("Expected track:2 in loaded plan")
	}
	if track2Item.Status != PlanItemStatusCompleted {
		t.Errorf("Expected track:2 status COMPLETED, got %s", track2Item.Status)
	}
}

// Mock downloader for workflow tests
type mockWorkflowDownloader struct {
	downloadResults map[string]mockDownloadResult
}

type mockDownloadResult struct {
	success  bool
	filePath string
	err      error
}

func (m *mockWorkflowDownloader) DownloadTrack(ctx context.Context, item *PlanItem) (bool, string, error) {
	// Use SpotifyURL as key, fallback to YouTubeURL if SpotifyURL is empty
	url := item.SpotifyURL
	if url == "" {
		url = item.YouTubeURL
	}
	if result, ok := m.downloadResults[url]; ok {
		return result.success, result.filePath, result.err
	}
	return false, "", nil
}
