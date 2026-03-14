package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sv4u/musicdl/download/history"
)

// newTestServerWithHistory creates an APIServer with a real history tracker
// pointed at a temporary directory. The tracker is returned separately for
// direct manipulation in tests.
func newTestServerWithHistory(t *testing.T) (*APIServer, *history.Tracker) {
	t.Helper()
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")
	tracker, err := history.NewTracker(historyPath, 50, 30)
	if err != nil {
		t.Fatalf("failed to create history tracker: %v", err)
	}
	t.Cleanup(func() { tracker.Close() })

	cacheDir := filepath.Join(tmpDir, "cache")
	os.MkdirAll(cacheDir, 0755)

	server := &APIServer{
		port:              0,
		currentRunTracker: &RunTracker{},
		logBroadcaster:    NewLogBroadcaster(),
		planBroadcaster:   NewPlanBroadcaster(),
		statsTracker:      NewStatsTracker(cacheDir),
		historyTracker:    tracker,
		circuitBreaker:    NewCircuitBreaker(5, 3, 60*time.Second),
		resumeState:       NewResumeState(cacheDir),
	}
	return server, tracker
}

// createTestRun starts and stops a run on the tracker with the given stats.
func createTestRun(t *testing.T, tracker *history.Tracker, runID string, stats map[string]interface{}, state string) {
	t.Helper()
	tracker.StartRun(runID)
	time.Sleep(5 * time.Millisecond)
	errMsg := ""
	if state == "error" {
		errMsg = "test error"
	}
	if err := tracker.StopRun(runID, state, state, stats, errMsg); err != nil {
		t.Fatalf("failed to stop run: %v", err)
	}
}

// --- Unit Tests: /api/history/runs ---

func TestHistoryRunsHandler_Empty(t *testing.T) {
	server, _ := newTestServerWithHistory(t)

	req := httptest.NewRequest(http.MethodGet, "/api/history/runs", nil)
	rec := httptest.NewRecorder()
	server.historyRunsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp HistoryRunsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Runs) != 0 {
		t.Errorf("expected 0 runs, got %d", len(resp.Runs))
	}
	if resp.TotalRuns != 0 {
		t.Errorf("expected totalRuns 0, got %d", resp.TotalRuns)
	}
}

func TestHistoryRunsHandler_WithRuns(t *testing.T) {
	server, tracker := newTestServerWithHistory(t)

	stats := map[string]interface{}{"downloaded": 10, "failed": 1}
	createTestRun(t, tracker, "run_001", stats, "completed")
	createTestRun(t, tracker, "run_002", stats, "completed")
	createTestRun(t, tracker, "run_003", stats, "error")

	req := httptest.NewRequest(http.MethodGet, "/api/history/runs", nil)
	rec := httptest.NewRecorder()
	server.historyRunsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp HistoryRunsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Runs) != 3 {
		t.Errorf("expected 3 runs, got %d", len(resp.Runs))
	}
	if resp.TotalRuns != 3 {
		t.Errorf("expected totalRuns 3, got %d", resp.TotalRuns)
	}

	for _, run := range resp.Runs {
		if run.RunID == "" {
			t.Error("expected non-empty runId")
		}
		if run.StartedAt == "" {
			t.Error("expected non-empty startedAt")
		}
		if run.State == "" {
			t.Error("expected non-empty state")
		}
	}

	hasErrorRun := false
	for _, run := range resp.Runs {
		if run.State == "error" {
			hasErrorRun = true
			if run.Error == "" {
				t.Error("expected non-empty error on failed run")
			}
		}
	}
	if !hasErrorRun {
		t.Error("expected at least one run with state 'error'")
	}
}

func TestHistoryRunsHandler_LimitParam(t *testing.T) {
	server, tracker := newTestServerWithHistory(t)

	stats := map[string]interface{}{"downloaded": 5}
	for i := 0; i < 5; i++ {
		createTestRun(t, tracker, "run_limit_"+string(rune('a'+i)), stats, "completed")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/history/runs?limit=2", nil)
	rec := httptest.NewRecorder()
	server.historyRunsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp HistoryRunsResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Runs) != 2 {
		t.Errorf("expected 2 runs, got %d", len(resp.Runs))
	}
	if resp.TotalRuns != 5 {
		t.Errorf("expected totalRuns 5, got %d", resp.TotalRuns)
	}
}

// --- Unit Tests: /api/history/runs/{runID} ---

func TestHistoryRunDetailHandler_Found(t *testing.T) {
	server, tracker := newTestServerWithHistory(t)

	tracker.StartRun("detail_run")
	tracker.AddSnapshot(25.0, map[string]interface{}{"completed": 5, "total": 20}, "running", "executing")
	tracker.AddSnapshot(50.0, map[string]interface{}{"completed": 10, "total": 20}, "running", "executing")
	tracker.AddSnapshot(75.0, map[string]interface{}{"completed": 15, "total": 20}, "running", "executing")
	tracker.StopRun("detail_run", "completed", "completed", map[string]interface{}{"downloaded": 20}, "")

	req := httptest.NewRequest(http.MethodGet, "/api/history/runs/detail_run", nil)
	req.SetPathValue("runID", "detail_run")
	rec := httptest.NewRecorder()
	server.historyRunDetailHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var run history.RunHistory
	if err := json.NewDecoder(rec.Body).Decode(&run); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if run.RunID != "detail_run" {
		t.Errorf("expected runId 'detail_run', got %q", run.RunID)
	}
	if len(run.Snapshots) != 3 {
		t.Errorf("expected 3 snapshots, got %d", len(run.Snapshots))
	}
	if run.State != "completed" {
		t.Errorf("expected state 'completed', got %q", run.State)
	}
	if run.CompletedAt == nil {
		t.Error("expected non-nil completedAt")
	}
}

func TestHistoryRunDetailHandler_NotFound(t *testing.T) {
	server, _ := newTestServerWithHistory(t)

	req := httptest.NewRequest(http.MethodGet, "/api/history/runs/nonexistent", nil)
	req.SetPathValue("runID", "nonexistent")
	rec := httptest.NewRecorder()
	server.historyRunDetailHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if resp["error"] != "Run not found" {
		t.Errorf("expected error 'Run not found', got %q", resp["error"])
	}
}

// --- Unit Tests: /api/history/activity ---

func TestHistoryActivityHandler_Empty(t *testing.T) {
	server, _ := newTestServerWithHistory(t)

	req := httptest.NewRequest(http.MethodGet, "/api/history/activity", nil)
	rec := httptest.NewRecorder()
	server.historyActivityHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp history.ActivityHistory
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Entries) != 0 {
		t.Errorf("expected 0 entries, got %d", len(resp.Entries))
	}
}

func TestHistoryActivityHandler_WithEntries(t *testing.T) {
	server, tracker := newTestServerWithHistory(t)

	createTestRun(t, tracker, "activity_run", map[string]interface{}{"downloaded": 5}, "completed")

	req := httptest.NewRequest(http.MethodGet, "/api/history/activity", nil)
	rec := httptest.NewRecorder()
	server.historyActivityHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp history.ActivityHistory
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Entries) < 2 {
		t.Fatalf("expected at least 2 entries (started + completed), got %d", len(resp.Entries))
	}

	hasStarted := false
	hasCompleted := false
	for _, entry := range resp.Entries {
		if entry.Type == "download_started" {
			hasStarted = true
		}
		if entry.Type == "download_completed" {
			hasCompleted = true
		}
		if entry.ID == "" {
			t.Error("expected non-empty activity ID")
		}
		if entry.Timestamp.IsZero() {
			t.Error("expected non-zero timestamp")
		}
		if entry.Message == "" {
			t.Error("expected non-empty message")
		}
	}
	if !hasStarted {
		t.Error("expected a 'download_started' activity entry")
	}
	if !hasCompleted {
		t.Error("expected a 'download_completed' activity entry")
	}
}

func TestHistoryActivityHandler_LimitParam(t *testing.T) {
	server, tracker := newTestServerWithHistory(t)

	for i := 0; i < 5; i++ {
		createTestRun(t, tracker, "limit_run_"+string(rune('a'+i)), map[string]interface{}{}, "completed")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/history/activity?limit=3", nil)
	rec := httptest.NewRecorder()
	server.historyActivityHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var resp history.ActivityHistory
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if len(resp.Entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(resp.Entries))
	}
}

// --- Behavioral Tests ---

func TestHistoryRunsHandler_ResponseFormat(t *testing.T) {
	server, tracker := newTestServerWithHistory(t)
	createTestRun(t, tracker, "format_run", map[string]interface{}{"downloaded": 1}, "completed")

	req := httptest.NewRequest(http.MethodGet, "/api/history/runs", nil)
	rec := httptest.NewRecorder()
	server.historyRunsHandler(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got %q", ct)
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(rec.Body).Decode(&raw); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if _, ok := raw["runs"]; !ok {
		t.Error("response missing 'runs' field")
	}
	if _, ok := raw["totalRuns"]; !ok {
		t.Error("response missing 'totalRuns' field")
	}

	var resp HistoryRunsResponse
	json.Unmarshal(raw["runs"], &resp.Runs)
	if len(resp.Runs) == 0 {
		t.Fatal("expected at least 1 run")
	}
	run := resp.Runs[0]
	if run.StartedAt == "" {
		t.Error("startedAt must not be empty")
	}
	if _, err := time.Parse(time.RFC3339, run.StartedAt); err != nil {
		t.Errorf("startedAt is not valid RFC3339: %q", run.StartedAt)
	}
	if run.CompletedAt != "" {
		if _, err := time.Parse(time.RFC3339, run.CompletedAt); err != nil {
			t.Errorf("completedAt is not valid RFC3339: %q", run.CompletedAt)
		}
	}
}

func TestHistoryRunsHandler_InvalidLimit(t *testing.T) {
	server, _ := newTestServerWithHistory(t)

	tests := []struct {
		name  string
		query string
	}{
		{"negative", "?limit=-1"},
		{"non-numeric", "?limit=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/history/runs"+tt.query, nil)
			rec := httptest.NewRecorder()
			server.historyRunsHandler(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for %s, got %d", tt.query, rec.Code)
			}
		})
	}
}

// --- Integration Tests ---

func TestHistoryTracker_RetentionCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")
	tracker, err := history.NewTracker(historyPath, 3, 30)
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}
	defer tracker.Close()

	for i := 0; i < 5; i++ {
		createTestRun(t, tracker, "retention_"+string(rune('a'+i)),
			map[string]interface{}{"downloaded": i}, "completed")
	}

	runs, err := tracker.ListRuns()
	if err != nil {
		t.Fatalf("ListRuns failed: %v", err)
	}
	if len(runs) != 3 {
		t.Errorf("expected 3 runs after retention cleanup, got %d", len(runs))
	}

	entries, err := os.ReadDir(historyPath)
	if err != nil {
		t.Fatalf("failed to read history dir: %v", err)
	}
	runFileCount := 0
	for _, entry := range entries {
		if !entry.IsDir() && len(entry.Name()) > 4 && entry.Name()[:4] == "run_" {
			runFileCount++
		}
	}
	if runFileCount != 3 {
		t.Errorf("expected 3 run files on disk, got %d", runFileCount)
	}
}

func TestHistoryTracker_GracefulShutdown(t *testing.T) {
	tmpDir := t.TempDir()
	historyPath := filepath.Join(tmpDir, "history")
	tracker, err := history.NewTracker(historyPath, 50, 30)
	if err != nil {
		t.Fatalf("failed to create tracker: %v", err)
	}

	tracker.AddActivity("test_event", "Test activity entry", map[string]interface{}{"key": "value"})

	if err := tracker.Close(); err != nil {
		t.Fatalf("Close() failed: %v", err)
	}

	activityPath := filepath.Join(historyPath, "activity.json")
	data, err := os.ReadFile(activityPath)
	if err != nil {
		t.Fatalf("activity.json not found after Close(): %v", err)
	}

	var activity history.ActivityHistory
	if err := activity.FromJSON(data); err != nil {
		t.Fatalf("failed to parse activity.json: %v", err)
	}
	if len(activity.Entries) == 0 {
		t.Error("expected at least 1 activity entry after Close()")
	}
}

func TestDownloadLifecycle_RecordsHistory(t *testing.T) {
	server, tracker := newTestServerWithHistory(t)

	runID := server.statsTracker.StartRun("download")
	tracker.StartRun(runID)

	tracker.AddSnapshot(50.0, map[string]interface{}{
		"completed": 5, "failed": 0, "total": 10,
	}, "running", "executing")

	server.statsTracker.RecordDownload(1024)
	server.statsTracker.RecordDownload(2048)
	server.statsTracker.RecordFailure()

	runStats := server.buildRunStatistics()
	tracker.StopRun(runID, "completed", "completed", runStats, "")
	server.statsTracker.EndRunByID(runID)

	req := httptest.NewRequest(http.MethodGet, "/api/history/runs", nil)
	rec := httptest.NewRecorder()
	server.historyRunsHandler(rec, req)

	var resp HistoryRunsResponse
	json.NewDecoder(rec.Body).Decode(&resp)
	if len(resp.Runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(resp.Runs))
	}

	run := resp.Runs[0]
	if run.State != "completed" {
		t.Errorf("expected state 'completed', got %q", run.State)
	}
	if run.SnapshotCount != 1 {
		t.Errorf("expected 1 snapshot, got %d", run.SnapshotCount)
	}
	if run.Statistics == nil {
		t.Fatal("expected non-nil statistics")
	}
	downloaded, _ := run.Statistics["downloaded"].(float64)
	if downloaded != 2 {
		t.Errorf("expected downloaded=2, got %v", downloaded)
	}
	failed, _ := run.Statistics["failed"].(float64)
	if failed != 1 {
		t.Errorf("expected failed=1, got %v", failed)
	}

	actReq := httptest.NewRequest(http.MethodGet, "/api/history/activity", nil)
	actRec := httptest.NewRecorder()
	server.historyActivityHandler(actRec, actReq)

	var actResp history.ActivityHistory
	json.NewDecoder(actRec.Body).Decode(&actResp)

	hasStarted := false
	hasCompleted := false
	for _, entry := range actResp.Entries {
		if entry.Type == "download_started" {
			hasStarted = true
		}
		if entry.Type == "download_completed" {
			hasCompleted = true
		}
	}
	if !hasStarted {
		t.Error("expected download_started activity")
	}
	if !hasCompleted {
		t.Error("expected download_completed activity")
	}
}
