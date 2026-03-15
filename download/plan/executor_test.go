package plan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// mockDownloader is a mock downloader for testing.
type mockDownloader struct {
	downloadFunc func(ctx context.Context, item *PlanItem) (bool, string, error)
}

func (m *mockDownloader) DownloadTrack(ctx context.Context, item *PlanItem) (bool, string, error) {
	if m.downloadFunc != nil {
		return m.downloadFunc(ctx, item)
	}
	return false, "", nil
}

func TestNewExecutor(t *testing.T) {
	downloader := &mockDownloader{}
	executor := NewExecutor(downloader, 4)
	if executor == nil {
		t.Fatal("NewExecutor() returned nil")
	}
	if executor.maxWorkers != 4 {
		t.Errorf("Expected maxWorkers 4, got %d", executor.maxWorkers)
	}
}

func TestNewExecutor_DefaultWorkers(t *testing.T) {
	downloader := &mockDownloader{}
	executor := NewExecutor(downloader, 0)
	if executor.maxWorkers != 4 {
		t.Errorf("Expected default maxWorkers 4, got %d", executor.maxWorkers)
	}
}

func TestExecutor_Execute_EmptyPlan(t *testing.T) {
	downloader := &mockDownloader{}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	stats, err := executor.Execute(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if stats["total"] != 0 {
		t.Errorf("Expected total 0, got %d", stats["total"])
	}
}

func TestExecutor_Execute_SingleTrack(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp3")

	// Create a test file
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	downloader := &mockDownloader{
		downloadFunc: func(ctx context.Context, item *PlanItem) (bool, string, error) {
			return true, testFile, nil
		},
	}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	trackItem := &PlanItem{
		ItemID:     "track:1",
		ItemType:   PlanItemTypeTrack,
		SpotifyURL: "https://open.spotify.com/track/test",
		Name:       "Test Track",
		Status:     PlanItemStatusPending,
	}
	plan.AddItem(trackItem)

	stats, err := executor.Execute(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if stats["completed"] != 1 {
		t.Errorf("Expected 1 completed, got %d", stats["completed"])
	}
	if trackItem.Status != PlanItemStatusCompleted {
		t.Errorf("Expected track status COMPLETED, got %s", trackItem.Status)
	}
	if trackItem.FilePath != testFile {
		t.Errorf("Expected file path %s, got %s", testFile, trackItem.FilePath)
	}
}

func TestExecutor_Execute_TrackFailure(t *testing.T) {
	downloader := &mockDownloader{
		downloadFunc: func(ctx context.Context, item *PlanItem) (bool, string, error) {
			return false, "", nil
		},
	}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	trackItem := &PlanItem{
		ItemID:     "track:1",
		ItemType:   PlanItemTypeTrack,
		SpotifyURL: "https://open.spotify.com/track/test",
		Name:       "Test Track",
		Status:     PlanItemStatusPending,
	}
	plan.AddItem(trackItem)

	stats, err := executor.Execute(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if stats["failed"] != 1 {
		t.Errorf("Expected 1 failed, got %d", stats["failed"])
	}
	if trackItem.Status != PlanItemStatusFailed {
		t.Errorf("Expected track status FAILED, got %s", trackItem.Status)
	}
}

func TestExecutor_Execute_MissingSpotifyURL(t *testing.T) {
	downloader := &mockDownloader{}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	trackItem := &PlanItem{
		ItemID:   "track:1",
		ItemType: PlanItemTypeTrack,
		Name:     "Test Track",
		Status:   PlanItemStatusPending,
		// Missing SpotifyURL
	}
	plan.AddItem(trackItem)

	stats, err := executor.Execute(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}

	if stats["failed"] != 1 {
		t.Errorf("Expected 1 failed, got %d", stats["failed"])
	}
	if trackItem.Status != PlanItemStatusFailed {
		t.Errorf("Expected track status FAILED, got %s", trackItem.Status)
	}
}

func TestExecutor_Execute_SourceURLOnly(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sc_track.mp3")
	if err := os.WriteFile(testFile, []byte("audio"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	downloader := &mockDownloader{
		downloadFunc: func(ctx context.Context, item *PlanItem) (bool, string, error) {
			return true, testFile, nil
		},
	}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	trackItem := &PlanItem{
		ItemID:    "track:soundcloud:artist/track",
		ItemType:  PlanItemTypeTrack,
		Source:    SourceTypeSoundCloud,
		SourceURL: "https://soundcloud.com/artist/track",
		Name:      "SC Track",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(trackItem)

	stats, err := executor.Execute(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("Execute() returned error: %v", err)
	}
	if stats["completed"] != 1 {
		t.Errorf("expected 1 completed, got %d", stats["completed"])
	}
	if trackItem.Status != PlanItemStatusCompleted {
		t.Errorf("expected status COMPLETED, got %s", trackItem.Status)
	}
}

func TestExecutor_UpdateContainerStatus_AllCompleted(t *testing.T) {
	downloader := &mockDownloader{}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	// Create container with completed children
	containerItem := &PlanItem{
		ItemID:   "album:1",
		ItemType: PlanItemTypeAlbum,
		Name:     "Test Album",
		Status:   PlanItemStatusPending,
		ChildIDs: []string{"track:1", "track:2"},
	}
	plan.AddItem(containerItem)

	track1 := &PlanItem{
		ItemID:   "track:1",
		ItemType: PlanItemTypeTrack,
		Status:   PlanItemStatusCompleted,
	}
	plan.AddItem(track1)

	track2 := &PlanItem{
		ItemID:   "track:2",
		ItemType: PlanItemTypeTrack,
		Status:   PlanItemStatusCompleted,
	}
	plan.AddItem(track2)

	executor.updateContainerStatus(containerItem, plan)

	if containerItem.Status != PlanItemStatusCompleted {
		t.Errorf("Expected container status COMPLETED, got %s", containerItem.Status)
	}
}

func TestExecutor_UpdateContainerStatus_SomeFailed(t *testing.T) {
	downloader := &mockDownloader{}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	containerItem := &PlanItem{
		ItemID:   "album:1",
		ItemType: PlanItemTypeAlbum,
		Name:     "Test Album",
		Status:   PlanItemStatusPending,
		ChildIDs: []string{"track:1", "track:2"},
	}
	plan.AddItem(containerItem)

	track1 := &PlanItem{
		ItemID:   "track:1",
		ItemType: PlanItemTypeTrack,
		Status:   PlanItemStatusCompleted,
	}
	plan.AddItem(track1)

	track2 := &PlanItem{
		ItemID:   "track:2",
		ItemType: PlanItemTypeTrack,
		Status:   PlanItemStatusFailed,
	}
	plan.AddItem(track2)

	executor.updateContainerStatus(containerItem, plan)

	if containerItem.Status != PlanItemStatusFailed {
		t.Errorf("Expected container status FAILED, got %s", containerItem.Status)
	}
}

func TestExecutor_UpdateContainerStatus_NoChildren(t *testing.T) {
	downloader := &mockDownloader{}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	containerItem := &PlanItem{
		ItemID:   "album:1",
		ItemType: PlanItemTypeAlbum,
		Name:     "Test Album",
		Status:   PlanItemStatusPending,
		ChildIDs: []string{}, // No children
	}
	plan.AddItem(containerItem)

	executor.updateContainerStatus(containerItem, plan)

	if containerItem.Status != PlanItemStatusFailed {
		t.Errorf("Expected container status FAILED, got %s", containerItem.Status)
	}
}

func TestExecutor_CreateM3UFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test track files
	track1File := filepath.Join(tmpDir, "track1.mp3")
	track2File := filepath.Join(tmpDir, "track2.mp3")
	_ = os.WriteFile(track1File, []byte("test1"), 0644)
	_ = os.WriteFile(track2File, []byte("test2"), 0644)

	downloader := &mockDownloader{}
	executor := NewExecutor(downloader, 2)

	track1 := &PlanItem{
		ItemID:   "track:1",
		ItemType: PlanItemTypeTrack,
		Status:   PlanItemStatusCompleted,
		FilePath: track1File,
	}
	track2 := &PlanItem{
		ItemID:   "track:2",
		ItemType: PlanItemTypeTrack,
		Status:   PlanItemStatusCompleted,
		FilePath: track2File,
	}

	tracks := []*PlanItem{track1, track2}

	m3uPath, err := executor.createM3UFile("Test Playlist", tracks)
	if err != nil {
		t.Fatalf("createM3UFile() returned error: %v", err)
	}

	// Check if file exists
	if _, err := os.Stat(m3uPath); err != nil {
		t.Fatalf("M3U file not created: %v", err)
	}

	// Read and verify content
	content, err := os.ReadFile(m3uPath)
	if err != nil {
		t.Fatalf("Failed to read M3U file: %v", err)
	}

	contentStr := string(content)
	if contentStr[:8] != "#EXTM3U\n" {
		t.Errorf("M3U file missing header, got: %s", contentStr[:min(20, len(contentStr))])
	}

	// Check if tracks are included (relative paths for portability)
	if !contains(contentStr, "track1.mp3") || !contains(contentStr, "track2.mp3") {
		t.Errorf("M3U file missing track paths, got: %s", contentStr)
	}
}

func TestExecutor_CreateM3UFile_RelativePathsCrossDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate Artist/Album structure: M3U in Artist1/Album1/, track2 in Artist2/Album2/
	dir1 := filepath.Join(tmpDir, "Artist1", "Album1")
	dir2 := filepath.Join(tmpDir, "Artist2", "Album2")
	_ = os.MkdirAll(dir1, 0755)
	_ = os.MkdirAll(dir2, 0755)

	track1File := filepath.Join(dir1, "track1.mp3")
	track2File := filepath.Join(dir2, "track2.mp3")
	_ = os.WriteFile(track1File, []byte("test1"), 0644)
	_ = os.WriteFile(track2File, []byte("test2"), 0644)

	executor := NewExecutor(&mockDownloader{}, 2)
	tracks := []*PlanItem{
		{ItemID: "track:1", ItemType: PlanItemTypeTrack, Status: PlanItemStatusCompleted, FilePath: track1File},
		{ItemID: "track:2", ItemType: PlanItemTypeTrack, Status: PlanItemStatusCompleted, FilePath: track2File},
	}

	m3uPath, err := executor.createM3UFile("Cross Dir Playlist", tracks)
	if err != nil {
		t.Fatalf("createM3UFile() returned error: %v", err)
	}

	content, err := os.ReadFile(m3uPath)
	if err != nil {
		t.Fatalf("Failed to read M3U file: %v", err)
	}
	contentStr := string(content)

	// M3U is in Artist1/Album1/; track2 is in Artist2/Album2/ -> ../../Artist2/Album2/track2.mp3
	if !contains(contentStr, "track1.mp3") {
		t.Errorf("M3U missing track1, got: %s", contentStr)
	}
	if !contains(contentStr, "Artist2") || !contains(contentStr, "track2.mp3") {
		t.Errorf("M3U missing relative path to track2, got: %s", contentStr)
	}
	// Should not contain absolute paths (no leading slash for track paths in same-dir case, but cross-dir uses ..)
	if contains(contentStr, tmpDir) {
		t.Errorf("M3U should use relative paths, not absolute; got: %s", contentStr)
	}
}

func TestExecutor_RequestShutdown(t *testing.T) {
	downloader := &mockDownloader{
		downloadFunc: func(ctx context.Context, item *PlanItem) (bool, string, error) {
			// Simulate slow download
			time.Sleep(100 * time.Millisecond)
			return true, "/tmp/test.mp3", nil
		},
	}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	trackItem := &PlanItem{
		ItemID:     "track:1",
		ItemType:   PlanItemTypeTrack,
		SpotifyURL: "https://open.spotify.com/track/test",
		Name:       "Test Track",
		Status:     PlanItemStatusPending,
	}
	plan.AddItem(trackItem)

	// Request shutdown in a goroutine
	go func() {
		time.Sleep(10 * time.Millisecond)
		executor.RequestShutdown()
	}()

	_, err := executor.Execute(context.Background(), plan, nil)
	// Should get shutdown error
	if err == nil {
		t.Error("Expected shutdown error, got nil")
	}

	// Check that shutdown was requested
	if !executor.isShutdownRequested() {
		t.Error("Shutdown was not requested")
	}
}

func TestExecutor_WaitForShutdown_NoActiveExecution(t *testing.T) {
	downloader := &mockDownloader{
		downloadFunc: func(ctx context.Context, item *PlanItem) (bool, string, error) {
			return true, "/tmp/test.mp3", nil
		},
	}
	executor := NewExecutor(downloader, 2)

	// Wait for shutdown when no execution is active
	completed := executor.WaitForShutdown(1 * time.Second)
	if !completed {
		t.Error("Expected immediate completion when no execution is active")
	}
}

func TestExecutor_WaitForShutdown_WithActiveExecution(t *testing.T) {
	downloader := &mockDownloader{
		downloadFunc: func(ctx context.Context, item *PlanItem) (bool, string, error) {
			// Simulate download that takes 200ms
			time.Sleep(200 * time.Millisecond)
			return true, "/tmp/test.mp3", nil
		},
	}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	trackItem := &PlanItem{
		ItemID:     "track:1",
		ItemType:   PlanItemTypeTrack,
		SpotifyURL: "https://open.spotify.com/track/test",
		Name:       "Test Track",
		Status:     PlanItemStatusPending,
	}
	plan.AddItem(trackItem)

	// Start execution in goroutine
	done := make(chan bool)
	go func() {
		_, _ = executor.Execute(context.Background(), plan, nil)
		done <- true
	}()

	// Wait a bit for execution to start
	time.Sleep(50 * time.Millisecond)

	// Request shutdown
	executor.RequestShutdown()

	// Wait for shutdown with sufficient timeout
	completed := executor.WaitForShutdown(1 * time.Second)
	if !completed {
		t.Error("Expected shutdown to complete within timeout")
	}

	// Wait for execution goroutine to finish
	<-done
}

func TestExecutor_WaitForShutdown_Timeout(t *testing.T) {
	downloader := &mockDownloader{
		downloadFunc: func(ctx context.Context, item *PlanItem) (bool, string, error) {
			// Simulate very slow download (longer than timeout)
			time.Sleep(2 * time.Second)
			return true, "/tmp/test.mp3", nil
		},
	}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	trackItem := &PlanItem{
		ItemID:     "track:1",
		ItemType:   PlanItemTypeTrack,
		SpotifyURL: "https://open.spotify.com/track/test",
		Name:       "Test Track",
		Status:     PlanItemStatusPending,
	}
	plan.AddItem(trackItem)

	// Start execution in goroutine
	done := make(chan bool)
	go func() {
		_, _ = executor.Execute(context.Background(), plan, nil)
		done <- true
	}()

	// Wait a bit for execution to start
	time.Sleep(50 * time.Millisecond)

	// Wait for shutdown with short timeout (should timeout)
	completed := executor.WaitForShutdown(100 * time.Millisecond)
	if completed {
		t.Error("Expected timeout when downloads take longer than timeout")
	}

	// Request shutdown to ensure goroutine completes
	executor.RequestShutdown()

	// Wait for execution goroutine to finish (with timeout to prevent hanging)
	select {
	case <-done:
		// Execution completed
	case <-time.After(3 * time.Second):
		t.Error("Execution goroutine did not complete within timeout")
	}
}

// TestExecutor_WaitForShutdown_NilWG tests ISSUE-6 fix:
// Ensures WaitForShutdown handles nil WaitGroup correctly when wg becomes nil
// between the check and the Wait() call.
func TestExecutor_WaitForShutdown_NilWG(t *testing.T) {
	downloader := &mockDownloader{
		downloadFunc: func(ctx context.Context, item *PlanItem) (bool, string, error) {
			return true, "/tmp/test.mp3", nil
		},
	}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	trackItem := &PlanItem{
		ItemID:     "track:1",
		ItemType:   PlanItemTypeTrack,
		SpotifyURL: "https://open.spotify.com/track/test",
		Name:       "Test Track",
		Status:     PlanItemStatusPending,
	}
	plan.AddItem(trackItem)

	// Start execution
	done := make(chan bool)
	go func() {
		_, _ = executor.Execute(context.Background(), plan, nil)
		done <- true
	}()

	// Wait for execution to complete
	<-done

	// Now executionWg should be nil, but WaitForShutdown should handle it safely
	// This tests the fix where we re-check wg inside the goroutine
	completed := executor.WaitForShutdown(1 * time.Second)
	if !completed {
		t.Error("Expected completion when execution is done (wg is nil)")
	}

	// Test concurrent access - multiple calls to WaitForShutdown when wg is nil
	const numCalls = 10
	results := make(chan bool, numCalls)
	for i := 0; i < numCalls; i++ {
		go func() {
			results <- executor.WaitForShutdown(1 * time.Second)
		}()
	}

	// All should complete successfully
	for i := 0; i < numCalls; i++ {
		if !<-results {
			t.Error("Expected all WaitForShutdown calls to complete when wg is nil")
		}
	}
}

// --- Bug 4: Executor must not leak goroutines on context cancellation ---

func TestExecutor_Execute_ContextCancellation_NoLeak(t *testing.T) {
	downloadStarted := make(chan struct{}, 10)
	downloadBlocking := make(chan struct{})

	downloader := &mockDownloader{
		downloadFunc: func(ctx context.Context, item *PlanItem) (bool, string, error) {
			downloadStarted <- struct{}{}
			select {
			case <-downloadBlocking:
				return true, "/tmp/test.mp3", nil
			case <-ctx.Done():
				return false, "", ctx.Err()
			}
		},
	}
	executor := NewExecutor(downloader, 2)
	plan := NewDownloadPlan(nil)

	for i := 0; i < 5; i++ {
		plan.AddItem(&PlanItem{
			ItemID:     fmt.Sprintf("track:%d", i),
			ItemType:   PlanItemTypeTrack,
			SpotifyURL: fmt.Sprintf("https://open.spotify.com/track/test%d", i),
			Name:       fmt.Sprintf("Track %d", i),
			Status:     PlanItemStatusPending,
		})
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		_, err := executor.Execute(ctx, plan, nil)
		errCh <- err
	}()

	// Wait for at least one download to start
	<-downloadStarted

	// Cancel the context and unblock all downloads
	cancel()
	close(downloadBlocking)

	// Execute must return without hanging (goroutines properly cleaned up)
	select {
	case <-errCh:
		// success: Execute returned
	case <-time.After(5 * time.Second):
		t.Fatal("Execute did not return after context cancellation; goroutine leak likely")
	}
}

// Helper functions
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			contains(s[1:], substr)))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
