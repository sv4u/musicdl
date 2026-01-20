package history

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Tracker manages history tracking for download runs.
type Tracker struct {
	historyPath      string
	retention        int
	snapshotInterval time.Duration
	activityPath     string

	// Current run tracking
	currentRun   *RunHistory
	currentRunMu sync.RWMutex

	// Activity tracking
	activityHistory *ActivityHistory
	activityMu      sync.RWMutex

	// Snapshot ticker
	snapshotTicker *time.Ticker
	snapshotStop   chan struct{}
	snapshotWg     sync.WaitGroup // WaitGroup to ensure goroutine exits before creating new one
	snapshotMu     sync.Mutex
}

// NewTracker creates a new history tracker.
func NewTracker(historyPath string, retention int, snapshotInterval int) (*Tracker, error) {
	// Validate snapshotInterval: must be positive (time.NewTicker panics on non-positive duration)
	if snapshotInterval <= 0 {
		return nil, fmt.Errorf("snapshotInterval must be positive, got %d", snapshotInterval)
	}

	// Ensure history directory exists
	if err := os.MkdirAll(historyPath, 0755); err != nil {
		return nil, fmt.Errorf("failed to create history directory: %w", err)
	}

	tracker := &Tracker{
		historyPath:      historyPath,
		retention:        retention,
		snapshotInterval: time.Duration(snapshotInterval) * time.Second,
		activityPath:     filepath.Join(historyPath, "activity.json"),
		activityHistory:  &ActivityHistory{Entries: make([]ActivityEntry, 0)},
		snapshotStop:     make(chan struct{}),
	}

	// Load existing activity history
	if err := tracker.loadActivityHistory(); err != nil {
		log.Printf("WARN: failed to load activity history: %v", err)
	}

	return tracker, nil
}

// StartRun starts tracking a new run.
func (t *Tracker) StartRun(runID string) {
	t.currentRunMu.Lock()
	defer t.currentRunMu.Unlock()

	now := time.Now()
	t.currentRun = &RunHistory{
		RunID:     runID,
		StartedAt: now,
		State:     "running",
		Phase:     "executing",
		Snapshots: make([]RunSnapshot, 0),
	}

	// Start snapshot ticker
	t.startSnapshotTicker()

	// Add activity entry
	t.addActivity("download_started", fmt.Sprintf("Download started (run_id: %s)", runID), map[string]interface{}{
		"run_id": runID,
	})
}

// StopRun stops tracking the current run and saves it.
func (t *Tracker) StopRun(state, phase string, statistics map[string]interface{}, errMsg string) error {
	t.currentRunMu.Lock()
	defer t.currentRunMu.Unlock()

	if t.currentRun == nil {
		return nil // No run to stop
	}

	// Stop snapshot ticker
	t.stopSnapshotTicker()

	now := time.Now()
	t.currentRun.CompletedAt = &now
	t.currentRun.State = state
	t.currentRun.Phase = phase
	t.currentRun.Statistics = statistics
	if errMsg != "" {
		t.currentRun.Error = errMsg
	}

	// Save run history
	if err := t.saveRunHistory(t.currentRun); err != nil {
		log.Printf("ERROR: failed to save run history: %v", err)
	}

	// Add activity entry
	activityType := "download_completed"
	if state == "error" {
		activityType = "download_failed"
	}
	t.addActivity(activityType, fmt.Sprintf("Download %s (run_id: %s)", state, t.currentRun.RunID), map[string]interface{}{
		"run_id":     t.currentRun.RunID,
		"state":      state,
		"statistics": statistics,
	})

	// Cleanup old runs if retention is enabled
	if t.retention > 0 {
		if err := t.cleanupOldRuns(); err != nil {
			log.Printf("WARN: failed to cleanup old runs: %v", err)
		}
	}

	t.currentRun = nil
	return nil
}

// AddSnapshot adds a progress snapshot to the current run.
func (t *Tracker) AddSnapshot(progress float64, statistics map[string]interface{}, state, phase string) {
	// Create snapshot first (no lock needed)
	snapshot := RunSnapshot{
		Timestamp:  time.Now(),
		Progress:   progress,
		Statistics: statistics,
		State:      state,
		Phase:      phase,
	}

	// Hold lock for entire check-and-append operation to prevent TOCTOU race
	t.currentRunMu.Lock()
	defer t.currentRunMu.Unlock()
	
	if t.currentRun == nil {
		return
	}
	
	t.currentRun.Snapshots = append(t.currentRun.Snapshots, snapshot)
	
	// Limit snapshots to prevent unbounded memory growth
	// Keep only the most recent 10000 snapshots per run
	const maxSnapshots = 10000
	if len(t.currentRun.Snapshots) > maxSnapshots {
		// Remove oldest snapshots, keeping the most recent ones
		excess := len(t.currentRun.Snapshots) - maxSnapshots
		t.currentRun.Snapshots = t.currentRun.Snapshots[excess:]
	}
}

// GetCurrentRun returns the current run history.
func (t *Tracker) GetCurrentRun() *RunHistory {
	t.currentRunMu.RLock()
	defer t.currentRunMu.RUnlock()
	if t.currentRun == nil {
		return nil
	}
	// Return a copy
	runCopy := *t.currentRun
	return &runCopy
}

// GetRunHistory loads a specific run history by ID.
func (t *Tracker) GetRunHistory(runID string) (*RunHistory, error) {
	runPath := filepath.Join(t.historyPath, fmt.Sprintf("run_%s.json", runID))
	data, err := os.ReadFile(runPath)
	if err != nil {
		return nil, err
	}

	var run RunHistory
	if err := run.FromJSON(data); err != nil {
		return nil, err
	}

	return &run, nil
}

// ListRuns returns a list of all run IDs, sorted by start time (newest first).
func (t *Tracker) ListRuns() ([]string, error) {
	entries, err := os.ReadDir(t.historyPath)
	if err != nil {
		return nil, err
	}

	var runIDs []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if len(name) > 8 && name[:4] == "run_" && name[len(name)-5:] == ".json" {
			runID := name[4 : len(name)-5]
			runIDs = append(runIDs, runID)
		}
	}

	// Sort by start time (load each run to get start time)
	type runInfo struct {
		ID        string
		StartedAt time.Time
	}
	var runInfos []runInfo
	for _, runID := range runIDs {
		run, err := t.GetRunHistory(runID)
		if err != nil {
			continue
		}
		runInfos = append(runInfos, runInfo{
			ID:        runID,
			StartedAt: run.StartedAt,
		})
	}

	// Sort by StartedAt descending
	for i := 0; i < len(runInfos)-1; i++ {
		for j := i + 1; j < len(runInfos); j++ {
			if runInfos[i].StartedAt.Before(runInfos[j].StartedAt) {
				runInfos[i], runInfos[j] = runInfos[j], runInfos[i]
			}
		}
	}

	result := make([]string, len(runInfos))
	for i, info := range runInfos {
		result[i] = info.ID
	}

	return result, nil
}

// GetActivityHistory returns the activity history.
func (t *Tracker) GetActivityHistory(limit int) *ActivityHistory {
	t.activityMu.RLock()
	defer t.activityMu.RUnlock()

	history := &ActivityHistory{
		Entries: make([]ActivityEntry, 0),
	}

	entries := t.activityHistory.Entries
	if limit > 0 && limit < len(entries) {
		// Return most recent entries
		entries = entries[len(entries)-limit:]
	}

	history.Entries = entries
	return history
}

// AddActivity adds an activity entry.
func (t *Tracker) AddActivity(activityType, message string, details map[string]interface{}) {
	t.addActivity(activityType, message, details)
}

// addActivity is the internal method to add activity.
func (t *Tracker) addActivity(activityType, message string, details map[string]interface{}) {
	entry := ActivityEntry{
		ID:        fmt.Sprintf("%d", time.Now().UnixNano()),
		Timestamp: time.Now(),
		Type:      activityType,
		Message:   message,
		Details:   details,
	}

	t.activityMu.Lock()
	t.activityHistory.Entries = append(t.activityHistory.Entries, entry)
	// Keep only last 1000 entries in memory
	if len(t.activityHistory.Entries) > 1000 {
		t.activityHistory.Entries = t.activityHistory.Entries[len(t.activityHistory.Entries)-1000:]
	}
	t.activityMu.Unlock()

	// Save activity history
	if err := t.saveActivityHistory(); err != nil {
		log.Printf("WARN: failed to save activity history: %v", err)
	}
}

// saveRunHistory saves a run history to disk.
func (t *Tracker) saveRunHistory(run *RunHistory) error {
	runPath := filepath.Join(t.historyPath, fmt.Sprintf("run_%s.json", run.RunID))
	data, err := run.ToJSON()
	if err != nil {
		return err
	}
	return os.WriteFile(runPath, data, 0644)
}

// loadActivityHistory loads activity history from disk.
func (t *Tracker) loadActivityHistory() error {
	if _, err := os.Stat(t.activityPath); os.IsNotExist(err) {
		return nil // File doesn't exist yet, that's okay
	}

	data, err := os.ReadFile(t.activityPath)
	if err != nil {
		return err
	}

	var history ActivityHistory
	if err := history.FromJSON(data); err != nil {
		return err
	}

	t.activityMu.Lock()
	t.activityHistory = &history
	t.activityMu.Unlock()

	return nil
}

// saveActivityHistory saves activity history to disk.
func (t *Tracker) saveActivityHistory() error {
	t.activityMu.RLock()
	data, err := t.activityHistory.ToJSON()
	t.activityMu.RUnlock()
	if err != nil {
		return err
	}
	return os.WriteFile(t.activityPath, data, 0644)
}

// startSnapshotTicker starts the snapshot ticker.
func (t *Tracker) startSnapshotTicker() {
	t.snapshotMu.Lock()
	defer t.snapshotMu.Unlock()

	if t.snapshotTicker != nil {
		return // Already running
	}

	t.snapshotTicker = time.NewTicker(t.snapshotInterval)
	tickerChan := t.snapshotTicker.C // Capture channel reference before starting goroutine
	stopChan := t.snapshotStop      // Capture stop channel reference
	
	t.snapshotWg.Add(1)
	go func() {
		defer t.snapshotWg.Done()
		for {
			select {
			case <-tickerChan:
				// Ticker fired, check if we should continue
				t.currentRunMu.RLock()
				if t.currentRun != nil {
					// Get current statistics from service (will be passed via AddSnapshot)
					// For now, we'll rely on explicit AddSnapshot calls
				}
				t.currentRunMu.RUnlock()
			case <-stopChan:
				return
			}
		}
	}()
}

// stopSnapshotTicker stops the snapshot ticker.
func (t *Tracker) stopSnapshotTicker() {
	t.snapshotMu.Lock()
	defer t.snapshotMu.Unlock()

	if t.snapshotTicker != nil {
		t.snapshotTicker.Stop()
		t.snapshotTicker = nil
		close(t.snapshotStop)
		// Wait for goroutine to exit before creating new channel
		t.snapshotMu.Unlock()
		t.snapshotWg.Wait()
		t.snapshotMu.Lock()
		t.snapshotStop = make(chan struct{})
	}
}

// cleanupOldRuns removes old runs if retention limit is reached.
func (t *Tracker) cleanupOldRuns() error {
	runIDs, err := t.ListRuns()
	if err != nil {
		return err
	}

	if len(runIDs) <= t.retention {
		return nil // No cleanup needed
	}

	// Remove oldest runs
	runsToRemove := runIDs[t.retention:]
	for _, runID := range runsToRemove {
		runPath := filepath.Join(t.historyPath, fmt.Sprintf("run_%s.json", runID))
		if err := os.Remove(runPath); err != nil {
			log.Printf("WARN: failed to remove old run %s: %v", runID, err)
		}
	}

	return nil
}

// Close closes the tracker and saves any pending data.
func (t *Tracker) Close() error {
	t.stopSnapshotTicker()
	return t.saveActivityHistory()
}
