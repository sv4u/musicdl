package handlers

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/sv4u/musicdl/download/plan"
)

// Health handles GET /api/health - JSON healthcheck endpoint for Docker HEALTHCHECK.
func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	// Try to get service status first (if service is initialized)
	service, err := h.getService()
	var planStatus map[string]interface{}
	var planFile string
	var planStats map[string]interface{}
	
	if err == nil && service != nil {
		// Service is available, use its status
		status := service.GetStatus()
		planStatus = status
		if pf, ok := status["plan_file"].(string); ok {
			planFile = pf
		}
		if ps, ok := status["plan_stats"].(map[string]interface{}); ok {
			planStats = ps
		}
	} else if h.planPath != "" {
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
	uptime := time.Since(h.startTime).Seconds()
	
	// Try to get service status and plan statistics
	service, err := h.getService()
	var planStats map[string]interface{}
	var planStatus string
	var totalProcessed int
	var successCount int
	var failureCount int
	var successRate float64
	var failureRate float64
	
	if err == nil && service != nil {
		// Service is available, use its status
		status := service.GetStatus()
		if ps, ok := status["plan_stats"].(map[string]interface{}); ok {
			planStats = ps
		}
		if phase, ok := status["phase"].(string); ok {
			planStatus = phase
		} else {
			planStatus = "idle"
		}
	} else if h.planPath != "" {
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
	if err == nil && service != nil {
		loadedPlan = service.GetPlan()
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
