package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sv4u/musicdl/download/plan"
)

// Health handles GET /api/health - JSON healthcheck endpoint for Docker HEALTHCHECK.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	var planStatus map[string]interface{}
	var planFile string
	var planStats map[string]interface{}
	var downloadServiceHealth string
	var downloadServiceVersion string

	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	if svcManager.IsRunning() {
		// Service is running - get status via gRPC
		client, err := svcManager.GetClient(ctx)
		if err == nil {
			statusResp, err := client.GetStatus(ctx)
			if err == nil && statusResp != nil {
				planStatus = map[string]interface{}{
					"state": statusResp.State.String(),
					"phase": statusResp.Phase.String(),
				}
				if statusResp.ErrorMessage != "" {
					planStatus["error"] = statusResp.ErrorMessage
				}
				planStats = map[string]interface{}{
					"total":       statusResp.TotalItems,
					"completed":   statusResp.CompletedItems,
					"failed":      statusResp.FailedItems,
					"pending":     statusResp.PendingItems,
					"in_progress": statusResp.InProgressItems,
				}

				// Get health check
				healthResp, err := client.HealthCheck(ctx)
				if err == nil && healthResp != nil {
					downloadServiceHealth = healthResp.ServiceHealth.String()
					if healthResp.ServerVersion != nil {
						downloadServiceVersion = healthResp.ServerVersion.Version
					}
				}
			}
		}
	}

	// Fallback: try to load plan file directly if service not running
	if planStatus == nil && h.planPath != "" {
		// Service not available, try to load plan file directly
		progressPath := filepath.Join(h.planPath, "download_plan_progress.json")
		if _, err := os.Stat(progressPath); err == nil {
			// Plan file exists, try to load it
			if loadedPlan, err := plan.LoadPlan(progressPath); err == nil {
				planFile = progressPath
				planStats = loadedPlan.GetStatistics()
				
				// Determine phase from plan metadata
				phase := "idle"
				if loadedPlan.Metadata != nil {
					if p, ok := loadedPlan.Metadata["phase"].(string); ok {
						phase = p
					}
				}
				
				planStatus = map[string]interface{}{
					"phase": phase,
					"plan_file": planFile,
				}
			}
		}
	}

	// Determine health status based on plan state
	healthStatus := "healthy"
	reason := "Server is responding"
	
	if planStatus != nil {
		if phase, ok := planStatus["phase"].(string); ok {
			switch phase {
			case "error":
				healthStatus = "unhealthy"
				reason = "Service is in error state"
			case "executing":
				healthStatus = "healthy"
				reason = "Service is executing downloads"
			case "generating", "optimizing":
				healthStatus = "healthy"
				reason = "Service is processing plan"
			case "completed":
				healthStatus = "healthy"
				reason = "Service completed execution"
			default:
				healthStatus = "healthy"
				reason = "Service is idle"
			}
		}
	}

	response := map[string]interface{}{
		"status":        healthStatus,
		"reason":        reason,
		"timestamp":     time.Now().Unix(),
		"plan_file":     planFile,
		"statistics":    planStats,
		"server_health": "healthy",
		"services": map[string]interface{}{
			"web_server": map[string]interface{}{
				"status": "healthy",
			},
			"download_service": map[string]interface{}{
				"status":  downloadServiceHealth,
				"version": downloadServiceVersion,
			},
		},
	}

	// Add phase if available
	if planStatus != nil {
		if phase, ok := planStatus["phase"].(string); ok {
			response["phase"] = phase
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if healthStatus == "unhealthy" {
		w.WriteHeader(http.StatusServiceUnavailable)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	json.NewEncoder(w).Encode(response)
}

// HealthStats handles GET /api/health/stats - Detailed health metrics.
func (h *Handlers) HealthStats(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	uptime := time.Since(h.startTime).Seconds()
	
	var planStats map[string]interface{}
	var planStatus string
	var totalProcessed int
	var successCount int
	var failureCount int
	var successRate float64
	var failureRate float64

	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	if svcManager.IsRunning() {
		// Service is running - get status via gRPC
		client, err := svcManager.GetClient(ctx)
		if err == nil {
			statusResp, err := client.GetStatus(ctx)
			if err == nil && statusResp != nil {
				planStatus = statusResp.Phase.String()
				planStats = map[string]interface{}{
					"total":       statusResp.TotalItems,
					"completed":   statusResp.CompletedItems,
					"failed":      statusResp.FailedItems,
					"pending":     statusResp.PendingItems,
					"in_progress": statusResp.InProgressItems,
				}
				totalProcessed = int(statusResp.TotalItems)
				successCount = int(statusResp.CompletedItems)
				failureCount = int(statusResp.FailedItems)
			}
		}
	}

	// Fallback: try to load plan file directly
	if planStats == nil && h.planPath != "" {
		// Service not available, try to load plan file directly
		progressPath := filepath.Join(h.planPath, "download_plan_progress.json")
		if _, err := os.Stat(progressPath); err == nil {
			if loadedPlan, err := plan.LoadPlan(progressPath); err == nil {
				planStats = loadedPlan.GetStatistics()
				if loadedPlan.Metadata != nil {
					if p, ok := loadedPlan.Metadata["phase"].(string); ok {
						planStatus = p
					} else {
						planStatus = "idle"
					}
				} else {
					planStatus = "idle"
				}
			}
		}
	}
	
	// Calculate statistics from plan
	// Try to get plan directly to calculate execution stats
	var loadedPlan *plan.DownloadPlan
	if svcManager.IsRunning() {
		// Get plan items via gRPC and reconstruct plan structure if needed
		// For now, use stats from status response
	} else if h.planPath != "" {
		progressPath := filepath.Join(h.planPath, "download_plan_progress.json")
		if _, err := os.Stat(progressPath); err == nil {
			if p, err := plan.LoadPlan(progressPath); err == nil {
				loadedPlan = p
			}
		}
	}
	
	if loadedPlan != nil {
		// Calculate execution stats from plan items
		// Count only track items (exclude containers and M3U)
		for _, item := range loadedPlan.Items {
			if item.ItemType == plan.PlanItemTypeTrack {
				totalProcessed++
				switch item.GetStatus() {
				case plan.PlanItemStatusCompleted:
					successCount++
				case plan.PlanItemStatusFailed:
					failureCount++
				}
			}
		}
		
		// Calculate rates
		if totalProcessed > 0 {
			successRate = float64(successCount) / float64(totalProcessed) * 100.0
			failureRate = float64(failureCount) / float64(totalProcessed) * 100.0
		}
		
		// Also include plan stats if available
		if planStats == nil {
			planStats = loadedPlan.GetStatistics()
		}
	} else if planStats != nil {
		// Fallback: try to extract from planStats if available
		if total, ok := planStats["total"].(int); ok {
			totalProcessed = total
		} else if total, ok := planStats["total"].(float64); ok {
			totalProcessed = int(total)
		}
		
		if completed, ok := planStats["completed"].(int); ok {
			successCount = completed
		} else if completed, ok := planStats["completed"].(float64); ok {
			successCount = int(completed)
		}
		
		if failed, ok := planStats["failed"].(int); ok {
			failureCount = failed
		} else if failed, ok := planStats["failed"].(float64); ok {
			failureCount = int(failed)
		}
		
		// Calculate rates
		if totalProcessed > 0 {
			successRate = float64(successCount) / float64(totalProcessed) * 100.0
			failureRate = float64(failureCount) / float64(totalProcessed) * 100.0
		}
	}
	
	response := map[string]interface{}{
		"server_health":    "healthy",
		"uptime_seconds":   int64(uptime),
		"plan_status":      planStatus,
		"timestamp":        time.Now().Unix(),
		"statistics": map[string]interface{}{
			"total_processed": totalProcessed,
			"success_count":   successCount,
			"failure_count":    failureCount,
			"success_rate":    successRate,
			"failure_rate":    failureRate,
		},
	}
	
	// Add plan statistics if available
	if planStats != nil {
		response["plan_statistics"] = planStats
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
