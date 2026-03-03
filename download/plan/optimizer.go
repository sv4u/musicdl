package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sv4u/musicdl/download/config"
)

// Optimizer optimizes download plans by removing duplicates, pre-computing output paths, and checking file existence.
type Optimizer struct {
	checkFileExistence bool
	overwriteMode      config.OverwriteMode
	outputTemplate     string
	outputFormat       string
}

// NewOptimizer creates a new plan optimizer.
// If outputTemplate is non-empty, the optimizer will resolve output paths from plan item metadata before checking files.
// overwriteMode is used to decide whether to mark existing files as skipped (only when OverwriteSkip).
func NewOptimizer(checkFileExistence bool, overwriteMode config.OverwriteMode, outputTemplate, outputFormat string) *Optimizer {
	if outputTemplate == "" {
		outputTemplate = "{artist}/{album}/{track-number} - {title}.{output-ext}"
	}
	if outputFormat == "" {
		outputFormat = "mp3"
	}
	return &Optimizer{
		checkFileExistence: checkFileExistence,
		overwriteMode:      overwriteMode,
		outputTemplate:     outputTemplate,
		outputFormat:       outputFormat,
	}
}

// Optimize optimizes a download plan.
func (o *Optimizer) Optimize(plan *DownloadPlan) {
	o.removeDuplicates(plan)
	o.resolveOutputPaths(plan)
	if o.checkFileExistence {
		o.checkFiles(plan)
	}
}

// resolveOutputPaths sets FilePath on track items that have sufficient metadata.
func (o *Optimizer) resolveOutputPaths(plan *DownloadPlan) {
	for _, item := range plan.Items {
		if item.ItemType != PlanItemTypeTrack || item.Status != PlanItemStatusPending {
			continue
		}
		if item.Metadata == nil {
			continue
		}
		path := o.pathFromMetadata(item)
		if path != "" {
			item.FilePath = path
		}
	}
}

func (o *Optimizer) pathFromMetadata(item *PlanItem) string {
	artist := getMetaString(item.Metadata, "artist")
	album := getMetaString(item.Metadata, "album")
	title := getMetaString(item.Metadata, "title")
	if title == "" {
		title = item.Name
	}
	trackNum := getMetaInt(item.Metadata, "track_number")
	discNum := getMetaInt(item.Metadata, "disc_number")
	output := o.outputTemplate
	output = strings.ReplaceAll(output, "{artist}", sanitizePathPart(artist))
	output = strings.ReplaceAll(output, "{album}", sanitizePathPart(album))
	output = strings.ReplaceAll(output, "{title}", sanitizePathPart(title))
	output = strings.ReplaceAll(output, "{track-number}", fmt.Sprintf("%02d", trackNum))
	output = strings.ReplaceAll(output, "{disc-number}", fmt.Sprintf("%02d", discNum))
	output = strings.ReplaceAll(output, "{output-ext}", o.outputFormat)
	return filepath.Clean(output)
}

func getMetaString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok {
		return ""
	}
	s, _ := v.(string)
	return s
}

func getMetaInt(m map[string]interface{}, key string) int {
	if m == nil {
		return 0
	}
	v, ok := m[key]
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	default:
		return 0
	}
}

func sanitizePathPart(name string) string {
	if name == "" {
		return "_"
	}
	const maxLen = 255
	result := []rune(name)
	if len(result) > maxLen {
		result = result[:maxLen]
	}
	invalid := []rune{'/', '\\', ':', '*', '?', '"', '<', '>', '|'}
	for i, r := range result {
		for _, inv := range invalid {
			if r == inv {
				result[i] = '_'
				break
			}
		}
	}
	s := strings.ReplaceAll(string(result), "..", "_")
	s = strings.Trim(s, ". ")
	if s == "" {
		return "_"
	}
	return s
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

// checkFiles checks if files already exist and marks items as skipped only when overwrite mode is skip.
func (o *Optimizer) checkFiles(plan *DownloadPlan) {
	if o.overwriteMode != config.OverwriteSkip {
		return
	}
	for _, item := range plan.Items {
		if item.ItemType != PlanItemTypeTrack {
			continue
		}
		if item.Status != PlanItemStatusPending {
			continue
		}
		if item.FilePath != "" {
			if _, err := os.Stat(item.FilePath); err == nil {
				item.MarkSkipped(item.FilePath)
			}
		}
	}
}
