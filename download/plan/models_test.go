package plan

import (
	"testing"
	"time"
)

func TestPlanItem_MarkStarted(t *testing.T) {
	item := &PlanItem{
		ItemID:   "test:1",
		ItemType: PlanItemTypeTrack,
		Status:   PlanItemStatusPending,
	}

	item.MarkStarted()

	if item.Status != PlanItemStatusInProgress {
		t.Errorf("Expected status IN_PROGRESS, got %s", item.Status)
	}
	if item.StartedAt == nil {
		t.Error("Expected StartedAt to be set")
	}
}

func TestPlanItem_MarkCompleted(t *testing.T) {
	item := &PlanItem{
		ItemID:   "test:1",
		ItemType: PlanItemTypeTrack,
		Status:   PlanItemStatusInProgress,
	}

	item.MarkCompleted("/path/to/file.mp3")

	if item.Status != PlanItemStatusCompleted {
		t.Errorf("Expected status COMPLETED, got %s", item.Status)
	}
	if item.FilePath != "/path/to/file.mp3" {
		t.Errorf("Expected file path '/path/to/file.mp3', got '%s'", item.FilePath)
	}
	if item.Progress != 1.0 {
		t.Errorf("Expected progress 1.0, got %f", item.Progress)
	}
	if item.CompletedAt == nil {
		t.Error("Expected CompletedAt to be set")
	}
}

func TestPlanItem_MarkFailed(t *testing.T) {
	item := &PlanItem{
		ItemID:   "test:1",
		ItemType: PlanItemTypeTrack,
		Status:   PlanItemStatusInProgress,
	}

	item.MarkFailed("download error")

	if item.Status != PlanItemStatusFailed {
		t.Errorf("Expected status FAILED, got %s", item.Status)
	}
	if item.Error != "download error" {
		t.Errorf("Expected error 'download error', got '%s'", item.Error)
	}
	if item.Progress != 0.0 {
		t.Errorf("Expected progress 0.0, got %f", item.Progress)
	}
}

func TestDownloadPlan_AddItem(t *testing.T) {
	plan := NewDownloadPlan(nil)
	item := &PlanItem{
		ItemID:   "test:1",
		ItemType: PlanItemTypeTrack,
	}

	plan.AddItem(item)

	if len(plan.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(plan.Items))
	}
	if plan.Items[0].ItemID != "test:1" {
		t.Errorf("Expected item ID 'test:1', got '%s'", plan.Items[0].ItemID)
	}
}

func TestDownloadPlan_GetItem(t *testing.T) {
	plan := NewDownloadPlan(nil)
	item := &PlanItem{
		ItemID:   "test:1",
		ItemType: PlanItemTypeTrack,
	}
	plan.AddItem(item)

	found := plan.GetItem("test:1")
	if found == nil {
		t.Fatal("Expected to find item 'test:1'")
	}
	if found.ItemID != "test:1" {
		t.Errorf("Expected item ID 'test:1', got '%s'", found.ItemID)
	}

	notFound := plan.GetItem("nonexistent")
	if notFound != nil {
		t.Error("Expected nil for nonexistent item")
	}
}

func TestDownloadPlan_GetItemsByType(t *testing.T) {
	plan := NewDownloadPlan(nil)
	plan.AddItem(&PlanItem{ItemID: "track:1", ItemType: PlanItemTypeTrack})
	plan.AddItem(&PlanItem{ItemID: "album:1", ItemType: PlanItemTypeAlbum})
	plan.AddItem(&PlanItem{ItemID: "track:2", ItemType: PlanItemTypeTrack})

	tracks := plan.GetItemsByType(PlanItemTypeTrack)
	if len(tracks) != 2 {
		t.Errorf("Expected 2 tracks, got %d", len(tracks))
	}
}

func TestDownloadPlan_GetStatistics(t *testing.T) {
	plan := NewDownloadPlan(nil)
	plan.AddItem(&PlanItem{ItemID: "track:1", ItemType: PlanItemTypeTrack})
	plan.AddItem(&PlanItem{ItemID: "album:1", ItemType: PlanItemTypeAlbum})

	stats := plan.GetStatistics()
	if stats["total_items"].(int) != 2 {
		t.Errorf("Expected total_items 2, got %d", stats["total_items"])
	}

	byType := stats["by_type"].(map[string]int)
	if byType["track"] != 1 {
		t.Errorf("Expected 1 track, got %d", byType["track"])
	}
	if byType["album"] != 1 {
		t.Errorf("Expected 1 album, got %d", byType["album"])
	}
}

func TestDownloadPlan_SaveLoad(t *testing.T) {
	plan := NewDownloadPlan(map[string]interface{}{
		"config_version": "1.0",
	})
	item := &PlanItem{
		ItemID:   "test:1",
		ItemType: PlanItemTypeTrack,
		Name:     "Test Track",
		Status:   PlanItemStatusPending,
		CreatedAt: time.Now(),
	}
	plan.AddItem(item)

	// Save
	tmpFile := t.TempDir() + "/test_plan.json"
	if err := plan.Save(tmpFile); err != nil {
		t.Fatalf("Failed to save plan: %v", err)
	}

	// Load
	loaded, err := LoadPlan(tmpFile)
	if err != nil {
		t.Fatalf("Failed to load plan: %v", err)
	}

	if len(loaded.Items) != 1 {
		t.Errorf("Expected 1 item, got %d", len(loaded.Items))
	}
	if loaded.Items[0].ItemID != "test:1" {
		t.Errorf("Expected item ID 'test:1', got '%s'", loaded.Items[0].ItemID)
	}
	if loaded.Metadata["config_version"] != "1.0" {
		t.Errorf("Expected config_version '1.0', got '%v'", loaded.Metadata["config_version"])
	}
}

func TestDownloadPlan_SaveLoad_FileNotFound(t *testing.T) {
	_, err := LoadPlan("/nonexistent/file.json")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}
