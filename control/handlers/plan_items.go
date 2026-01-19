package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sv4u/musicdl/download/proto"
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
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	if !svcManager.IsRunning() {
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

	// Get gRPC client
	client, err := svcManager.GetClient(ctx)
	if err != nil {
		h.logError("PlanItems", err)
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
	filterStatus := query.Get("status")
	sortBy := query.Get("sort")
	sortOrder := query.Get("order")
	search := query.Get("search")
	itemType := query.Get("type")
	showHierarchy := query.Get("hierarchy") == "true"

	// Build filters for gRPC request
	var filters *proto.PlanItemFilters
	if filterStatus != "" || itemType != "" || search != "" {
		filters = &proto.PlanItemFilters{
			Search: search,
		}

		// Convert status filter
		if filterStatus != "" {
			switch filterStatus {
			case "pending":
				filters.Status = []proto.PlanItemStatus{proto.PlanItemStatus_PLAN_ITEM_STATUS_PENDING}
			case "in_progress":
				filters.Status = []proto.PlanItemStatus{proto.PlanItemStatus_PLAN_ITEM_STATUS_IN_PROGRESS}
			case "completed":
				filters.Status = []proto.PlanItemStatus{proto.PlanItemStatus_PLAN_ITEM_STATUS_COMPLETED}
			case "failed":
				filters.Status = []proto.PlanItemStatus{proto.PlanItemStatus_PLAN_ITEM_STATUS_FAILED}
			case "skipped":
				filters.Status = []proto.PlanItemStatus{proto.PlanItemStatus_PLAN_ITEM_STATUS_SKIPPED}
			}
		}

		// Convert type filter
		if itemType != "" {
			switch itemType {
			case "track":
				filters.Type = []proto.PlanItemType{proto.PlanItemType_PLAN_ITEM_TYPE_TRACK}
			case "album":
				filters.Type = []proto.PlanItemType{proto.PlanItemType_PLAN_ITEM_TYPE_ALBUM}
			case "artist":
				filters.Type = []proto.PlanItemType{proto.PlanItemType_PLAN_ITEM_TYPE_ARTIST}
			case "playlist":
				filters.Type = []proto.PlanItemType{proto.PlanItemType_PLAN_ITEM_TYPE_PLAYLIST}
			case "m3u":
				filters.Type = []proto.PlanItemType{proto.PlanItemType_PLAN_ITEM_TYPE_M3U}
			}
		}
	}

	// Get plan items via gRPC
	planItemsResp, err := client.GetPlanItems(ctx, filters)
	if err != nil {
		h.logError("PlanItems", err)
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

	// Convert proto plan items to views
	itemViews := h.convertProtoPlanItemsToViews(planItemsResp.Items, showHierarchy)

	// Apply additional client-side filtering (for complex filters not supported by server)
	filtered := h.filterItems(itemViews, filterStatus, itemType, search)

	// Apply sorting
	h.sortItems(filtered, sortBy, sortOrder)

	// Calculate statistics from items
	stats := h.calculateStatistics(itemViews)

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

// convertProtoPlanItemsToViews converts proto plan items to view models.
func (h *Handlers) convertProtoPlanItemsToViews(items []*proto.PlanItem, showHierarchy bool) []PlanItemView {
	views := make([]PlanItemView, 0)

	// Build maps for relationships
	itemMap := make(map[string]*proto.PlanItem)
	parentMap := make(map[string][]string)

	for _, item := range items {
		itemMap[item.ItemId] = item
		if item.ParentId != "" {
			parentMap[item.ParentId] = append(parentMap[item.ParentId], item.ItemId)
		}
	}

	// Helper to find playlists for a track
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

			if item.ItemType == proto.PlanItemType_PLAN_ITEM_TYPE_PLAYLIST {
				playlists = append(playlists, item.Name)
			}

			if item.ParentId != "" {
				traverse(item.ParentId)
			}
		}

		traverse(itemID)
		return playlists
	}

	// Convert items to views
	for _, item := range items {
		view := PlanItemView{
			ItemID:    item.ItemId,
			ItemType:  item.ItemType.String(),
			Name:      item.Name,
			Status:    item.Status.String(),
			Progress:  item.Progress * 100.0,
			Error:     item.Error,
			FilePath:  item.FilePath,
			ParentID:  item.ParentId,
			ChildIDs:  item.ChildIds,
			CreatedAt: time.Unix(item.CreatedAt, 0).Format(time.RFC3339),
		}

		if item.StartedAt != nil {
			view.StartedAt = time.Unix(*item.StartedAt, 0).Format(time.RFC3339)
		}
		if item.CompletedAt != nil {
			view.CompletedAt = time.Unix(*item.CompletedAt, 0).Format(time.RFC3339)
		}

		// Convert metadata (proto has string values, convert back to interface{})
		if len(item.Metadata) > 0 {
			metadata := make(map[string]interface{})
			for k, v := range item.Metadata {
				// Try to parse as number, boolean, or keep as string
				if num, err := strconv.ParseFloat(v, 64); err == nil {
					metadata[k] = num
				} else if b, err := strconv.ParseBool(v); err == nil {
					metadata[k] = b
				} else {
					metadata[k] = v
				}
			}
			view.Metadata = metadata

			// Extract artist and album
			if artist, ok := metadata["artist"].(string); ok {
				view.Artist = artist
			}
			if album, ok := metadata["album"].(string); ok {
				view.Album = album
			}
		}

		// Find playlists for tracks
		if item.ItemType == proto.PlanItemType_PLAN_ITEM_TYPE_TRACK {
			view.Playlists = findPlaylists(item.ItemId)
		}

		// Filter based on hierarchy display preference
		if showHierarchy || item.ItemType == proto.PlanItemType_PLAN_ITEM_TYPE_TRACK {
			views = append(views, view)
		}
	}

	return views
}

// calculateStatistics calculates statistics from plan item views.
func (h *Handlers) calculateStatistics(items []PlanItemView) map[string]int {
	stats := map[string]int{
		"total":    0,
		"completed": 0,
		"failed":   0,
		"pending":  0,
		"in_progress": 0,
	}

	for _, item := range items {
		if item.ItemType == "PLAN_ITEM_TYPE_TRACK" {
			stats["total"]++
			switch item.Status {
			case "PLAN_ITEM_STATUS_COMPLETED":
				stats["completed"]++
			case "PLAN_ITEM_STATUS_FAILED":
				stats["failed"]++
			case "PLAN_ITEM_STATUS_PENDING":
				stats["pending"]++
			case "PLAN_ITEM_STATUS_IN_PROGRESS":
				stats["in_progress"]++
			}
		}
	}

	return stats
}

// convertPlanItemsToViews converts plan items to view models with enriched information (legacy - kept for compatibility).
func (h *Handlers) convertPlanItemsToViews(items []*proto.PlanItem, showHierarchy bool) []PlanItemView {
	return h.convertProtoPlanItemsToViews(items, showHierarchy)
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
