package plan

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Downloader interface for downloading tracks.
type Downloader interface {
	// DownloadTrack downloads a track from a PlanItem.
	// The PlanItem contains either a SpotifyURL or YouTubeURL, along with metadata.
	// Returns (success, filePath, error)
	DownloadTrack(ctx context.Context, item *PlanItem) (bool, string, error)
}

// Executor executes download plans with parallel processing.
type Executor struct {
	downloader        Downloader
	maxWorkers        int
	mu                sync.Mutex
	progressCallback  func(*PlanItem)
	shutdownRequested bool
	shutdownMu        sync.RWMutex
	currentPlan       *DownloadPlan
	executionWg       *sync.WaitGroup // WaitGroup for active execution
	executionWgMu     sync.RWMutex    // Protects executionWg
}

// NewExecutor creates a new plan executor.
func NewExecutor(downloader Downloader, maxWorkers int) *Executor {
	if maxWorkers <= 0 {
		maxWorkers = 4 // Default
	}
	return &Executor{
		downloader: downloader,
		maxWorkers: maxWorkers,
	}
}

// Execute executes a download plan.
func (e *Executor) Execute(ctx context.Context, plan *DownloadPlan, progressCallback func(*PlanItem)) (map[string]int, error) {
	e.progressCallback = progressCallback
	e.currentPlan = plan
	e.setShutdownRequested(false)

	// Create WaitGroup for tracking active execution
	wg := &sync.WaitGroup{}
	e.executionWgMu.Lock()
	e.executionWg = wg
	e.executionWgMu.Unlock()

	startTime := time.Now()

	// Get pending track items
	trackItems := make([]*PlanItem, 0)
	for _, item := range plan.Items {
		if item.ItemType == PlanItemTypeTrack && item.Status == PlanItemStatusPending {
			trackItems = append(trackItems, item)
		}
	}

	// Execute tracks in parallel using goroutines
	// Use a semaphore to limit concurrent downloads
	sem := make(chan struct{}, e.maxWorkers)

	for _, item := range trackItems {
		// Check for shutdown
		if e.isShutdownRequested() {
			break
		}

		// Check context cancellation
		if err := ctx.Err(); err != nil {
			return nil, err
		}

		wg.Add(1)
		go func(trackItem *PlanItem) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			e.executeTrack(ctx, trackItem, plan)
		}(item)
	}

	// Wait for all downloads to complete
	wg.Wait()

	// Clear execution WaitGroup after completion
	e.executionWgMu.Lock()
	e.executionWg = nil
	e.executionWgMu.Unlock()

	// Process containers and M3U files after tracks complete (if not shutting down)
	if !e.isShutdownRequested() {
		e.processContainers(plan)
		e.processM3UFiles(plan)
		// Update containers again after M3U processing
		e.processContainers(plan)
	}

	elapsed := time.Since(startTime)
	stats := e.getExecutionStats(plan)

	if e.isShutdownRequested() {
		// Save plan progress on shutdown (via progress callback)
		// The progress callback will handle saving if persistence is enabled
		return stats, fmt.Errorf("plan execution interrupted after %v: %d completed, %d failed, %d pending",
			elapsed, stats["completed"], stats["failed"], stats["pending"])
	}

	return stats, nil
}

// executeTrack executes a single track item.
func (e *Executor) executeTrack(ctx context.Context, item *PlanItem, plan *DownloadPlan) {
	item.MarkStarted()
	e.notifyProgress(item)

	defer func() {
		e.notifyProgress(item)
	}()

	// Check if item has a valid URL (either Spotify or YouTube)
	if item.SpotifyURL == "" && item.YouTubeURL == "" {
		item.MarkFailed("Missing URL for track (neither SpotifyURL nor YouTubeURL provided)")
		return
	}

	// Download track
	success, filePath, err := e.downloader.DownloadTrack(ctx, item)
	if err != nil {
		item.MarkFailed(err.Error())
		return
	}

	if success && filePath != "" {
		item.MarkCompleted(filePath)
	} else {
		item.MarkFailed("Download returned failure")
	}
}

// processContainers processes container items (albums, artists, playlists).
func (e *Executor) processContainers(plan *DownloadPlan) {
	// Process albums
	albumItems := plan.GetItemsByType(PlanItemTypeAlbum)
	for _, albumItem := range albumItems {
		e.updateContainerStatus(albumItem, plan)
	}

	// Process artists
	artistItems := plan.GetItemsByType(PlanItemTypeArtist)
	for _, artistItem := range artistItems {
		e.updateContainerStatus(artistItem, plan)
	}

	// Process playlists
	playlistItems := plan.GetItemsByType(PlanItemTypePlaylist)
	for _, playlistItem := range playlistItems {
		e.updateContainerStatus(playlistItem, plan)
	}
}

// updateContainerStatus updates container item status based on child items.
func (e *Executor) updateContainerStatus(containerItem *PlanItem, plan *DownloadPlan) {
	if len(containerItem.ChildIDs) == 0 {
		containerItem.MarkFailed("Container has no child items")
		return
	}

	// Snapshot child items and their statuses atomically
	e.mu.Lock()
	childItems := make([]*PlanItem, 0, len(containerItem.ChildIDs))
	childStatuses := make(map[string]PlanItemStatus)
	for _, childID := range containerItem.ChildIDs {
		child := plan.GetItem(childID)
		if child != nil {
			childItems = append(childItems, child)
			childStatuses[childID] = child.GetStatus()
		}
	}
	e.mu.Unlock()

	if len(childItems) == 0 {
		containerItem.MarkFailed("All child item references are invalid")
		return
	}

	// Filter to only TRACK items for status calculation
	trackItems := make([]*PlanItem, 0)
	for _, item := range childItems {
		if item.ItemType == PlanItemTypeTrack {
			trackItems = append(trackItems, item)
		}
	}

	if len(trackItems) == 0 {
		// No track items - check status of all children (e.g., M3U items)
		completedCount := 0
		failedCount := 0
		for _, item := range childItems {
			status := childStatuses[item.ItemID]
			switch status {
			case PlanItemStatusCompleted:
				completedCount++
			case PlanItemStatusFailed:
				failedCount++
			}
		}

		if completedCount == len(childItems) {
			containerItem.MarkCompleted("")
		} else if failedCount > 0 {
			pendingCount := 0
			for _, item := range childItems {
				if childStatuses[item.ItemID] == PlanItemStatusPending {
					pendingCount++
				}
			}
			if pendingCount == 0 {
				containerItem.MarkFailed(fmt.Sprintf("%d of %d child items failed", failedCount, len(childItems)))
			}
		}
		return
	}

	// Count track statuses
	completed := 0
	failed := 0
	skipped := 0
	pending := 0
	inProgress := 0

	for _, item := range trackItems {
		status := childStatuses[item.ItemID]
		switch status {
		case PlanItemStatusCompleted:
			completed++
		case PlanItemStatusFailed:
			failed++
		case PlanItemStatusSkipped:
			skipped++
		case PlanItemStatusPending:
			pending++
		case PlanItemStatusInProgress:
			inProgress++
		}
	}

	total := len(trackItems)

	// Update container status
	if total == 0 {
		containerItem.MarkCompleted("")
	} else if completed == total {
		containerItem.MarkCompleted("")
	} else if completed+skipped == total {
		// All completed or skipped (no failures)
		containerItem.MarkCompleted("")
	} else if failed > 0 {
		// Some child items failed
		containerItem.MarkFailed(fmt.Sprintf("%d of %d child items failed (%d completed, %d skipped)",
			failed, total, completed, skipped))
	} else if completed > 0 || skipped > 0 {
		// Partial completion but no failures
		if pending > 0 || inProgress > 0 {
			// Some children are still pending - update progress
			progress := float64(completed+skipped) / float64(total)
			containerItem.mu.Lock()
			containerItem.Progress = progress
			if containerItem.Status == PlanItemStatusPending {
				containerItem.Status = PlanItemStatusInProgress
				now := time.Now()
				containerItem.StartedAt = &now
			}
			containerItem.mu.Unlock()
		} else {
			// All items are processed
			progress := float64(completed+skipped) / float64(total)
			containerItem.MarkCompleted("")
			containerItem.Progress = progress
		}
	}
	// If all items are pending/in_progress, leave status unchanged
}

// processM3UFiles creates M3U files for playlists and albums.
func (e *Executor) processM3UFiles(plan *DownloadPlan) {
	m3uItems := plan.GetItemsByType(PlanItemTypeM3U)

	for _, m3uItem := range m3uItems {
		if m3uItem.Status != PlanItemStatusPending {
			continue // Already processed
		}

		m3uItem.MarkStarted()
		e.notifyProgress(m3uItem)

		// Get parent container
		if m3uItem.ParentID == "" {
			m3uItem.MarkFailed("M3U item missing parent container ID")
			e.notifyProgress(m3uItem)
			continue
		}

		containerItem := plan.GetItem(m3uItem.ParentID)
		if containerItem == nil {
			m3uItem.MarkFailed(fmt.Sprintf("Parent container not found: %s", m3uItem.ParentID))
			e.notifyProgress(m3uItem)
			continue
		}

		// Get all track items for this container
		trackItems := make([]*PlanItem, 0)
		for _, childID := range containerItem.ChildIDs {
			item := plan.GetItem(childID)
			if item != nil && item.ItemType == PlanItemTypeTrack {
				trackItems = append(trackItems, item)
			}
		}

		// Filter to completed or skipped tracks with file paths
		availableTracks := make([]*PlanItem, 0)
		for _, item := range trackItems {
			if (item.Status == PlanItemStatusCompleted || item.Status == PlanItemStatusSkipped) &&
				item.FilePath != "" {
				// Check if file exists
				if _, err := os.Stat(item.FilePath); err == nil {
					availableTracks = append(availableTracks, item)
				}
			}
		}

		if len(availableTracks) == 0 {
			m3uItem.MarkFailed("No available tracks to include in M3U")
			e.notifyProgress(m3uItem)
			continue
		}

		// Create M3U file
		containerName := m3uItem.Metadata["playlist_name"]
		if containerName == nil {
			containerName = m3uItem.Metadata["album_name"]
		}
		if containerName == nil {
			containerName = containerItem.Name
		}

		containerNameStr, ok := containerName.(string)
		if !ok {
			containerNameStr = containerItem.Name
		}

		m3uPath, err := e.createM3UFile(containerNameStr, availableTracks)
		if err != nil {
			m3uItem.MarkFailed(err.Error())
			e.notifyProgress(m3uItem)
			continue
		}

		m3uItem.MarkCompleted(m3uPath)
		e.notifyProgress(m3uItem)
	}
}

// createM3UFile creates an M3U playlist file.
func (e *Executor) createM3UFile(playlistName string, tracks []*PlanItem) (string, error) {
	// Sanitize playlist name for filename
	playlistNameSafe := sanitizeFilename(playlistName)

	// Get base output directory from first track
	if len(tracks) == 0 {
		return "", fmt.Errorf("no tracks provided for M3U file")
	}

	firstTrackPath := tracks[0].FilePath
	baseDir := filepath.Dir(firstTrackPath)

	m3uPath := filepath.Join(baseDir, playlistNameSafe+".m3u")

	// Handle name collisions
	counter := 1
	for {
		if _, err := os.Stat(m3uPath); os.IsNotExist(err) {
			break
		}
		m3uPath = filepath.Join(baseDir, fmt.Sprintf("%s_%d.m3u", playlistNameSafe, counter))
		counter++
		if counter > 100 {
			return "", fmt.Errorf("too many M3U file collisions for %s", playlistName)
		}
	}

	// Create M3U file
	file, err := os.Create(m3uPath)
	if err != nil {
		return "", fmt.Errorf("cannot create M3U file: %w", err)
	}
	defer func() { _ = file.Close() }()

	// Write M3U header
	if _, err := file.WriteString("#EXTM3U\n"); err != nil {
		return "", fmt.Errorf("cannot write M3U header: %w", err)
	}

	// Write tracks
	for _, item := range tracks {
		// Extract title from filename
		title := filepath.Base(item.FilePath)
		title = title[:len(title)-len(filepath.Ext(title))] // Remove extension

		// Get absolute path
		absPath, err := filepath.Abs(item.FilePath)
		if err != nil {
			continue // Skip if we can't get absolute path
		}

		// Write EXTINF line
		if _, err := fmt.Fprintf(file, "#EXTINF:-1,%s\n", title); err != nil {
			continue
		}

		// Write file path
		if _, err := file.WriteString(absPath + "\n"); err != nil {
			continue
		}
	}

	return m3uPath, nil
}

// sanitizeFilename sanitizes a filename by removing invalid characters.
func sanitizeFilename(name string) string {
	// Remove invalid filename characters
	invalidChars := []rune{'/', '\\', ':', '*', '?', '"', '<', '>', '|'}
	result := []rune(name)
	for i, r := range result {
		for _, invalid := range invalidChars {
			if r == invalid {
				result[i] = '_'
				break
			}
		}
	}
	return string(result)
}

// notifyProgress notifies progress callback if set.
func (e *Executor) notifyProgress(item *PlanItem) {
	if e.progressCallback != nil {
		e.progressCallback(item)
	}
}

// getExecutionStats returns execution statistics.
func (e *Executor) getExecutionStats(plan *DownloadPlan) map[string]int {
	// Get statistics for TRACK items only (exclude containers and M3U)
	// Filter out SKIPPED items
	trackItems := make([]*PlanItem, 0)
	for _, item := range plan.Items {
		if item.ItemType == PlanItemTypeTrack && item.Status != PlanItemStatusSkipped {
			trackItems = append(trackItems, item)
		}
	}

	completed := 0
	failed := 0
	pending := 0
	inProgress := 0

	for _, item := range trackItems {
		switch item.Status {
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

// setShutdownRequested sets the shutdown requested flag.
func (e *Executor) setShutdownRequested(value bool) {
	e.shutdownMu.Lock()
	defer e.shutdownMu.Unlock()
	e.shutdownRequested = value
}

// isShutdownRequested checks if shutdown has been requested.
func (e *Executor) isShutdownRequested() bool {
	e.shutdownMu.RLock()
	defer e.shutdownMu.RUnlock()
	return e.shutdownRequested
}

// RequestShutdown requests graceful shutdown of the executor.
func (e *Executor) RequestShutdown() {
	e.setShutdownRequested(true)
}

// WaitForShutdown waits for all active downloads to complete with a timeout.
// Returns true if shutdown completed within timeout, false if timeout exceeded.
// If no execution is active, returns true immediately.
func (e *Executor) WaitForShutdown(timeout time.Duration) bool {
	e.executionWgMu.RLock()
	wg := e.executionWg
	e.executionWgMu.RUnlock()

	if wg == nil {
		// No active execution - shutdown is immediate
		return true
	}

	// Wait for completion with timeout
	done := make(chan struct{})
	go func() {
		defer close(done)
		// Re-check wg is still valid (could be set to nil by another goroutine)
		e.executionWgMu.RLock()
		wg := e.executionWg
		e.executionWgMu.RUnlock()

		if wg != nil {
			wg.Wait()
		}
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		// Timeout occurred - goroutine will continue but will exit when wg.Wait() completes
		// This is acceptable as the goroutine will eventually clean up
		return false
	}
}
