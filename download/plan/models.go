package plan

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// PlanItemType represents the type of a plan item.
type PlanItemType string

const (
	PlanItemTypeTrack    PlanItemType = "track"
	PlanItemTypeAlbum    PlanItemType = "album"
	PlanItemTypeArtist   PlanItemType = "artist"
	PlanItemTypePlaylist PlanItemType = "playlist"
	PlanItemTypeM3U      PlanItemType = "m3u"
)

// PlanItemStatus represents the status of a plan item.
type PlanItemStatus string

const (
	PlanItemStatusPending     PlanItemStatus = "pending"
	PlanItemStatusInProgress  PlanItemStatus = "in_progress"
	PlanItemStatusCompleted   PlanItemStatus = "completed"
	PlanItemStatusFailed      PlanItemStatus = "failed"
	PlanItemStatusSkipped     PlanItemStatus = "skipped"
)

// PlanItem represents a single item in the download plan.
type PlanItem struct {
	// Identification
	ItemID     string       `json:"item_id"`
	ItemType   PlanItemType `json:"item_type"`
	SpotifyID  string       `json:"spotify_id,omitempty"`
	SpotifyURL string       `json:"spotify_url,omitempty"`
	YouTubeURL string       `json:"youtube_url,omitempty"`

	// Hierarchy
	ParentID string   `json:"parent_id,omitempty"`
	ChildIDs []string `json:"child_ids,omitempty"`

	// Metadata
	Name     string                 `json:"name"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Status tracking
	Status   PlanItemStatus `json:"status"`
	Error    string         `json:"error,omitempty"`
	FilePath string         `json:"file_path,omitempty"`

	// Timestamps
	CreatedAt   time.Time  `json:"created_at"`
	StartedAt   *time.Time `json:"started_at,omitempty"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`

	// Progress tracking
	Progress float64 `json:"progress"` // 0.0 to 1.0

	// Thread safety (not serialized)
	mu sync.RWMutex `json:"-"`
}

// MarkStarted marks the item as started.
func (p *PlanItem) MarkStarted() {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	p.Status = PlanItemStatusInProgress
	if p.StartedAt == nil {
		p.StartedAt = &now
	}
}

// MarkCompleted marks the item as completed.
func (p *PlanItem) MarkCompleted(filePath string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	now := time.Now()
	p.Status = PlanItemStatusCompleted
	p.CompletedAt = &now
	p.Progress = 1.0
	if filePath != "" {
		p.FilePath = filePath
	}
}

// MarkFailed marks the item as failed.
func (p *PlanItem) MarkFailed(errorMsg string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = PlanItemStatusFailed
	p.Error = errorMsg
	p.Progress = 0.0
}

// MarkSkipped marks the item as skipped.
func (p *PlanItem) MarkSkipped(filePath string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Status = PlanItemStatusSkipped
	p.Progress = 1.0
	if filePath != "" {
		p.FilePath = filePath
	}
}

// GetStatus returns the current status (thread-safe).
func (p *PlanItem) GetStatus() PlanItemStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.Status
}

// DownloadPlan represents a complete download plan.
type DownloadPlan struct {
	Items    []*PlanItem        `json:"items"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// NewDownloadPlan creates a new download plan.
func NewDownloadPlan(metadata map[string]interface{}) *DownloadPlan {
	return &DownloadPlan{
		Items:    make([]*PlanItem, 0),
		Metadata: metadata,
	}
}

// AddItem adds an item to the plan.
func (p *DownloadPlan) AddItem(item *PlanItem) {
	p.Items = append(p.Items, item)
}

// GetItem retrieves an item by ID.
func (p *DownloadPlan) GetItem(itemID string) *PlanItem {
	for _, item := range p.Items {
		if item.ItemID == itemID {
			return item
		}
	}
	return nil
}

// GetItemsByType returns all items of a specific type.
func (p *DownloadPlan) GetItemsByType(itemType PlanItemType) []*PlanItem {
	result := make([]*PlanItem, 0)
	for _, item := range p.Items {
		if item.ItemType == itemType {
			result = append(result, item)
		}
	}
	return result
}

// GetStatistics returns statistics about the plan.
func (p *DownloadPlan) GetStatistics() map[string]interface{} {
	stats := map[string]interface{}{
		"total_items": len(p.Items),
		"by_type": make(map[string]int),
	}

	byType := make(map[string]int)
	for _, item := range p.Items {
		byType[string(item.ItemType)]++
	}
	stats["by_type"] = byType

	return stats
}

// GetExecutionStatistics returns execution statistics for track items.
// This method counts only track items (excluding containers and M3U files)
// and excludes skipped items. It returns status-based statistics that are
// useful for tracking download progress.
// The returned map contains: "completed", "failed", "pending", "in_progress", "total".
func (p *DownloadPlan) GetExecutionStatistics() map[string]int {
	// Filter to only track items, excluding skipped items
	trackItems := make([]*PlanItem, 0)
	for _, item := range p.Items {
		if item.ItemType == PlanItemTypeTrack {
			// Use thread-safe GetStatus() to check status
			status := item.GetStatus()
			if status != PlanItemStatusSkipped {
				trackItems = append(trackItems, item)
			}
		}
	}

	// Count by status
	completed := 0
	failed := 0
	pending := 0
	inProgress := 0

	for _, item := range trackItems {
		// Use thread-safe GetStatus() method
		status := item.GetStatus()
		switch status {
		case PlanItemStatusCompleted:
			completed++
		case PlanItemStatusFailed:
			failed++
		case PlanItemStatusPending:
			pending++
		case PlanItemStatusInProgress:
			inProgress++
		}
	}

	return map[string]int{
		"completed":   completed,
		"failed":      failed,
		"pending":     pending,
		"in_progress": inProgress,
		"total":       len(trackItems),
	}
}

// Save saves the plan to a JSON file.
func (p *DownloadPlan) Save(filePath string) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filePath, data, 0644)
}

// Load loads a plan from a JSON file.
func LoadPlan(filePath string) (*DownloadPlan, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var plan DownloadPlan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, err
	}

	return &plan, nil
}
