package plan

import (
	"os"
)

// Optimizer optimizes download plans by removing duplicates and checking file existence.
type Optimizer struct {
	checkFileExistence bool
}

// NewOptimizer creates a new plan optimizer.
func NewOptimizer(checkFileExistence bool) *Optimizer {
	return &Optimizer{
		checkFileExistence: checkFileExistence,
	}
}

// Optimize optimizes a download plan.
func (o *Optimizer) Optimize(plan *DownloadPlan) {
	// Remove duplicate tracks (keep first occurrence)
	o.removeDuplicates(plan)

	// Check file existence if enabled
	if o.checkFileExistence {
		o.checkFiles(plan)
	}
}

// removeDuplicates removes duplicate track items from the plan.
func (o *Optimizer) removeDuplicates(plan *DownloadPlan) {
	seenTrackIDs := make(map[string]*PlanItem)
	itemsToRemove := make(map[int]bool)

	for i, item := range plan.Items {
		if item.ItemType != PlanItemTypeTrack {
			continue
		}

		if item.SpotifyID == "" {
			continue
		}

		// Check if we've seen this track ID before
		if existing, exists := seenTrackIDs[item.SpotifyID]; exists {
			// Mark for removal
			itemsToRemove[i] = true

			// Update all parents that reference this duplicate item
			// Find all parents that have this item in their ChildIDs
			for _, parentItem := range plan.Items {
				if parentItem.ItemType == PlanItemTypeTrack {
					continue // Skip tracks, only check containers
				}
				
				// Check if this parent references the duplicate item
				hasDuplicate := false
				hasExisting := false
				for _, childID := range parentItem.ChildIDs {
					if childID == item.ItemID {
						hasDuplicate = true
					}
					if childID == existing.ItemID {
						hasExisting = true
					}
				}

				if hasDuplicate {
					// Update parent's child references
					newChildIDs := make([]string, 0, len(parentItem.ChildIDs))
					for _, childID := range parentItem.ChildIDs {
						if childID == item.ItemID {
							// Skip the duplicate
							continue
						}
						newChildIDs = append(newChildIDs, childID)
					}
					// Add existing item ID if not already present
					if !hasExisting {
						newChildIDs = append(newChildIDs, existing.ItemID)
					}
					parentItem.ChildIDs = newChildIDs
				}
			}
		} else {
			seenTrackIDs[item.SpotifyID] = item
		}
	}

	// Remove duplicate items (in reverse order to maintain indices)
	newItems := make([]*PlanItem, 0, len(plan.Items))
	for i, item := range plan.Items {
		if !itemsToRemove[i] {
			newItems = append(newItems, item)
		}
	}
	plan.Items = newItems
}

// checkFiles checks if files already exist and marks items as skipped if appropriate.
func (o *Optimizer) checkFiles(plan *DownloadPlan) {
	for _, item := range plan.Items {
		if item.ItemType != PlanItemTypeTrack {
			continue
		}

		// Only check pending items
		if item.Status != PlanItemStatusPending {
			continue
		}

		// If item already has a file path, check if it exists
		if item.FilePath != "" {
			if _, err := os.Stat(item.FilePath); err == nil {
				// File exists - mark as skipped
				item.MarkSkipped(item.FilePath)
			}
		}
	}
}
