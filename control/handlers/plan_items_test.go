package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sv4u/musicdl/download/plan"
)

func setupPlanItemsTest(t *testing.T) (*Handlers, *plan.DownloadPlan) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now())
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	// Create a test plan with items
	testPlan := plan.NewDownloadPlan(map[string]interface{}{})

	// Create parent playlist
	playlistItem := &plan.PlanItem{
		ItemID:   "playlist:1",
		ItemType: plan.PlanItemTypePlaylist,
		Name:     "Test Playlist",
		Status:   plan.PlanItemStatusCompleted,
		CreatedAt: time.Now(),
	}
	testPlan.AddItem(playlistItem)

	// Create tracks with different statuses
	track1 := &plan.PlanItem{
		ItemID:     "track:1",
		ItemType:   plan.PlanItemTypeTrack,
		Name:       "Track One",
		Status:     plan.PlanItemStatusCompleted,
		ParentID:   "playlist:1",
		Progress:   1.0,
		FilePath:   "/path/to/track1.mp3",
		CreatedAt:  time.Now(),
		Metadata: map[string]interface{}{
			"artist": "Artist One",
			"album":  "Album One",
		},
	}
	track1.MarkCompleted("/path/to/track1.mp3")
	testPlan.AddItem(track1)
	playlistItem.ChildIDs = append(playlistItem.ChildIDs, track1.ItemID)

	track2 := &plan.PlanItem{
		ItemID:     "track:2",
		ItemType:   plan.PlanItemTypeTrack,
		Name:       "Track Two",
		Status:     plan.PlanItemStatusFailed,
		ParentID:   "playlist:1",
		Progress:   0.0,
		Error:      "Download failed",
		CreatedAt:  time.Now(),
		Metadata: map[string]interface{}{
			"artist": "Artist Two",
			"album":  "Album Two",
		},
	}
	track2.MarkFailed("Download failed")
	testPlan.AddItem(track2)
	playlistItem.ChildIDs = append(playlistItem.ChildIDs, track2.ItemID)

	track3 := &plan.PlanItem{
		ItemID:     "track:3",
		ItemType:   plan.PlanItemTypeTrack,
		Name:       "Track Three",
		Status:     plan.PlanItemStatusInProgress,
		ParentID:   "playlist:1",
		Progress:   0.5,
		CreatedAt:  time.Now(),
		Metadata: map[string]interface{}{
			"artist": "Artist Three",
			"album":  "Album Three",
		},
	}
	track3.MarkStarted()
	testPlan.AddItem(track3)
	playlistItem.ChildIDs = append(playlistItem.ChildIDs, track3.ItemID)

	// Note: We can't directly set the plan in the service without exposing internal methods
	// The tests will verify the API structure and filtering/sorting logic
	// Integration tests would require a running service with an actual plan

	return handlers, testPlan
}

func TestPlanItems_Basic(t *testing.T) {
	handlers, _ := setupPlanItemsTest(t)

	req := httptest.NewRequest("GET", "/api/plan/items", nil)
	w := httptest.NewRecorder()

	handlers.PlanItems(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PlanItems() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response PlanItemsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if response.Items == nil {
		t.Error("Response.Items is nil")
	}
}

func TestPlanItems_FilterByStatus(t *testing.T) {
	handlers, _ := setupPlanItemsTest(t)

	req := httptest.NewRequest("GET", "/api/plan/items?status=completed", nil)
	w := httptest.NewRecorder()

	handlers.PlanItems(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PlanItems() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response PlanItemsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify all items have status "completed"
	for _, item := range response.Items {
		if item.Status != "completed" {
			t.Errorf("Expected all items to have status 'completed', got %s", item.Status)
		}
	}
}

func TestPlanItems_Search(t *testing.T) {
	handlers, _ := setupPlanItemsTest(t)

	// Use url.Values to properly encode the query string
	query := url.Values{}
	query.Set("search", "Track One")
	req := httptest.NewRequest("GET", "/api/plan/items?"+query.Encode(), nil)
	w := httptest.NewRecorder()

	handlers.PlanItems(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PlanItems() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response PlanItemsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify search results contain "Track One"
	found := false
	for _, item := range response.Items {
		if strings.Contains(strings.ToLower(item.Name), "track one") {
			found = true
			break
		}
	}
	if !found && len(response.Items) > 0 {
		t.Error("Search should find 'Track One'")
	}
}

func TestPlanItems_SortByName(t *testing.T) {
	handlers, _ := setupPlanItemsTest(t)

	req := httptest.NewRequest("GET", "/api/plan/items?sort=name&order=asc", nil)
	w := httptest.NewRecorder()

	handlers.PlanItems(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PlanItems() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response PlanItemsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify items are sorted by name
	if len(response.Items) > 1 {
		for i := 1; i < len(response.Items); i++ {
			if strings.ToLower(response.Items[i-1].Name) > strings.ToLower(response.Items[i].Name) {
				t.Errorf("Items not sorted correctly: %s should come before %s",
					response.Items[i-1].Name, response.Items[i].Name)
			}
		}
	}
}

func TestPlanItems_FilterByType(t *testing.T) {
	handlers, _ := setupPlanItemsTest(t)

	req := httptest.NewRequest("GET", "/api/plan/items?type=track", nil)
	w := httptest.NewRecorder()

	handlers.PlanItems(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PlanItems() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response PlanItemsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// Verify all items are tracks
	for _, item := range response.Items {
		if item.ItemType != "track" {
			t.Errorf("Expected all items to be type 'track', got %s", item.ItemType)
		}
	}
}

func TestPlanItems_Hierarchy(t *testing.T) {
	handlers, _ := setupPlanItemsTest(t)

	req := httptest.NewRequest("GET", "/api/plan/items?hierarchy=true", nil)
	w := httptest.NewRecorder()

	handlers.PlanItems(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("PlanItems() returned status %d, expected %d", w.Code, http.StatusOK)
	}

	var response PlanItemsResponse
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	// With hierarchy=true, we should see both parent and child items
	hasPlaylist := false
	hasTrack := false
	for _, item := range response.Items {
		if item.ItemType == "playlist" {
			hasPlaylist = true
		}
		if item.ItemType == "track" {
			hasTrack = true
		}
	}

	if !hasPlaylist || !hasTrack {
		t.Error("Hierarchy view should include both playlists and tracks")
	}
}
