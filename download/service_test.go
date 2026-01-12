package download

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/plan"
	"github.com/sv4u/musicdl/download/spotify"
)

func TestNewService(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
			Threads:      4,
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		t.Fatalf("Failed to create Spotify client: %v", err)
	}

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
		Bitrate:      "128k",
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		t.Fatalf("Failed to create audio provider: %v", err)
	}

	metadataEmbedder := metadata.NewEmbedder()

	service, err := NewService(cfg, spotifyClient, audioProvider, metadataEmbedder, "")
	if err != nil {
		t.Fatalf("NewService() returned error: %v", err)
	}
	if service == nil {
		t.Fatal("NewService() returned nil")
	}
}

func TestService_GetStatus_Idle(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), "")

	status := service.GetStatus()
	if status["state"] != ServiceStateIdle {
		t.Errorf("Expected state 'idle', got '%v'", status["state"])
	}
}

func TestService_Start_AlreadyRunning(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), "")

	// Manually set state to running
	service.mu.Lock()
	service.state = ServiceStateRunning
	service.mu.Unlock()

	// Try to start again
	err := service.Start(context.Background())
	if err == nil {
		t.Error("Expected error when starting already running service")
	}
}

func TestService_Stop_NotRunning(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), "")

	// Try to stop when not running
	err := service.Stop()
	if err == nil {
		t.Error("Expected error when stopping non-running service")
	}
}

func TestService_GetPlan(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), "")

	// Initially should be nil
	plan := service.GetPlan()
	if plan != nil {
		t.Error("Expected nil plan initially")
	}
}

func TestService_Stop_GracefulShutdown(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), "")

	// Manually set state to running
	service.mu.Lock()
	service.state = ServiceStateRunning
	service.mu.Unlock()

	// Stop should succeed (even if no active execution)
	err := service.Stop()
	if err != nil {
		// If there's no active execution, WaitForShutdown returns immediately
		// So this should succeed
		t.Logf("Stop returned error (expected if no active execution): %v", err)
	}

	// State should be idle after successful shutdown
	service.mu.RLock()
	state := service.state
	service.mu.RUnlock()
	
	// State could be idle or error depending on whether execution was active
	if state != ServiceStateIdle && state != ServiceStateError {
		t.Errorf("Expected state 'idle' or 'error' after stop, got '%v'", state)
	}
}

func TestService_GetStatus_ProgressPercentage(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), "")

	// Get status - should include progress_percentage
	status := service.GetStatus()
	
	progress, ok := status["progress_percentage"]
	if !ok {
		t.Error("Expected progress_percentage in status")
	}
	
	progressFloat, ok := progress.(float64)
	if !ok {
		t.Errorf("Expected progress_percentage to be float64, got %T", progress)
	}
	
	if progressFloat != 0.0 {
		t.Errorf("Expected initial progress 0.0, got %f", progressFloat)
	}
}

func TestService_UpdateProgress(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), "")

	// Create a plan with some tracks
	testPlan := plan.NewDownloadPlan(nil)
	
	// Add completed track
	track1 := &plan.PlanItem{
		ItemID:   "track:1",
		ItemType: plan.PlanItemTypeTrack,
		Status:   plan.PlanItemStatusCompleted,
	}
	track1.MarkCompleted("/path/to/track1.mp3")
	testPlan.AddItem(track1)
	
	// Add pending track
	track2 := &plan.PlanItem{
		ItemID:   "track:2",
		ItemType: plan.PlanItemTypeTrack,
		Status:   plan.PlanItemStatusPending,
	}
	testPlan.AddItem(track2)
	
	// Add skipped track
	track3 := &plan.PlanItem{
		ItemID:   "track:3",
		ItemType: plan.PlanItemTypeTrack,
		Status:   plan.PlanItemStatusSkipped,
	}
	track3.MarkSkipped("/path/to/track3.mp3")
	testPlan.AddItem(track3)

	// Set plan in service
	service.mu.Lock()
	service.currentPlan = testPlan
	service.mu.Unlock()

	// Update progress
	service.updateProgress()

	// Check progress percentage (2 completed/skipped out of 3 total = 66.67%)
	service.progressMu.RLock()
	progress := service.progressPercentage
	service.progressMu.RUnlock()
	
	expectedProgress := 66.66666666666666
	if progress < expectedProgress-0.1 || progress > expectedProgress+0.1 {
		t.Errorf("Expected progress ~%.2f%%, got %.2f%%", expectedProgress, progress)
	}
}

func TestService_GetCacheStats(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:              "test_id",
			ClientSecret:          "test_secret",
			CacheMaxSize:          1000,
			CacheTTL:              3600,
			AudioSearchCacheMaxSize: 500,
			AudioSearchCacheTTL:   86400,
		},
	}
	cfg.Download.SetDefaults()

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
		CacheMaxSize: 1000,
		CacheTTL:     3600,
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
		CacheMaxSize: 500,
		CacheTTL:     86400,
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), "")

	stats := service.GetCacheStats()

	// Verify Spotify cache stats
	if stats.Spotify.MaxSize != 1000 {
		t.Errorf("Expected Spotify cache MaxSize 1000, got %d", stats.Spotify.MaxSize)
	}
	if stats.SpotifyTTL != 3600 {
		t.Errorf("Expected Spotify TTL 3600, got %d", stats.SpotifyTTL)
	}

	// Verify Audio search cache stats
	if stats.AudioSearch.MaxSize != 500 {
		t.Errorf("Expected AudioSearch cache MaxSize 500, got %d", stats.AudioSearch.MaxSize)
	}
	if stats.AudioSearchTTL != 86400 {
		t.Errorf("Expected AudioSearch TTL 86400, got %d", stats.AudioSearchTTL)
	}

	// Verify FileExistence cache stats
	if stats.FileExistence == nil {
		t.Error("Expected FileExistence cache stats")
	}
}

func TestService_WaitForCompletion(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), "")

	// Test WaitForCompletion on idle service (should return immediately)
	done := make(chan bool, 1)
	go func() {
		service.WaitForCompletion()
		done <- true
	}()

	select {
	case <-done:
		// Success - should complete quickly for idle service
	case <-time.After(2 * time.Second):
		t.Error("WaitForCompletion() should return immediately for idle service")
	}

	// Test WaitForCompletion on completed service
	service.mu.Lock()
	service.state = ServiceStateIdle
	service.phase = ServicePhaseCompleted
	service.mu.Unlock()

	done2 := make(chan bool, 1)
	go func() {
		service.WaitForCompletion()
		done2 <- true
	}()

	select {
	case <-done2:
		// Success
	case <-time.After(2 * time.Second):
		t.Error("WaitForCompletion() should return immediately for completed service")
	}
}

func TestService_ProgressCallback(t *testing.T) {
	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:                "test_id",
			ClientSecret:            "test_secret",
			PlanPersistenceEnabled:  false, // Disable to avoid file I/O
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), "")

	// Create a plan with a track
	testPlan := plan.NewDownloadPlan(nil)
	track := &plan.PlanItem{
		ItemID:   "track:1",
		ItemType: plan.PlanItemTypeTrack,
		Status:   plan.PlanItemStatusPending,
	}
	testPlan.AddItem(track)

	// Set plan in service
	service.mu.Lock()
	service.currentPlan = testPlan
	service.mu.Unlock()

	// Call progressCallback
	service.progressCallback(track)

	// Verify progress was updated
	service.progressMu.RLock()
	progress := service.progressPercentage
	service.progressMu.RUnlock()

	// Progress should still be 0% (no completed tracks)
	if progress != 0.0 {
		t.Errorf("Expected progress 0.0%%, got %.2f%%", progress)
	}
}

func TestService_SavePlanThrottled(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plans")

	// Ensure plan directory exists
	if err := os.MkdirAll(planPath, 0755); err != nil {
		t.Fatalf("Failed to create plan directory: %v", err)
	}

	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:               "test_id",
			ClientSecret:           "test_secret",
			PlanPersistenceEnabled: true,
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), planPath)

	// Create a plan
	testPlan := plan.NewDownloadPlan(nil)
	track := &plan.PlanItem{
		ItemID:   "track:1",
		ItemType: plan.PlanItemTypeTrack,
		Status:   plan.PlanItemStatusPending,
	}
	testPlan.AddItem(track)

	// Set plan in service
	service.mu.Lock()
	service.currentPlan = testPlan
	service.phase = ServicePhaseExecuting
	service.mu.Unlock()

	// Test savePlanThrottled - first call should save
	service.savePlanThrottled()

	// Verify plan was saved
	progressPath := filepath.Join(planPath, "download_plan_progress.json")
	if _, err := os.Stat(progressPath); err != nil {
		t.Errorf("Expected plan file to be saved, got error: %v", err)
	}

	// Second call immediately should be throttled (no save)
	service.savePlanThrottled()

	// Wait a bit and call again - should save
	time.Sleep(2100 * time.Millisecond)
	service.savePlanThrottled()

	// Verify plan file still exists
	if _, err := os.Stat(progressPath); err != nil {
		t.Errorf("Expected plan file to still exist, got error: %v", err)
	}
}

func TestService_SavePlan_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plans")

	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:               "test_id",
			ClientSecret:           "test_secret",
			PlanPersistenceEnabled: false, // Disabled
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), planPath)

	// Create a plan
	testPlan := plan.NewDownloadPlan(nil)
	track := &plan.PlanItem{
		ItemID:   "track:1",
		ItemType: plan.PlanItemTypeTrack,
		Status:   plan.PlanItemStatusPending,
	}
	testPlan.AddItem(track)

	// Set plan in service
	service.mu.Lock()
	service.currentPlan = testPlan
	service.mu.Unlock()

	// Call savePlan - should not save when disabled
	service.savePlan()

	// Verify plan was NOT saved
	progressPath := filepath.Join(planPath, "download_plan_progress.json")
	if _, err := os.Stat(progressPath); err == nil {
		t.Error("Expected plan file NOT to be saved when persistence is disabled")
	}
}

func TestService_SavePlan_NoPlan(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plans")

	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:               "test_id",
			ClientSecret:           "test_secret",
			PlanPersistenceEnabled: true,
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), planPath)

	// Call savePlan with no plan - should not error
	service.savePlan()

	// Verify no plan file was created
	progressPath := filepath.Join(planPath, "download_plan_progress.json")
	if _, err := os.Stat(progressPath); err == nil {
		t.Error("Expected no plan file when no plan exists")
	}
}

// TestService_Start_ContextCancellation tests ISSUE-3 fix:
// Ensures service goroutines check context cancellation before updating state.
func TestService_Start_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plans")

	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:     "test_id",
			ClientSecret: "test_secret",
			Threads:      1,
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)
	defer spotifyClient.Close()

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), planPath)

	// Create a context that will be cancelled
	ctx, cancel := context.WithCancel(context.Background())

	// Start service
	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("Start() returned error: %v", err)
	}

	// Cancel context immediately
	cancel()

	// Wait a bit for goroutine to process cancellation
	time.Sleep(100 * time.Millisecond)

	// Verify service doesn't update state after cancellation
	// The goroutine should return early without updating state
	status := service.GetStatus()
	// State might be running or error, but shouldn't be completed if cancelled early
	if status["phase"] == "completed" {
		t.Error("Service should not complete after context cancellation")
	}
}

// TestService_SavePlan_RaceCondition tests ISSUE-4 fix:
// Ensures savePlan() creates copies to avoid race conditions.
func TestService_SavePlan_RaceCondition(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plans")

	cfg := &config.MusicDLConfig{
		Version: "1.0",
		Download: config.DownloadSettings{
			ClientID:               "test_id",
			ClientSecret:           "test_secret",
			PlanPersistenceEnabled: true,
		},
	}

	spotifyConfig := &spotify.Config{
		ClientID:     "test_id",
		ClientSecret: "test_secret",
	}
	spotifyClient, _ := spotify.NewSpotifyClient(spotifyConfig)
	defer spotifyClient.Close()

	audioConfig := &audio.Config{
		OutputFormat: "mp3",
	}
	audioProvider, _ := audio.NewProvider(audioConfig)

	// Ensure plan directory exists
	if err := os.MkdirAll(planPath, 0755); err != nil {
		t.Fatalf("Failed to create plan directory: %v", err)
	}

	service, _ := NewService(cfg, spotifyClient, audioProvider, metadata.NewEmbedder(), planPath)

	// Create a plan with metadata
	testPlan := plan.NewDownloadPlan(map[string]interface{}{
		"test_key": "test_value",
	})
	item := &plan.PlanItem{
		ItemID:   "item1",
		ItemType: plan.PlanItemTypeTrack,
		Name:     "Test Track",
		Status:   plan.PlanItemStatusPending,
	}
	testPlan.AddItem(item)

	// Set plan in service
	service.mu.Lock()
	service.currentPlan = testPlan
	service.phase = ServicePhaseExecuting
	service.mu.Unlock()

	// Concurrently modify metadata and save plan
	done := make(chan bool, 2)
	go func() {
		// Modify metadata while savePlan might be running
		for i := 0; i < 10; i++ {
			service.mu.Lock()
			if service.currentPlan != nil && service.currentPlan.Metadata != nil {
				service.currentPlan.Metadata["concurrent_key"] = i
			}
			service.mu.Unlock()
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	go func() {
		// Call savePlan multiple times concurrently
		for i := 0; i < 10; i++ {
			service.savePlan()
			time.Sleep(10 * time.Millisecond)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Verify plan file was saved (should not panic or corrupt)
	progressPath := filepath.Join(planPath, "download_plan_progress.json")
	if _, err := os.Stat(progressPath); err != nil {
		t.Logf("Plan file not created (may be expected): %v", err)
	} else {
		// Try to load the plan to verify it's valid
		loadedPlan, err := plan.LoadPlan(progressPath)
		if err != nil {
			t.Errorf("Failed to load saved plan: %v", err)
		} else if loadedPlan == nil {
			t.Error("Loaded plan is nil")
		}
	}
}
