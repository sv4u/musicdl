package main

import (
	"context"
	"log"
	"time"

	"github.com/sv4u/musicdl/download/graph"
	"github.com/sv4u/musicdl/download/plan"
)

// graphSyncPlan syncs a generated plan into the graph. Non-fatal on error.
func graphSyncPlan(ctx context.Context, graphClient *graph.Client, dp *plan.DownloadPlan, runID string) {
	if graphClient == nil || dp == nil {
		return
	}
	if err := graphClient.SyncPlan(ctx, dp, runID); err != nil {
		log.Printf("WARN: graph_sync plan failed (non-fatal): %v", err)
	}
}

// graphSyncDownloadResults syncs all track and M3U results after execution.
func graphSyncDownloadResults(ctx context.Context, graphClient *graph.Client, dp *plan.DownloadPlan, runID string, stats map[string]int) {
	if graphClient == nil || dp == nil {
		return
	}

	start := time.Now()
	trackCount := 0
	m3uCount := 0

	for _, item := range dp.Items {
		switch item.ItemType {
		case plan.PlanItemTypeTrack:
			if item.GetStatus() != plan.PlanItemStatusPending {
				if err := graphClient.SyncTrackResult(ctx, item, runID); err != nil {
					log.Printf("WARN: graph_sync track %q: %v", item.Name, err)
				}
				trackCount++
			}
		case plan.PlanItemTypeM3U:
			if item.GetStatus() == plan.PlanItemStatusCompleted && item.GetFilePath() != "" {
				tracks := collectM3UTracks(item, dp)
				if err := graphClient.SyncM3U(ctx, item, tracks, runID); err != nil {
					log.Printf("WARN: graph_sync m3u %q: %v", item.Name, err)
				}
				m3uCount++
			}
		}
	}

	if err := graphClient.CompleteRun(ctx, runID, stats); err != nil {
		log.Printf("WARN: graph_sync run completion: %v", err)
	}

	log.Printf("INFO: graph_sync download results synced in %v (%d tracks, %d m3u)", time.Since(start), trackCount, m3uCount)
}

// collectM3UTracks gathers the track items referenced by an M3U plan item.
func collectM3UTracks(m3uItem *plan.PlanItem, dp *plan.DownloadPlan) []*plan.PlanItem {
	if m3uItem.ParentID == "" {
		return nil
	}
	container := dp.GetItem(m3uItem.ParentID)
	if container == nil {
		return nil
	}
	var tracks []*plan.PlanItem
	for _, childID := range container.ChildIDs {
		child := dp.GetItem(childID)
		if child == nil || child.ItemType != plan.PlanItemTypeTrack {
			continue
		}
		if (child.GetStatus() == plan.PlanItemStatusCompleted || child.GetStatus() == plan.PlanItemStatusSkipped) && child.GetFilePath() != "" {
			tracks = append(tracks, child)
		}
	}
	return tracks
}

// generateRunID creates a run ID from the current timestamp.
func generateRunID() string {
	return time.Now().Format("20060102_150405")
}
