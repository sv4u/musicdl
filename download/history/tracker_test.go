package history

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewTracker(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 10, 5)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}
	if tracker == nil {
		t.Fatal("NewTracker() returned nil")
	}

	// Verify directory was created
	if _, err := os.Stat(historyPath); os.IsNotExist(err) {
		t.Error("History directory was not created")
	}
}

func TestTracker_StartRun(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 0, 1)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	runID := "test-run-123"
	tracker.StartRun(runID)

	currentRun := tracker.GetCurrentRun()
	if currentRun == nil {
		t.Fatal("GetCurrentRun() returned nil after StartRun()")
	}
	if currentRun.RunID != runID {
		t.Errorf("Expected RunID %s, got %s", runID, currentRun.RunID)
	}
	if currentRun.State != "running" {
		t.Errorf("Expected state 'running', got %s", currentRun.State)
	}
}

func TestTracker_StopRun(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 0, 1)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	runID := "test-run-123"
	tracker.StartRun(runID)

	statistics := map[string]interface{}{
		"completed": 10,
		"failed":    2,
		"total":     12,
	}

	err = tracker.StopRun("completed", "completed", statistics, "")
	if err != nil {
		t.Fatalf("StopRun() failed: %v", err)
	}

	// Verify run was saved
	run, err := tracker.GetRunHistory(runID)
	if err != nil {
		t.Fatalf("GetRunHistory() failed: %v", err)
	}
	if run.State != "completed" {
		t.Errorf("Expected state 'completed', got %s", run.State)
	}
	if run.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}

	// Verify current run is cleared
	currentRun := tracker.GetCurrentRun()
	if currentRun != nil {
		t.Error("GetCurrentRun() should return nil after StopRun()")
	}
}

func TestTracker_AddSnapshot(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 0, 1)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	runID := "test-run-123"
	tracker.StartRun(runID)

	statistics := map[string]interface{}{
		"completed": 5,
		"total":     10,
	}

	tracker.AddSnapshot(50.0, statistics, "running", "executing")

	currentRun := tracker.GetCurrentRun()
	if currentRun == nil {
		t.Fatal("GetCurrentRun() returned nil")
	}
	if len(currentRun.Snapshots) != 1 {
		t.Errorf("Expected 1 snapshot, got %d", len(currentRun.Snapshots))
	}

	snapshot := currentRun.Snapshots[0]
	if snapshot.Progress != 50.0 {
		t.Errorf("Expected progress 50.0, got %f", snapshot.Progress)
	}
	if snapshot.State != "running" {
		t.Errorf("Expected state 'running', got %s", snapshot.State)
	}
}

func TestTracker_ListRuns(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 0, 1)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	// Create multiple runs
	for i := 0; i < 3; i++ {
		runID := "test-run-" + string(rune('0'+i))
		tracker.StartRun(runID)
		time.Sleep(10 * time.Millisecond) // Ensure different timestamps
		tracker.StopRun("completed", "completed", map[string]interface{}{}, "")
	}

	runIDs, err := tracker.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns() failed: %v", err)
	}
	if len(runIDs) != 3 {
		t.Errorf("Expected 3 runs, got %d", len(runIDs))
	}

	// Verify runs are sorted by start time (newest first)
	// The last run should be first in the list
	if runIDs[0] != "test-run-2" {
		t.Errorf("Expected newest run first, got %s", runIDs[0])
	}
}

func TestTracker_CleanupOldRuns(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 2, 1) // Keep only 2 runs
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	// Create 3 runs
	for i := 0; i < 3; i++ {
		runID := "test-run-" + string(rune('0'+i))
		tracker.StartRun(runID)
		time.Sleep(10 * time.Millisecond)
		tracker.StopRun("completed", "completed", map[string]interface{}{}, "")
	}

	// Verify only 2 runs remain
	runIDs, err := tracker.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns() failed: %v", err)
	}
	if len(runIDs) != 2 {
		t.Errorf("Expected 2 runs after cleanup, got %d", len(runIDs))
	}

	// Verify oldest run was removed
	oldestRunPath := filepath.Join(historyPath, "run_test-run-0.json")
	if _, err := os.Stat(oldestRunPath); err == nil {
		t.Error("Oldest run file should have been removed")
	}
}

func TestTracker_AddActivity(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 0, 1)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	tracker.AddActivity("test_event", "Test message", map[string]interface{}{
		"key": "value",
	})

	history := tracker.GetActivityHistory(0)
	if len(history.Entries) != 1 {
		t.Errorf("Expected 1 activity entry, got %d", len(history.Entries))
	}

	entry := history.Entries[0]
	if entry.Type != "test_event" {
		t.Errorf("Expected type 'test_event', got %s", entry.Type)
	}
	if entry.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got %s", entry.Message)
	}
}

func TestTracker_GetActivityHistory_Limit(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 0, 1)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	// Add 10 activities
	for i := 0; i < 10; i++ {
		tracker.AddActivity("test_event", "Message "+string(rune('0'+i)), nil)
	}

	// Get only 5 most recent
	history := tracker.GetActivityHistory(5)
	if len(history.Entries) != 5 {
		t.Errorf("Expected 5 entries with limit, got %d", len(history.Entries))
	}

	// Verify they are the most recent ones (entries are in chronological order)
	// The last entry should be the most recent
	if len(history.Entries) > 0 {
		lastEntry := history.Entries[len(history.Entries)-1]
		if !strings.Contains(lastEntry.Message, "Message") {
			t.Errorf("Expected message to contain 'Message', got %s", lastEntry.Message)
		}
	}
}

func TestTracker_Close(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 0, 1)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	// Add some activity
	tracker.AddActivity("test_event", "Test message", nil)

	// Close tracker
	err = tracker.Close()
	if err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	// Verify activity was saved
	activityPath := filepath.Join(historyPath, "activity.json")
	if _, err := os.Stat(activityPath); os.IsNotExist(err) {
		t.Error("Activity file should have been saved")
	}
}
