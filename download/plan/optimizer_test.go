package plan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewOptimizer(t *testing.T) {
	optimizer := NewOptimizer(true)
	if optimizer == nil {
		t.Fatal("NewOptimizer() returned nil")
	}
	if !optimizer.checkFileExistence {
		t.Error("Expected checkFileExistence to be true")
	}
}

func TestOptimizer_RemoveDuplicates(t *testing.T) {
	optimizer := NewOptimizer(false)
	plan := NewDownloadPlan(nil)

	// Add duplicate tracks
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
		SpotifyID: "spotify_track_123", // Same Spotify ID
		Name:      "Track 1 (duplicate)",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track2)

	// Add a parent that references both
	album := &PlanItem{
		ItemID:   "album:1",
		ItemType: PlanItemTypeAlbum,
		Name:     "Album 1",
		Status:   PlanItemStatusPending,
		ChildIDs: []string{"track:1", "track:2"},
	}
	plan.AddItem(album)

	optimizer.removeDuplicates(plan)

	// Check that duplicate was removed
	if len(plan.Items) != 2 {
		t.Errorf("Expected 2 items (album + 1 track), got %d", len(plan.Items))
	}

	// Check that parent references were updated
	if len(album.ChildIDs) != 1 {
		t.Errorf("Expected 1 child reference, got %d", len(album.ChildIDs))
	}
	if album.ChildIDs[0] != "track:1" {
		t.Errorf("Expected child reference to be 'track:1', got '%s'", album.ChildIDs[0])
	}
}

func TestOptimizer_CheckFiles(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp3")
	
	// Create a test file
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	optimizer := NewOptimizer(true)
	plan := NewDownloadPlan(nil)

	// Add track with existing file
	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123",
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
		FilePath:  testFile,
	}
	plan.AddItem(track1)

	// Add track with non-existent file
	track2 := &PlanItem{
		ItemID:    "track:2",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_456",
		Name:      "Track 2",
		Status:    PlanItemStatusPending,
		FilePath:  filepath.Join(tmpDir, "nonexistent.mp3"),
	}
	plan.AddItem(track2)

	optimizer.checkFiles(plan)

	// Check that track1 was marked as skipped
	if track1.Status != PlanItemStatusSkipped {
		t.Errorf("Expected track1 status SKIPPED, got %s", track1.Status)
	}

	// Check that track2 remains pending
	if track2.Status != PlanItemStatusPending {
		t.Errorf("Expected track2 status PENDING, got %s", track2.Status)
	}
}

func TestOptimizer_Optimize(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp3")
	os.WriteFile(testFile, []byte("test"), 0644)

	optimizer := NewOptimizer(true)
	plan := NewDownloadPlan(nil)

	// Add duplicate tracks
	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123",
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
		FilePath:  testFile,
	}
	plan.AddItem(track1)

	track2 := &PlanItem{
		ItemID:    "track:2",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123", // Duplicate
		Name:      "Track 1 (duplicate)",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track2)

	optimizer.Optimize(plan)

	// Check that duplicate was removed
	if len(plan.Items) != 1 {
		t.Errorf("Expected 1 item after optimization, got %d", len(plan.Items))
	}

	// Check that remaining track was marked as skipped (file exists)
	if plan.Items[0].Status != PlanItemStatusSkipped {
		t.Errorf("Expected status SKIPPED, got %s", plan.Items[0].Status)
	}
}

func TestOptimizer_RemoveDuplicates_MultipleDuplicates(t *testing.T) {
	optimizer := NewOptimizer(false)
	plan := NewDownloadPlan(nil)

	// Add original track
	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123",
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track1)

	// Add multiple duplicates
	track2 := &PlanItem{
		ItemID:    "track:2",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123", // Duplicate
		Name:      "Track 1 (duplicate 1)",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track2)

	track3 := &PlanItem{
		ItemID:    "track:3",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123", // Duplicate
		Name:      "Track 1 (duplicate 2)",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track3)

	optimizer.removeDuplicates(plan)

	// Should only have one track
	trackCount := 0
	for _, item := range plan.Items {
		if item.ItemType == PlanItemTypeTrack {
			trackCount++
		}
	}
	if trackCount != 1 {
		t.Errorf("Expected 1 track after removing duplicates, got %d", trackCount)
	}

	// Should be the first one
	if plan.Items[0].ItemID != "track:1" {
		t.Errorf("Expected first track to remain, got '%s'", plan.Items[0].ItemID)
	}
}

func TestOptimizer_RemoveDuplicates_MultipleParents(t *testing.T) {
	optimizer := NewOptimizer(false)
	plan := NewDownloadPlan(nil)

	// Add original track
	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123",
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track1)

	// Add duplicate track
	track2 := &PlanItem{
		ItemID:    "track:2",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123", // Duplicate
		Name:      "Track 1 (duplicate)",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track2)

	// Add multiple parents that reference the duplicate
	album1 := &PlanItem{
		ItemID:   "album:1",
		ItemType: PlanItemTypeAlbum,
		Name:     "Album 1",
		Status:   PlanItemStatusPending,
		ChildIDs: []string{"track:2"},
	}
	plan.AddItem(album1)

	album2 := &PlanItem{
		ItemID:   "album:2",
		ItemType: PlanItemTypeAlbum,
		Name:     "Album 2",
		Status:   PlanItemStatusPending,
		ChildIDs: []string{"track:2"},
	}
	plan.AddItem(album2)

	playlist1 := &PlanItem{
		ItemID:   "playlist:1",
		ItemType: PlanItemTypePlaylist,
		Name:     "Playlist 1",
		Status:   PlanItemStatusPending,
		ChildIDs: []string{"track:2"},
	}
	plan.AddItem(playlist1)

	optimizer.removeDuplicates(plan)

	// Check that all parents now reference track:1
	for _, item := range plan.Items {
		if item.ItemType != PlanItemTypeTrack {
			// Check that parent references track:1, not track:2
			hasTrack1 := false
			hasTrack2 := false
			for _, childID := range item.ChildIDs {
				if childID == "track:1" {
					hasTrack1 = true
				}
				if childID == "track:2" {
					hasTrack2 = true
				}
			}
			if hasTrack2 {
				t.Errorf("Parent %s should not reference duplicate track:2", item.ItemID)
			}
			if !hasTrack1 {
				t.Errorf("Parent %s should reference original track:1", item.ItemID)
			}
		}
	}
}

func TestOptimizer_RemoveDuplicates_NoParents(t *testing.T) {
	optimizer := NewOptimizer(false)
	plan := NewDownloadPlan(nil)

	// Add original track (no parent)
	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123",
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track1)

	// Add duplicate track (no parent)
	track2 := &PlanItem{
		ItemID:    "track:2",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123", // Duplicate
		Name:      "Track 1 (duplicate)",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track2)

	optimizer.removeDuplicates(plan)

	// Should only have one track
	if len(plan.Items) != 1 {
		t.Errorf("Expected 1 item after removing duplicates, got %d", len(plan.Items))
	}
}

func TestOptimizer_RemoveDuplicates_EmptySpotifyID(t *testing.T) {
	optimizer := NewOptimizer(false)
	plan := NewDownloadPlan(nil)

	// Add track with empty SpotifyID
	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "", // Empty
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track1)

	// Add another track with empty SpotifyID
	track2 := &PlanItem{
		ItemID:    "track:2",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "", // Empty
		Name:      "Track 2",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track2)

	optimizer.removeDuplicates(plan)

	// Both should remain (empty SpotifyID tracks are not deduplicated)
	if len(plan.Items) != 2 {
		t.Errorf("Expected 2 items (empty SpotifyID not deduplicated), got %d", len(plan.Items))
	}
}

func TestOptimizer_RemoveDuplicates_NonTrackItems(t *testing.T) {
	optimizer := NewOptimizer(false)
	plan := NewDownloadPlan(nil)

	// Add album items (should not be deduplicated)
	album1 := &PlanItem{
		ItemID:    "album:1",
		ItemType:  PlanItemTypeAlbum,
		SpotifyID: "spotify_album_123",
		Name:      "Album 1",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(album1)

	album2 := &PlanItem{
		ItemID:    "album:2",
		ItemType:  PlanItemTypeAlbum,
		SpotifyID: "spotify_album_123", // Same SpotifyID but different type
		Name:      "Album 1 (duplicate)",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(album2)

	optimizer.removeDuplicates(plan)

	// Both albums should remain (only tracks are deduplicated)
	if len(plan.Items) != 2 {
		t.Errorf("Expected 2 items (albums not deduplicated), got %d", len(plan.Items))
	}
}

func TestOptimizer_CheckFiles_Disabled(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp3")
	os.WriteFile(testFile, []byte("test"), 0644)

	optimizer := NewOptimizer(false) // File checking disabled
	plan := NewDownloadPlan(nil)

	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123",
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
		FilePath:  testFile,
	}
	plan.AddItem(track1)

	// Note: checkFiles is a private method that's only called from Optimize
	// when checkFileExistence is true. Since it's false here, we verify
	// that Optimize doesn't check files when disabled.
	optimizer.Optimize(plan)

	// Should remain pending (file checking disabled)
	if track1.Status != PlanItemStatusPending {
		t.Errorf("Expected status PENDING (file checking disabled), got %s", track1.Status)
	}
}

func TestOptimizer_CheckFiles_NonPendingStatus(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.mp3")
	os.WriteFile(testFile, []byte("test"), 0644)

	optimizer := NewOptimizer(true)
	plan := NewDownloadPlan(nil)

	// Add track with completed status
	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123",
		Name:      "Track 1",
		Status:    PlanItemStatusCompleted, // Not pending
		FilePath:  testFile,
	}
	plan.AddItem(track1)

	optimizer.checkFiles(plan)

	// Should remain completed (only pending items are checked)
	if track1.Status != PlanItemStatusCompleted {
		t.Errorf("Expected status COMPLETED (not pending), got %s", track1.Status)
	}
}

func TestOptimizer_CheckFiles_NoFilePath(t *testing.T) {
	optimizer := NewOptimizer(true)
	plan := NewDownloadPlan(nil)

	// Add track without FilePath
	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123",
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
		FilePath:  "", // No file path
	}
	plan.AddItem(track1)

	optimizer.checkFiles(plan)

	// Should remain pending (no file path to check)
	if track1.Status != PlanItemStatusPending {
		t.Errorf("Expected status PENDING (no file path), got %s", track1.Status)
	}
}

func TestOptimizer_CheckFiles_MixedStatuses(t *testing.T) {
	tmpDir := t.TempDir()
	existingFile := filepath.Join(tmpDir, "existing.mp3")
	nonexistentFile := filepath.Join(tmpDir, "nonexistent.mp3")
	os.WriteFile(existingFile, []byte("test"), 0644)

	optimizer := NewOptimizer(true)
	plan := NewDownloadPlan(nil)

	// Add track with existing file
	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123",
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
		FilePath:  existingFile,
	}
	plan.AddItem(track1)

	// Add track with non-existent file
	track2 := &PlanItem{
		ItemID:    "track:2",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_456",
		Name:      "Track 2",
		Status:    PlanItemStatusPending,
		FilePath:  nonexistentFile,
	}
	plan.AddItem(track2)

	// Add track without file path
	track3 := &PlanItem{
		ItemID:    "track:3",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_789",
		Name:      "Track 3",
		Status:    PlanItemStatusPending,
		FilePath:  "",
	}
	plan.AddItem(track3)

	// Add track with completed status
	track4 := &PlanItem{
		ItemID:    "track:4",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_101",
		Name:      "Track 4",
		Status:    PlanItemStatusCompleted,
		FilePath:  existingFile,
	}
	plan.AddItem(track4)

	optimizer.checkFiles(plan)

	// Track1 should be skipped (file exists)
	if track1.Status != PlanItemStatusSkipped {
		t.Errorf("Expected track1 status SKIPPED, got %s", track1.Status)
	}

	// Track2 should remain pending (file doesn't exist)
	if track2.Status != PlanItemStatusPending {
		t.Errorf("Expected track2 status PENDING, got %s", track2.Status)
	}

	// Track3 should remain pending (no file path)
	if track3.Status != PlanItemStatusPending {
		t.Errorf("Expected track3 status PENDING, got %s", track3.Status)
	}

	// Track4 should remain completed (not pending, so not checked)
	if track4.Status != PlanItemStatusCompleted {
		t.Errorf("Expected track4 status COMPLETED, got %s", track4.Status)
	}
}

func TestOptimizer_Optimize_FileExistenceDisabled(t *testing.T) {
	optimizer := NewOptimizer(false) // File checking disabled
	plan := NewDownloadPlan(nil)

	// Add duplicate tracks
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
		SpotifyID: "spotify_track_123", // Duplicate
		Name:      "Track 1 (duplicate)",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track2)

	optimizer.Optimize(plan)

	// Check that duplicate was removed
	if len(plan.Items) != 1 {
		t.Errorf("Expected 1 item after optimization, got %d", len(plan.Items))
	}

	// Remaining track should still be pending (file checking disabled)
	if plan.Items[0].Status != PlanItemStatusPending {
		t.Errorf("Expected status PENDING (file checking disabled), got %s", plan.Items[0].Status)
	}
}

func TestOptimizer_RemoveDuplicates_ComplexParentChildRelationships(t *testing.T) {
	optimizer := NewOptimizer(false)
	plan := NewDownloadPlan(nil)

	// Add original track
	track1 := &PlanItem{
		ItemID:    "track:1",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123",
		Name:      "Track 1",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track1)

	// Add duplicate track
	track2 := &PlanItem{
		ItemID:    "track:2",
		ItemType:  PlanItemTypeTrack,
		SpotifyID: "spotify_track_123", // Duplicate
		Name:      "Track 1 (duplicate)",
		Status:    PlanItemStatusPending,
	}
	plan.AddItem(track2)

	// Add parent that references both original and duplicate
	album1 := &PlanItem{
		ItemID:   "album:1",
		ItemType: PlanItemTypeAlbum,
		Name:     "Album 1",
		Status:   PlanItemStatusPending,
		ChildIDs: []string{"track:1", "track:2"}, // References both
	}
	plan.AddItem(album1)

	// Add parent that only references duplicate
	album2 := &PlanItem{
		ItemID:   "album:2",
		ItemType: PlanItemTypeAlbum,
		Name:     "Album 2",
		Status:   PlanItemStatusPending,
		ChildIDs: []string{"track:2"}, // Only duplicate
	}
	plan.AddItem(album2)

	optimizer.removeDuplicates(plan)

	// Find album1
	var album1Item *PlanItem
	for _, item := range plan.Items {
		if item.ItemID == "album:1" {
			album1Item = item
			break
		}
	}
	if album1Item == nil {
		t.Fatal("Expected album1 to exist")
	}

	// Album1 should reference track:1 (and not track:2)
	hasTrack1 := false
	hasTrack2 := false
	for _, childID := range album1Item.ChildIDs {
		if childID == "track:1" {
			hasTrack1 = true
		}
		if childID == "track:2" {
			hasTrack2 = true
		}
	}
	if !hasTrack1 {
		t.Error("Expected album1 to reference track:1")
	}
	if hasTrack2 {
		t.Error("Expected album1 to not reference duplicate track:2")
	}

	// Find album2
	var album2Item *PlanItem
	for _, item := range plan.Items {
		if item.ItemID == "album:2" {
			album2Item = item
			break
		}
	}
	if album2Item == nil {
		t.Fatal("Expected album2 to exist")
	}

	// Album2 should now reference track:1 (duplicate replaced)
	hasTrack1 = false
	hasTrack2 = false
	for _, childID := range album2Item.ChildIDs {
		if childID == "track:1" {
			hasTrack1 = true
		}
		if childID == "track:2" {
			hasTrack2 = true
		}
	}
	if !hasTrack1 {
		t.Error("Expected album2 to reference track:1 (duplicate replaced)")
	}
	if hasTrack2 {
		t.Error("Expected album2 to not reference duplicate track:2")
	}
}
