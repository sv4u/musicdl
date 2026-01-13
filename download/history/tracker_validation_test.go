package history

import (
	"path/filepath"
	"testing"
)

func TestNewTracker_NegativeSnapshotInterval(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	// Test with negative snapshot interval
	tracker, err := NewTracker(historyPath, 0, -5)

	// Should return an error
	if err == nil {
		t.Error("NewTracker() should fail with negative snapshotInterval")
	}
	if tracker != nil {
		t.Error("NewTracker() should return nil when snapshotInterval is negative")
	}
	if err != nil && err.Error() != "snapshotInterval must be positive, got -5" {
		t.Errorf("Expected specific error message, got: %v", err)
	}
}

func TestNewTracker_ZeroSnapshotInterval(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	// Test with zero snapshot interval
	tracker, err := NewTracker(historyPath, 0, 0)

	// Should return an error
	if err == nil {
		t.Error("NewTracker() should fail with zero snapshotInterval")
	}
	if tracker != nil {
		t.Error("NewTracker() should return nil when snapshotInterval is zero")
	}
}

func TestNewTracker_PositiveSnapshotInterval(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")

	// Test with positive snapshot interval
	tracker, err := NewTracker(historyPath, 0, 5)
	if err != nil {
		t.Fatalf("NewTracker() failed with positive snapshotInterval: %v", err)
	}
	if tracker == nil {
		t.Fatal("NewTracker() returned nil with valid snapshotInterval")
	}
}
