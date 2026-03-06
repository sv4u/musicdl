package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sv4u/musicdl/download/plan"
)

func TestPlanBroadcaster_NewAndClientCount(t *testing.T) {
	pb := NewPlanBroadcaster()

	if pb.ClientCount() != 0 {
		t.Errorf("expected ClientCount() == 0, got %d", pb.ClientCount())
	}
}

func TestPlanBroadcaster_SetPlan(t *testing.T) {
	pb := NewPlanBroadcaster()

	dp := plan.NewDownloadPlan(nil)
	dp.AddItem(&plan.PlanItem{
		ItemID:   "track:abc",
		ItemType: plan.PlanItemTypeTrack,
		Name:     "Track One",
		Status:   plan.PlanItemStatusPending,
	})
	dp.AddItem(&plan.PlanItem{
		ItemID:   "track:def",
		ItemType: plan.PlanItemTypeTrack,
		Name:     "Track Two",
		Status:   plan.PlanItemStatusPending,
	})
	dp.AddItem(&plan.PlanItem{
		ItemID:   "playlist:xyz",
		ItemType: plan.PlanItemTypePlaylist,
		Name:     "Test Playlist",
		Status:   plan.PlanItemStatusPending,
	})

	pb.SetPlan(dp, "testhash123")

	snapshot := pb.GetSnapshot()
	if snapshot == nil {
		t.Fatal("expected snapshot to be non-nil")
	}
	if snapshot.ConfigHash != "testhash123" {
		t.Errorf("expected ConfigHash 'testhash123', got %q", snapshot.ConfigHash)
	}
	if len(snapshot.Items) != 3 {
		t.Errorf("expected 3 items, got %d", len(snapshot.Items))
	}
	// computePlanStats counts all item types, not only tracks
	if snapshot.Stats.Total != 3 {
		t.Errorf("expected Stats.Total == 3, got %d", snapshot.Stats.Total)
	}
	if snapshot.Stats.Pending != 3 {
		t.Errorf("expected Stats.Pending == 3, got %d", snapshot.Stats.Pending)
	}
}

func TestPlanBroadcaster_BroadcastPhaseChange(t *testing.T) {
	pb := NewPlanBroadcaster()

	dp := plan.NewDownloadPlan(nil)
	dp.AddItem(&plan.PlanItem{
		ItemID:   "track:abc",
		ItemType: plan.PlanItemTypeTrack,
		Name:     "Test Track",
		Status:   plan.PlanItemStatusPending,
	})
	pb.SetPlan(dp, "hash")

	pb.BroadcastPhaseChange("downloading")

	snapshot := pb.GetSnapshot()
	if snapshot == nil {
		t.Fatal("expected snapshot to be non-nil")
	}
	if snapshot.Phase != "downloading" {
		t.Errorf("expected Phase 'downloading', got %q", snapshot.Phase)
	}
}

func TestPlanBroadcaster_BroadcastItemUpdate(t *testing.T) {
	pb := NewPlanBroadcaster()

	dp := plan.NewDownloadPlan(nil)
	dp.AddItem(&plan.PlanItem{
		ItemID:   "track:abc",
		ItemType: plan.PlanItemTypeTrack,
		Name:     "Test Track",
		Status:   plan.PlanItemStatusPending,
	})
	pb.SetPlan(dp, "hash")

	updatedItem := &plan.PlanItem{
		ItemID:   "track:abc",
		ItemType: plan.PlanItemTypeTrack,
		Name:     "Test Track",
		Status:   plan.PlanItemStatusPending,
	}
	updatedItem.MarkCompleted("/path/to/file.mp3")

	pb.BroadcastItemUpdate(updatedItem)

	snapshot := pb.GetSnapshot()
	if snapshot == nil {
		t.Fatal("expected snapshot to be non-nil")
	}

	var found bool
	for _, item := range snapshot.Items {
		if item.ItemID == "track:abc" {
			found = true
			if item.Status != "completed" {
				t.Errorf("expected status 'completed', got %q", item.Status)
			}
			break
		}
	}
	if !found {
		t.Error("item 'track:abc' not found in snapshot")
	}
}

func TestPlanBroadcaster_WebSocket(t *testing.T) {
	pb := NewPlanBroadcaster()

	dp := plan.NewDownloadPlan(nil)
	dp.AddItem(&plan.PlanItem{
		ItemID:   "track:abc",
		ItemType: plan.PlanItemTypeTrack,
		Name:     "Test Track",
		Status:   plan.PlanItemStatusPending,
	})
	pb.SetPlan(dp, "testhash")

	server := httptest.NewServer(http.HandlerFunc(pb.HandleWebSocket))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message failed: %v", err)
	}

	var planMsg PlanMessage
	if err := json.Unmarshal(msg, &planMsg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if planMsg.Type != "plan_loaded" {
		t.Errorf("expected type 'plan_loaded', got %q", planMsg.Type)
	}
	if planMsg.Plan == nil {
		t.Fatal("expected plan to be non-nil")
	}
	if len(planMsg.Plan.Items) != 1 {
		t.Errorf("expected 1 item in plan, got %d", len(planMsg.Plan.Items))
	}

	conn.Close()
	time.Sleep(200 * time.Millisecond)

	if pb.ClientCount() != 0 {
		t.Errorf("expected ClientCount() == 0 after close, got %d", pb.ClientCount())
	}
}

func TestAPIServer_PlanSnapshotEndpoint(t *testing.T) {
	srv := NewAPIServer(0)

	dp := plan.NewDownloadPlan(nil)
	dp.AddItem(&plan.PlanItem{
		ItemID:   "track:endpoint-test",
		ItemType: plan.PlanItemTypeTrack,
		Name:     "Endpoint Test Track",
		Status:   plan.PlanItemStatusCompleted,
	})
	dp.AddItem(&plan.PlanItem{
		ItemID:   "track:endpoint-test-2",
		ItemType: plan.PlanItemTypeTrack,
		Name:     "Endpoint Test Track 2",
		Status:   plan.PlanItemStatusFailed,
		Error:    "download failed",
	})
	srv.planBroadcaster.SetPlan(dp, "endpoint-hash")

	req := httptest.NewRequest("GET", "/api/plan", nil)
	w := httptest.NewRecorder()
	srv.planSnapshotHandler(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var snapshot PlanSnapshot
	if err := json.NewDecoder(w.Body).Decode(&snapshot); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(snapshot.Items) != 2 {
		t.Errorf("expected 2 items, got %d", len(snapshot.Items))
	}
	if snapshot.ConfigHash != "endpoint-hash" {
		t.Errorf("expected config hash 'endpoint-hash', got %q", snapshot.ConfigHash)
	}

	var completedCount, failedCount int
	for _, item := range snapshot.Items {
		switch item.Status {
		case "completed":
			completedCount++
		case "failed":
			failedCount++
		}
	}
	if completedCount != 1 {
		t.Errorf("expected 1 completed item, got %d", completedCount)
	}
	if failedCount != 1 {
		t.Errorf("expected 1 failed item, got %d", failedCount)
	}
}

func TestAPIServer_PlanWebSocketEndpoint(t *testing.T) {
	srv := NewAPIServer(0)

	dp := plan.NewDownloadPlan(nil)
	dp.AddItem(&plan.PlanItem{
		ItemID:   "track:ws-endpoint",
		ItemType: plan.PlanItemTypeTrack,
		Name:     "WS Test",
		Status:   plan.PlanItemStatusPending,
	})
	srv.planBroadcaster.SetPlan(dp, "ws-hash")

	server := httptest.NewServer(http.HandlerFunc(srv.wsPlanHandler))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("websocket dial failed: %v", err)
	}
	defer conn.Close()

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message failed: %v", err)
	}

	var planMsg PlanMessage
	if err := json.Unmarshal(msg, &planMsg); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if planMsg.Type != "plan_loaded" {
		t.Errorf("expected 'plan_loaded', got %q", planMsg.Type)
	}
	if planMsg.Plan == nil || len(planMsg.Plan.Items) != 1 {
		t.Error("expected plan with 1 item")
	}
}
