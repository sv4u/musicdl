package main

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// HistoryRunSummary is a lightweight representation of a run for the list endpoint.
// It omits the full snapshots array to keep the response small.
type HistoryRunSummary struct {
	RunID         string                 `json:"runId"`
	StartedAt     string                 `json:"startedAt"`
	CompletedAt   string                 `json:"completedAt,omitempty"`
	State         string                 `json:"state"`
	Phase         string                 `json:"phase"`
	Statistics    map[string]interface{} `json:"statistics"`
	Error         string                 `json:"error,omitempty"`
	SnapshotCount int                    `json:"snapshotCount"`
}

// HistoryRunsResponse is the response for GET /api/history/runs.
type HistoryRunsResponse struct {
	Runs      []HistoryRunSummary `json:"runs"`
	TotalRuns int                 `json:"totalRuns"`
}

// historyRunsHandler returns a summary list of all stored runs.
// @Summary List run history
// @Description Returns a summary list of past download/plan runs, sorted newest-first.
// @Tags history
// @Produce json
// @Param limit query int false "Max runs to return (default 20)"
// @Success 200 {object} HistoryRunsResponse
// @Failure 400 {object} map[string]string
// @Failure 503 {object} map[string]string "History tracking not available"
// @Router /api/history/runs [get]
func (s *APIServer) historyRunsHandler(w http.ResponseWriter, r *http.Request) {
	if s.historyTracker == nil {
		jsonError(w, "History tracking is not available", http.StatusServiceUnavailable)
		return
	}

	limit := 20
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 0 {
			jsonError(w, "Invalid limit parameter: must be a non-negative integer", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	runIDs, err := s.historyTracker.ListRuns()
	if err != nil {
		jsonError(w, "Failed to list runs", http.StatusInternalServerError)
		return
	}

	totalRuns := len(runIDs)

	if limit > 0 && limit < len(runIDs) {
		runIDs = runIDs[:limit]
	}

	summaries := make([]HistoryRunSummary, 0, len(runIDs))
	for _, runID := range runIDs {
		run, loadErr := s.historyTracker.GetRunHistory(runID)
		if loadErr != nil {
			continue
		}

		summary := HistoryRunSummary{
			RunID:         run.RunID,
			StartedAt:     run.StartedAt.Format("2006-01-02T15:04:05Z07:00"),
			State:         run.State,
			Phase:         run.Phase,
			Statistics:    run.Statistics,
			Error:         run.Error,
			SnapshotCount: len(run.Snapshots),
		}
		if run.CompletedAt != nil {
			summary.CompletedAt = run.CompletedAt.Format("2006-01-02T15:04:05Z07:00")
		}

		summaries = append(summaries, summary)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(HistoryRunsResponse{
		Runs:      summaries,
		TotalRuns: totalRuns,
	})
}

// historyRunDetailHandler returns the full history for a specific run.
// @Summary Get run detail
// @Description Returns the full run history including all progress snapshots.
// @Tags history
// @Produce json
// @Param runID path string true "Run ID"
// @Success 200 {object} history.RunHistory
// @Failure 404 {object} map[string]string
// @Failure 503 {object} map[string]string "History tracking not available"
// @Router /api/history/runs/{runID} [get]
func (s *APIServer) historyRunDetailHandler(w http.ResponseWriter, r *http.Request) {
	if s.historyTracker == nil {
		jsonError(w, "History tracking is not available", http.StatusServiceUnavailable)
		return
	}

	runID := r.PathValue("runID")
	if runID == "" {
		jsonError(w, "Run ID is required", http.StatusBadRequest)
		return
	}

	run, err := s.historyTracker.GetRunHistory(runID)
	if err != nil {
		jsonError(w, "Run not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}

// historyActivityHandler returns the activity log.
// @Summary Get activity log
// @Description Returns recent activity entries (download started, completed, failed, etc.)
// @Tags history
// @Produce json
// @Param limit query int false "Max entries to return (default 50)"
// @Success 200 {object} history.ActivityHistory
// @Failure 400 {object} map[string]string
// @Failure 503 {object} map[string]string "History tracking not available"
// @Router /api/history/activity [get]
func (s *APIServer) historyActivityHandler(w http.ResponseWriter, r *http.Request) {
	if s.historyTracker == nil {
		jsonError(w, "History tracking is not available", http.StatusServiceUnavailable)
		return
	}

	limit := 50
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		parsed, err := strconv.Atoi(limitStr)
		if err != nil || parsed < 0 {
			jsonError(w, "Invalid limit parameter: must be a non-negative integer", http.StatusBadRequest)
			return
		}
		limit = parsed
	}

	activity := s.historyTracker.GetActivityHistory(limit)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(activity)
}
