package mcp

import (
	"time"

	"github.com/sv4u/musicdl/download/history"
	"github.com/sv4u/musicdl/download/plan"
)

// DataProvider abstracts data access for the MCP server.
// FileDataProvider reads everything from disk (standalone mode).
// RuntimeDataProvider can be layered on top for live in-process data (embedded mode).
type DataProvider interface {
	// Plan
	GetCurrentPlan() (*plan.DownloadPlan, string, error)
	ListPlanFiles() ([]PlanFileSummary, error)

	// Download status
	GetDownloadStatus() (*DownloadStatusInfo, error)

	// Stats
	GetStats() (*StatsInfo, error)

	// Logs
	SearchLogs(filter LogFilter) ([]LogEntryInfo, error)
	GetRecentLogs(count int) ([]LogEntryInfo, error)
	ListRunLogDirs() ([]RunLogDir, error)

	// Config
	GetConfigRaw() (string, error)

	// History
	ListRuns(limit int) ([]RunSummaryInfo, error)
	GetRunDetails(runID string) (*history.RunHistory, error)
	GetActivity(limit int) ([]history.ActivityEntry, error)

	// Health
	GetHealth() (*HealthInfo, error)
	GetRecoveryStatus() (*RecoveryInfo, error)

	// Cache
	GetCacheInfo() (*CacheInfo, error)

	// Library
	BrowseLibrary(subpath string) ([]LibraryEntry, error)
	SearchLibrary(query string) ([]LibraryEntry, error)
	GetLibraryStats() (*LibraryStatsInfo, error)

	// Plex
	GetPlexSyncStatus() (*PlexSyncStatusInfo, error)
}

// PlexSyncStatusInfo mirrors plex.SyncStatus for MCP consumption.
type PlexSyncStatusInfo struct {
	IsRunning   bool                 `json:"is_running"`
	StartedAt   int64                `json:"started_at,omitempty"`
	CompletedAt int64                `json:"completed_at,omitempty"`
	Progress    int                  `json:"progress"`
	Total       int                  `json:"total"`
	Error       string               `json:"error,omitempty"`
	Results     []PlexSyncResultInfo `json:"results"`
}

// PlexSyncResultInfo represents a single playlist sync outcome.
type PlexSyncResultInfo struct {
	PlaylistName string `json:"playlist_name"`
	Action       string `json:"action"`
	Error        string `json:"error,omitempty"`
	TrackCount   int    `json:"track_count,omitempty"`
}

// RuntimeDataProvider is an optional interface for live runtime data
// available only when embedded in the API server process.
type RuntimeDataProvider interface {
	GetLiveDownloadStatus() *DownloadStatusInfo
	GetLiveRecentLogs() []LogEntryInfo
	GetLiveRateLimitInfo() *RateLimitInfo
	GetLivePlexSyncStatus() *PlexSyncStatusInfo
}

// PlanFileSummary describes a saved plan file on disk.
type PlanFileSummary struct {
	Hash       string    `json:"hash"`
	Path       string    `json:"path"`
	ModifiedAt time.Time `json:"modified_at"`
	SizeBytes  int64     `json:"size_bytes"`
	TrackCount int       `json:"track_count"`
}

// DownloadStatusInfo represents the current download operation state.
type DownloadStatusInfo struct {
	IsRunning     bool   `json:"is_running"`
	OperationType string `json:"operation_type"`
	Progress      int    `json:"progress"`
	Total         int    `json:"total"`
	StartedAt     int64  `json:"started_at"`
	ErrorMsg      string `json:"error,omitempty"`
}

// StatsInfo mirrors the stats.json on disk.
type StatsInfo struct {
	Cumulative *CumulativeStatsInfo `json:"cumulative"`
	CurrentRun *RunStatsInfo        `json:"currentRun"`
}

// CumulativeStatsInfo holds lifetime statistics.
type CumulativeStatsInfo struct {
	TotalDownloaded      int64   `json:"totalDownloaded"`
	TotalFailed          int64   `json:"totalFailed"`
	TotalSkipped         int64   `json:"totalSkipped"`
	TotalPlansGenerated  int64   `json:"totalPlansGenerated"`
	TotalRuns            int64   `json:"totalRuns"`
	TotalRateLimits      int64   `json:"totalRateLimits"`
	TotalRetries         int64   `json:"totalRetries"`
	TotalBytesWritten    int64   `json:"totalBytesWritten"`
	TotalTimeSpentSec    float64 `json:"totalTimeSpentSec"`
	PlanTimeSpentSec     float64 `json:"planTimeSpentSec"`
	DownloadTimeSpentSec float64 `json:"downloadTimeSpentSec"`
	FirstRunAt           int64   `json:"firstRunAt"`
	LastRunAt            int64   `json:"lastRunAt"`
	SuccessRate          float64 `json:"successRate"`
}

// RunStatsInfo holds stats for a single run.
type RunStatsInfo struct {
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

// LogFilter controls which log entries to return.
type LogFilter struct {
	Level   string `json:"level,omitempty"`
	Keyword string `json:"keyword,omitempty"`
	RunDir  string `json:"run_dir,omitempty"`
	Limit   int    `json:"limit,omitempty"`
}

// LogEntryInfo represents a single log entry.
type LogEntryInfo struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Service   string    `json:"service"`
	Operation string    `json:"operation,omitempty"`
	Error     string    `json:"error,omitempty"`
	Source    string    `json:"source,omitempty"`
}

// RunLogDir describes a log directory for a single run.
type RunLogDir struct {
	Name      string    `json:"name"`
	Path      string    `json:"path"`
	CreatedAt time.Time `json:"created_at"`
}

// HealthInfo represents system health.
type HealthInfo struct {
	Status         string `json:"status"`
	MusicdlVersion string `json:"musicdl_version"`
	SpotigoVersion string `json:"spotigo_version"`
	GoVersion      string `json:"go_version"`
	APIRunning     bool   `json:"api_running"`
}

// RateLimitInfo holds Spotify rate limit data.
type RateLimitInfo struct {
	Active              bool  `json:"active"`
	RetryAfterSeconds   int64 `json:"retryAfterSeconds"`
	RetryAfterTimestamp int64 `json:"retryAfterTimestamp"`
	RemainingSeconds    int64 `json:"remainingSeconds"`
}

// RecoveryInfo holds circuit breaker and resume state.
type RecoveryInfo struct {
	CircuitBreaker CircuitBreakerInfo `json:"circuitBreaker"`
	Resume         ResumeInfo         `json:"resume"`
}

// CircuitBreakerInfo mirrors the circuit breaker status.
type CircuitBreakerInfo struct {
	State            string `json:"state"`
	FailureCount     int    `json:"failureCount"`
	SuccessCount     int    `json:"successCount"`
	FailureThreshold int    `json:"failureThreshold"`
	SuccessThreshold int    `json:"successThreshold"`
	ResetTimeoutSec  int    `json:"resetTimeoutSec"`
	LastFailureAt    int64  `json:"lastFailureAt"`
	LastStateChange  int64  `json:"lastStateChange"`
	CanRetry         bool   `json:"canRetry"`
}

// ResumeInfo mirrors the resume state status.
type ResumeInfo struct {
	HasResumeData  bool              `json:"hasResumeData"`
	CompletedCount int               `json:"completedCount"`
	FailedCount    int               `json:"failedCount"`
	TotalItems     int               `json:"totalItems"`
	RemainingCount int               `json:"remainingCount"`
	FailedItems    []FailedItemEntry `json:"failedItems,omitempty"`
}

// FailedItemEntry describes a single failed download item.
type FailedItemEntry struct {
	ID          string `json:"id"`
	URL         string `json:"url"`
	Name        string `json:"name"`
	Error       string `json:"error"`
	Attempts    int    `json:"attempts"`
	LastAttempt int64  `json:"lastAttempt"`
	Retryable   bool   `json:"retryable"`
}

// CacheInfo describes the state of the cache directory.
type CacheInfo struct {
	CacheDir      string          `json:"cache_dir"`
	TotalSize     int64           `json:"total_size_bytes"`
	PlanFiles     []PlanFileSummary `json:"plan_files"`
	StatsFile     string          `json:"stats_file,omitempty"`
	ResumeFile    string          `json:"resume_file,omitempty"`
	HistoryDir    string          `json:"history_dir,omitempty"`
	HistoryCount  int             `json:"history_count"`
}

// LibraryEntry represents a file or directory in the music library.
type LibraryEntry struct {
	Name      string `json:"name"`
	Path      string `json:"path"`
	IsDir     bool   `json:"is_dir"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Format    string `json:"format,omitempty"`
}

// LibraryStatsInfo holds summary stats about the music library.
type LibraryStatsInfo struct {
	TotalFiles   int            `json:"total_files"`
	TotalSize    int64          `json:"total_size_bytes"`
	ByFormat     map[string]int `json:"by_format"`
	ArtistCount  int            `json:"artist_count"`
	AlbumCount   int            `json:"album_count"`
}

// RunSummaryInfo is a compact view of a historical run.
type RunSummaryInfo struct {
	RunID       string                 `json:"run_id"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	State       string                 `json:"state"`
	Statistics  map[string]interface{} `json:"statistics,omitempty"`
	Error       string                 `json:"error,omitempty"`
}
