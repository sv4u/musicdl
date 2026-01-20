package plan

import (
	"sync"
	"testing"
	"time"
)

func TestPlanItem_GetProgress(t *testing.T) {
	item := &PlanItem{
		ItemID:   "test:1",
		ItemType: PlanItemTypeTrack,
		Progress: 0.75,
	}

	progress := item.GetProgress()
	if progress != 0.75 {
		t.Errorf("Expected progress 0.75, got %f", progress)
	}
}

func TestPlanItem_GetError(t *testing.T) {
	item := &PlanItem{
		ItemID:   "test:1",
		ItemType: PlanItemTypeTrack,
		Error:    "test error",
	}

	errorMsg := item.GetError()
	if errorMsg != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", errorMsg)
	}
}

func TestPlanItem_GetFilePath(t *testing.T) {
	item := &PlanItem{
		ItemID:   "test:1",
		ItemType: PlanItemTypeTrack,
		FilePath: "/path/to/file.mp3",
	}

	filePath := item.GetFilePath()
	if filePath != "/path/to/file.mp3" {
		t.Errorf("Expected file path '/path/to/file.mp3', got '%s'", filePath)
	}
}

func TestPlanItem_GetTimestamps(t *testing.T) {
	now := time.Now()
	startedAt := now.Add(1 * time.Hour)
	completedAt := now.Add(2 * time.Hour)

	item := &PlanItem{
		ItemID:      "test:1",
		ItemType:    PlanItemTypeTrack,
		CreatedAt:   now,
		StartedAt:   &startedAt,
		CompletedAt: &completedAt,
	}

	created, started, completed := item.GetTimestamps()

	if !created.Equal(now) {
		t.Errorf("Expected CreatedAt %v, got %v", now, created)
	}
	if started == nil || !started.Equal(startedAt) {
		t.Errorf("Expected StartedAt %v, got %v", startedAt, started)
	}
	if completed == nil || !completed.Equal(completedAt) {
		t.Errorf("Expected CompletedAt %v, got %v", completedAt, completed)
	}
}

func TestPlanItem_GetTimestamps_Nil(t *testing.T) {
	now := time.Now()

	item := &PlanItem{
		ItemID:    "test:1",
		ItemType:  PlanItemTypeTrack,
		CreatedAt: now,
	}

	created, started, completed := item.GetTimestamps()

	if !created.Equal(now) {
		t.Errorf("Expected CreatedAt %v, got %v", now, created)
	}
	if started != nil {
		t.Error("Expected StartedAt to be nil")
	}
	if completed != nil {
		t.Error("Expected CompletedAt to be nil")
	}
}

func TestPlanItem_GetMetadata(t *testing.T) {
	originalMetadata := map[string]interface{}{
		"artist": "Test Artist",
		"album":  "Test Album",
		"year":   2024,
	}

	item := &PlanItem{
		ItemID:    "test:1",
		ItemType:  PlanItemTypeTrack,
		Metadata:  originalMetadata,
	}

	metadata := item.GetMetadata()

	// Verify it's a copy (not the same reference)
	// We can't directly compare maps, but we can verify by modifying and checking original
	if len(metadata) != len(originalMetadata) {
		t.Error("GetMetadata() should return metadata with same length")
	}

	// Verify values match
	if metadata["artist"] != "Test Artist" {
		t.Errorf("Expected artist 'Test Artist', got %v", metadata["artist"])
	}
	if metadata["album"] != "Test Album" {
		t.Errorf("Expected album 'Test Album', got %v", metadata["album"])
	}
	if metadata["year"] != 2024 {
		t.Errorf("Expected year 2024, got %v", metadata["year"])
	}

	// Modify copy and verify original is unchanged
	metadata["artist"] = "Modified Artist"
	if originalMetadata["artist"] == "Modified Artist" {
		t.Error("Modifying returned metadata should not affect original")
	}
}

func TestPlanItem_GetMetadata_Empty(t *testing.T) {
	item := &PlanItem{
		ItemID:   "test:1",
		ItemType: PlanItemTypeTrack,
		Metadata: nil,
	}

	metadata := item.GetMetadata()
	if metadata == nil {
		t.Error("GetMetadata() should return empty map, not nil")
	}
	if len(metadata) != 0 {
		t.Errorf("Expected empty metadata, got %d items", len(metadata))
	}
}

func TestPlanItem_ThreadSafety(t *testing.T) {
	item := &PlanItem{
		ItemID:   "test:1",
		ItemType: PlanItemTypeTrack,
		Status:   PlanItemStatusPending,
		Progress: 0.0,
	}

	// Test concurrent reads
	// Reduce memory usage when running with race detector
	var wg sync.WaitGroup
	numGoroutines := 10
	iterations := 100
	if testing.RaceEnabled() {
		numGoroutines = 5
		iterations = 50
	}

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				_ = item.GetStatus()
				_ = item.GetProgress()
				_ = item.GetError()
				_ = item.GetFilePath()
				_, _, _ = item.GetTimestamps()
				_ = item.GetMetadata()
			}
		}()
	}

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < iterations; j++ {
				if id%2 == 0 {
					item.MarkStarted()
				} else {
					item.MarkCompleted("/path/to/file.mp3")
				}
			}
		}(i)
	}

	wg.Wait()

	// Verify item is in a valid state
	status := item.GetStatus()
	if status != PlanItemStatusCompleted && status != PlanItemStatusInProgress {
		t.Errorf("Item should be in valid state after concurrent operations, got %s", status)
	}
}
