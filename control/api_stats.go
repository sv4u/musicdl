package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// StatsTracker tracks both per-run and cumulative download statistics.
type StatsTracker struct {
	mu         sync.RWMutex
	cumulative *CumulativeStats
	currentRun *RunStats
	filePath   string
}

// CumulativeStats holds lifetime statistics persisted to disk.
type CumulativeStats struct {
	TotalDownloaded     int64   `json:"totalDownloaded"`
	TotalFailed         int64   `json:"totalFailed"`
	TotalSkipped        int64   `json:"totalSkipped"`
	TotalPlansGenerated int64   `json:"totalPlansGenerated"`
	TotalRuns           int64   `json:"totalRuns"`
	TotalRateLimits     int64   `json:"totalRateLimits"`
	TotalRetries        int64   `json:"totalRetries"`
	TotalBytesWritten   int64   `json:"totalBytesWritten"`
	TotalTimeSpentSec   float64 `json:"totalTimeSpentSec"`
	PlanTimeSpentSec    float64 `json:"planTimeSpentSec"`
	DownloadTimeSpentSec float64 `json:"downloadTimeSpentSec"`
	FirstRunAt          int64   `json:"firstRunAt"`
	LastRunAt           int64   `json:"lastRunAt"`
	SuccessRate         float64 `json:"successRate"`
}

// RunStats holds statistics for the current running operation.
type RunStats struct {
	RunID         string  `json:"runId"`
	OperationType string  `json:"operationType"`
	StartedAt     int64   `json:"startedAt"`
	Downloaded    int64   `json:"downloaded"`
	Failed        int64   `json:"failed"`
	Skipped       int64   `json:"skipped"`
	Retries       int64   `json:"retries"`
	RateLimits    int64   `json:"rateLimits"`
	BytesWritten  int64   `json:"bytesWritten"`
	ElapsedSec    float64 `json:"elapsedSec"`
	TracksPerHour float64 `json:"tracksPerHour"`
	IsRunning     bool    `json:"isRunning"`
}

// StatsResponse is the combined stats response for the API.
type StatsResponse struct {
	Cumulative *CumulativeStats `json:"cumulative"`
	CurrentRun *RunStats        `json:"currentRun"`
}

// NewStatsTracker creates a new stats tracker, loading cumulative stats from disk.
func NewStatsTracker(cacheDir string) *StatsTracker {
	filePath := filepath.Join(cacheDir, "stats.json")
	st := &StatsTracker{
		filePath: filePath,
		cumulative: &CumulativeStats{},
		currentRun: &RunStats{},
	}
	st.load()
	return st
}

// load reads cumulative stats from disk.
func (st *StatsTracker) load() {
	data, err := os.ReadFile(st.filePath)
	if err != nil {
		return // File doesn't exist yet, start fresh
	}
	var stats CumulativeStats
	if err := json.Unmarshal(data, &stats); err != nil {
		return
	}
	st.cumulative = &stats
}

// save writes cumulative stats to disk.
func (st *StatsTracker) save() {
	dir := filepath.Dir(st.filePath)
	os.MkdirAll(dir, 0755)
	data, err := json.MarshalIndent(st.cumulative, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(st.filePath, data, 0644)
}

// StartRun begins tracking a new run and returns the run ID. If a previous run
// was never finalized via EndRunByID(), it is finalized first so cumulative
// stats are not lost. The returned run ID must be passed to EndRunByID() to
// ensure only the correct run is finalized (preventing the race where a stale
// goroutine's EndRun finalizes a newer run's stats).
func (st *StatsTracker) StartRun(operationType string) string {
	st.mu.Lock()
	defer st.mu.Unlock()
	// Finalize previous run if it was never ended, so cumulative time stats,
	// success rate, and other aggregates are not silently discarded.
	if st.currentRun != nil && st.currentRun.IsRunning {
		st.finalizeCurrent()
	}
	now := time.Now()
	runID := now.Format("20060102_150405") + fmt.Sprintf("_%d", now.UnixNano())
	st.currentRun = &RunStats{
		RunID:         runID,
		OperationType: operationType,
		StartedAt:     now.Unix(),
		IsRunning:     true,
	}
	st.cumulative.TotalRuns++
	if operationType == "plan" {
		st.cumulative.TotalPlansGenerated++
	}
	if st.cumulative.FirstRunAt == 0 {
		st.cumulative.FirstRunAt = now.Unix()
	}
	st.cumulative.LastRunAt = now.Unix()
	st.save()
	return runID
}

// EndRunByID finalizes the current run only if its ID matches the given runID.
// This prevents the race where a goroutine from a completed operation calls
// EndRun after a new operation has already started, which would incorrectly
// finalize the new run's stats.
func (st *StatsTracker) EndRunByID(runID string) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.currentRun == nil || !st.currentRun.IsRunning {
		return
	}
	if st.currentRun.RunID != runID {
		return
	}
	st.finalizeCurrent()
	st.save()
}

// finalizeCurrent updates the current run's elapsed time, marks it as not
// running, and rolls its stats into the cumulative totals. Caller must hold
// st.mu (write lock).
func (st *StatsTracker) finalizeCurrent() {
	elapsed := time.Since(time.Unix(st.currentRun.StartedAt, 0)).Seconds()
	st.currentRun.ElapsedSec = elapsed
	st.currentRun.IsRunning = false
	if elapsed > 0 {
		st.currentRun.TracksPerHour = float64(st.currentRun.Downloaded) / elapsed * 3600
	}
	// Update cumulative
	st.cumulative.TotalTimeSpentSec += elapsed
	if st.currentRun.OperationType == "plan" {
		st.cumulative.PlanTimeSpentSec += elapsed
	} else {
		st.cumulative.DownloadTimeSpentSec += elapsed
	}
	// Recalculate success rate
	total := st.cumulative.TotalDownloaded + st.cumulative.TotalFailed
	if total > 0 {
		st.cumulative.SuccessRate = float64(st.cumulative.TotalDownloaded) / float64(total) * 100.0
	}
}

// RecordDownload records a successful download.
func (st *StatsTracker) RecordDownload(bytesWritten int64) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.currentRun != nil {
		st.currentRun.Downloaded++
		st.currentRun.BytesWritten += bytesWritten
	}
	st.cumulative.TotalDownloaded++
	st.cumulative.TotalBytesWritten += bytesWritten
}

// RecordFailure records a failed download.
func (st *StatsTracker) RecordFailure() {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.currentRun != nil {
		st.currentRun.Failed++
	}
	st.cumulative.TotalFailed++
}

// RecordSkip records a skipped track.
func (st *StatsTracker) RecordSkip() {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.currentRun != nil {
		st.currentRun.Skipped++
	}
	st.cumulative.TotalSkipped++
}

// RecordRetry records a retry attempt.
func (st *StatsTracker) RecordRetry() {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.currentRun != nil {
		st.currentRun.Retries++
	}
	st.cumulative.TotalRetries++
}

// RecordRateLimit records a rate limit hit.
func (st *StatsTracker) RecordRateLimit() {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.currentRun != nil {
		st.currentRun.RateLimits++
	}
	st.cumulative.TotalRateLimits++
	st.save()
}

// GetStats returns the current statistics snapshot.
func (st *StatsTracker) GetStats() StatsResponse {
	st.mu.RLock()
	defer st.mu.RUnlock()
	// Update elapsed time for running operations
	run := *st.currentRun
	if run.IsRunning && run.StartedAt > 0 {
		elapsed := time.Since(time.Unix(run.StartedAt, 0)).Seconds()
		run.ElapsedSec = elapsed
		if elapsed > 0 {
			run.TracksPerHour = float64(run.Downloaded) / elapsed * 3600
		}
	}
	cumul := *st.cumulative
	return StatsResponse{
		Cumulative: &cumul,
		CurrentRun: &run,
	}
}

// Reset resets cumulative statistics. Current run stats are NOT affected.
func (st *StatsTracker) Reset() {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.cumulative = &CumulativeStats{}
	st.save()
}
