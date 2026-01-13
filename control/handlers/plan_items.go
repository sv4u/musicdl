package handlers

import (
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/sv4u/musicdl/download/plan"
)

// PlanItemsResponse represents the response for plan items API.
type PlanItemsResponse struct {
	Items      []PlanItemView `json:"items"`
	Total      int            `json:"total"`
	Filtered   int            `json:"filtered"`
	Statistics map[string]int  `json:"statistics"`
}

// PlanItemView represents a view of a plan item with enriched information.
type PlanItemView struct {
	ItemID       string                 `json:"item_id"`
	ItemType     string                 `json:"item_type"`
	Name         string                 `json:"name"`
	Status       string                 `json:"status"`
	Progress     float64                `json:"progress"`
	Error        string                 `json:"error,omitempty"`
	FilePath     string                 `json:"file_path,omitempty"`
	CreatedAt   string                 `json:"created_at"`
	StartedAt    string                 `json:"started_at,omitempty"`
	CompletedAt  string                 `json:"completed_at,omitempty"`
	Artist       string                 `json:"artist,omitempty"`
	Album        string                 `json:"album,omitempty"`
	Playlists    []string               `json:"playlists,omitempty"`
	ParentID     string                 `json:"parent_id,omitempty"`
	ChildIDs     []string               `json:"child_ids,omitempty"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
}

// PlanItems handles GET /api/plan/items - Get plan items with filtering, sorting, and search.
func (h *Handlers) PlanItems(w http.ResponseWriter, r *http.Request) {
	// Get service
	service, err := h.getService()
	if err != nil || service == nil {
		response := PlanItemsResponse{
			Items:      []PlanItemView{},
			Total:      0,
			Filtered:   0,
			Statistics: map[string]int{},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get plan
	currentPlan := service.GetPlan()
	if currentPlan == nil {
		response := PlanItemsResponse{
			Items:      []PlanItemView{},
			Total:      0,
			Filtered:   0,
			Statistics: map[string]int{},
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Parse query parameters
	query := r.URL.Query()
	filterStatus := query.Get("status")        // Filter by status
	sortBy := query.Get("sort")                // Sort by: name, status, timestamp
	sortOrder := query.Get("order")            // Order: asc, desc
	search := query.Get("search")              // Search by track/album/artist name
	itemType := query.Get("type")              // Filter by item type
	showHierarchy := query.Get("hierarchy") == "true" // Show hierarchy

	// Convert plan items to views
	itemViews := h.convertPlanItemsToViews(currentPlan, showHierarchy)

	// Apply filters
	filtered := h.filterItems(itemViews, filterStatus, itemType, search)

	// Apply sorting
	h.sortItems(filtered, sortBy, sortOrder)

	// Get statistics
	stats := currentPlan.GetExecutionStatistics()

	response := PlanItemsResponse{
		Items:      filtered,
		Total:      len(itemViews),
		Filtered:   len(filtered),
		Statistics: stats,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// convertPlanItemsToViews converts plan items to view models with enriched information.
func (h *Handlers) convertPlanItemsToViews(p *plan.DownloadPlan, showHierarchy bool) []PlanItemView {
	views := make([]PlanItemView, 0)

	// Build a map for quick lookup
	itemMap := make(map[string]*plan.PlanItem)
	for _, item := range p.Items {
		itemMap[item.ItemID] = item
	}

	// Build parent-child relationships for finding playlists
	parentMap := make(map[string][]string) // parent -> children
	for _, item := range p.Items {
		if item.ParentID != "" {
			parentMap[item.ParentID] = append(parentMap[item.ParentID], item.ItemID)
		}
	}

	// Helper to find all playlists for a track
	findPlaylists := func(itemID string) []string {
		playlists := make([]string, 0)
		visited := make(map[string]bool)

		var traverse func(id string)
		traverse = func(id string) {
			if visited[id] {
				return
			}
			visited[id] = true

			item, ok := itemMap[id]
			if !ok {
				return
			}

			if item.ItemType == plan.PlanItemTypePlaylist {
				playlists = append(playlists, item.Name)
			}

			if item.ParentID != "" {
				traverse(item.ParentID)
			}
		}

		traverse(itemID)
		return playlists
	}

	// Convert items to views
	for _, item := range p.Items {
		// Thread-safe access using getter methods
		status := string(item.GetStatus())
		progress := item.GetProgress()
		errorMsg := item.GetError()
		filePath := item.GetFilePath()
		createdAt, startedAt, completedAt := item.GetTimestamps()
		metadata := item.GetMetadata()
		
		// These fields are immutable after creation, safe to access directly
		name := item.Name
		itemType := string(item.ItemType)
		parentID := item.ParentID
		childIDs := item.ChildIDs

		view := PlanItemView{
			ItemID:      item.ItemID,
			ItemType:    itemType,
			Name:        name,
			Status:      status,
			Progress:    progress * 100.0, // Convert to percentage
			Error:       errorMsg,
			FilePath:    filePath,
			CreatedAt:   createdAt.Format("2006-01-02T15:04:05Z07:00"),
			ParentID:    parentID,
			ChildIDs:    childIDs,
			Metadata:    metadata,
		}

		if startedAt != nil {
			view.StartedAt = startedAt.Format("2006-01-02T15:04:05Z07:00")
		}
		if completedAt != nil {
			view.CompletedAt = completedAt.Format("2006-01-02T15:04:05Z07:00")
		}

		// Extract artist and album from metadata
		if artist, ok := metadata["artist"].(string); ok {
			view.Artist = artist
		} else if enhancement, ok := metadata["spotify_enhancement"].(map[string]interface{}); ok {
			if artist, ok := enhancement["artist"].(string); ok {
				view.Artist = artist
			}
		}

		if album, ok := metadata["album"].(string); ok {
			view.Album = album
		} else if enhancement, ok := metadata["spotify_enhancement"].(map[string]interface{}); ok {
			if album, ok := enhancement["album"].(string); ok {
				view.Album = album
			}
		}

		// Find playlists for tracks
		if item.ItemType == plan.PlanItemTypeTrack {
			view.Playlists = findPlaylists(item.ItemID)
		}

		// Filter based on hierarchy display preference
		if showHierarchy || item.ItemType == plan.PlanItemTypeTrack {
			views = append(views, view)
		}
	}

	return views
}

// filterItems applies filters to the item views.
func (h *Handlers) filterItems(items []PlanItemView, statusFilter, typeFilter, search string) []PlanItemView {
	filtered := make([]PlanItemView, 0)

	for _, item := range items {
		// Status filter
		if statusFilter != "" && item.Status != statusFilter {
			continue
		}

		// Type filter
		if typeFilter != "" && item.ItemType != typeFilter {
			continue
		}

		// Search filter
		if search != "" {
			searchLower := strings.ToLower(search)
			matched := false

			if strings.Contains(strings.ToLower(item.Name), searchLower) {
				matched = true
			}
			if strings.Contains(strings.ToLower(item.Artist), searchLower) {
				matched = true
			}
			if strings.Contains(strings.ToLower(item.Album), searchLower) {
				matched = true
			}
			for _, playlist := range item.Playlists {
				if strings.Contains(strings.ToLower(playlist), searchLower) {
					matched = true
					break
				}
			}

			if !matched {
				continue
			}
		}

		filtered = append(filtered, item)
	}

	return filtered
}

// sortItems sorts items based on the specified criteria.
func (h *Handlers) sortItems(items []PlanItemView, sortBy, order string) {
	if sortBy == "" {
		sortBy = "timestamp"
	}
	if order == "" {
		order = "desc"
	}

	sort.Slice(items, func(i, j int) bool {
		var less bool

		switch sortBy {
		case "name":
			less = strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		case "status":
			less = items[i].Status < items[j].Status
		case "timestamp":
			// Sort by started_at or created_at
			var timeI, timeJ string
			if items[i].StartedAt != "" {
				timeI = items[i].StartedAt
			} else {
				timeI = items[i].CreatedAt
			}
			if items[j].StartedAt != "" {
				timeJ = items[j].StartedAt
			} else {
				timeJ = items[j].CreatedAt
			}
			less = timeI < timeJ
		default:
			less = false
		}

		if order == "desc" {
			return !less
		}
		return less
	})
}
