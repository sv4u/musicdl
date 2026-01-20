package history

import (
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestAddSnapshot_RaceCondition tests that AddSnapshot doesn't panic when StopRun is called concurrently.
func TestAddSnapshot_RaceCondition(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 0, 1)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	runID := "test-run-race"
	tracker.StartRun(runID)

	// Concurrently call AddSnapshot and StopRun to trigger the race condition
	var wg sync.WaitGroup
	done := make(chan bool)

	// Reduce iterations when running with race detector to prevent OOM
	iterations := 1000
	if testing.RaceEnabled() {
		iterations = 100
	}

	// Goroutine 1: Continuously add snapshots
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			select {
			case <-done:
				return
			default:
				tracker.AddSnapshot(50.0, map[string]interface{}{"completed": 5}, "running", "executing")
				time.Sleep(1 * time.Microsecond)
			}
		}
	}()

	// Goroutine 2: Stop the run (which sets currentRun to nil)
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond) // Give AddSnapshot time to start
		statistics := map[string]interface{}{
			"completed": 10,
			"total":     10,
		}
		_ = tracker.StopRun("completed", "completed", statistics, "")
		close(done)
	}()

	// Wait for both goroutines to complete
	wg.Wait()

	// If we get here without a panic, the race condition is fixed
}

// TestSnapshotTicker_RaceCondition tests that the snapshot ticker goroutine doesn't panic
// when stopSnapshotTicker is called concurrently.
func TestSnapshotTicker_RaceCondition(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 0, 1)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	runID := "test-run-ticker"
	tracker.StartRun(runID)

	// Give the ticker goroutine time to start
	time.Sleep(50 * time.Millisecond)

	// Stop the run (which calls stopSnapshotTicker)
	statistics := map[string]interface{}{
		"completed": 10,
		"total":     10,
	}
	err = tracker.StopRun("completed", "completed", statistics, "")
	if err != nil {
		t.Fatalf("StopRun() failed: %v", err)
	}

	// Wait a bit to ensure the goroutine has time to exit
	time.Sleep(100 * time.Millisecond)

	// If we get here without a panic, the race condition is fixed
}

// TestAddSnapshot_ConcurrentAccess tests concurrent AddSnapshot calls.
func TestAddSnapshot_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	tracker, err := NewTracker(historyPath, 0, 1)
	if err != nil {
		t.Fatalf("NewTracker() failed: %v", err)
	}

	runID := "test-run-concurrent"
	tracker.StartRun(runID)

	// Concurrently add many snapshots
	// Reduce memory usage when running with race detector
	var wg sync.WaitGroup
	numGoroutines := 10
	snapshotsPerGoroutine := 100
	if testing.RaceEnabled() {
		// Reduce test data size when race detector is enabled to prevent OOM
		numGoroutines = 5
		snapshotsPerGoroutine = 20
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < snapshotsPerGoroutine; j++ {
				tracker.AddSnapshot(float64(j), map[string]interface{}{"completed": j}, "running", "executing")
			}
		}(i)
	}

	wg.Wait()

	// Verify snapshots were added
	currentRun := tracker.GetCurrentRun()
	if currentRun == nil {
		t.Fatal("GetCurrentRun() returned nil")
	}

	expectedSnapshots := numGoroutines * snapshotsPerGoroutine
	if len(currentRun.Snapshots) != expectedSnapshots {
		t.Errorf("Expected %d snapshots, got %d", expectedSnapshots, len(currentRun.Snapshots))
	}

	// Stop the run
	statistics := map[string]interface{}{
		"completed": 10,
		"total":     10,
	}
	_ = tracker.StopRun("completed", "completed", statistics, "")
}
