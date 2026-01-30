package history

import (
	"encoding/json"
	"time"
)

// RunSnapshot represents a snapshot of progress at a specific point in time.
type RunSnapshot struct {
	Timestamp  time.Time              `json:"timestamp"`
	Progress   float64                `json:"progress"`   // 0.0 to 100.0
	Statistics map[string]interface{} `json:"statistics"` // completed, failed, pending, in_progress, total
	State      string                 `json:"state"`      // idle, running, stopping, error
	Phase      string                 `json:"phase"`      // idle, generating, optimizing, executing, completed, error
}

// RunHistory represents a complete execution run with multiple snapshots.
type RunHistory struct {
	RunID       string                 `json:"run_id"` // Unique identifier for this run
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
	State       string                 `json:"state"`      // Final state
	Phase       string                 `json:"phase"`      // Final phase
	Statistics  map[string]interface{} `json:"statistics"` // Final statistics
	Snapshots   []RunSnapshot          `json:"snapshots"`
	Error       string                 `json:"error,omitempty"`
}

// ActivityEntry represents a single activity/event in the system.
type ActivityEntry struct {
	ID        string                 `json:"id"`
	Timestamp time.Time              `json:"timestamp"`
	Type      string                 `json:"type"` // download_started, download_completed, download_failed, track_completed, etc.
	Message   string                 `json:"message"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// ActivityHistory represents a collection of activity entries.
type ActivityHistory struct {
	Entries []ActivityEntry `json:"entries"`
}

// ToJSON converts RunHistory to JSON bytes.
func (r *RunHistory) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// FromJSON creates RunHistory from JSON bytes.
func (r *RunHistory) FromJSON(data []byte) error {
	return json.Unmarshal(data, r)
}

// ToJSON converts ActivityHistory to JSON bytes.
func (a *ActivityHistory) ToJSON() ([]byte, error) {
	return json.MarshalIndent(a, "", "  ")
}

// FromJSON creates ActivityHistory from JSON bytes.
func (a *ActivityHistory) FromJSON(data []byte) error {
	return json.Unmarshal(data, a)
}
