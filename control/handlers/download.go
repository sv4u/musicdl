package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
)

// DownloadStart handles POST /api/download/start - Trigger download with config.
func (h *Handlers) DownloadStart(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get current config
	cfg, err := h.configManager.Get()
	if err != nil {
		h.logError("DownloadStart", err)
		response := map[string]interface{}{
			"error":   "Failed to load configuration",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Apply any pending config update before starting
	if err := h.configManager.ApplyPendingUpdate(); err != nil {
		h.logError("DownloadStart", err)
		// Log but continue - pending update failure shouldn't block start
	}

	// Check if service is already running
	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	if svcManager.IsRunning() {
		response := map[string]interface{}{
			"error":   "Download service is already running",
			"message": "Stop the current download before starting a new one",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Start download service process
	if err := svcManager.StartService(ctx); err != nil {
		h.logError("DownloadStart", err)
		response := map[string]interface{}{
			"error":   "Failed to start download service",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get gRPC client
	client, err := svcManager.GetClient(ctx)
	if err != nil {
		h.logError("DownloadStart", err)
		response := map[string]interface{}{
			"error":   "Failed to connect to download service",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Convert config to proto
	protoConfig := convertConfigToProto(cfg)

	// Start download via gRPC
	// Use background context for long-running download
	bgCtx := context.Background()
	if err := client.StartDownload(bgCtx, protoConfig, h.planPath, h.logPath); err != nil {
		h.logError("DownloadStart", err)
		response := map[string]interface{}{
			"error":   "Failed to start download",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	response := map[string]interface{}{
		"message": "Download service started successfully",
		"status":  "running",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// DownloadStop handles POST /api/download/stop - Stop running download.
func (h *Handlers) DownloadStop(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	if !svcManager.IsRunning() {
		response := map[string]interface{}{
			"error":   "Download service is not running",
			"message": "No active download to stop",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get gRPC client
	client, err := svcManager.GetClient(ctx)
	if err != nil {
		h.logError("DownloadStop", err)
		response := map[string]interface{}{
			"error":   "Failed to connect to download service",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Stop download via gRPC
	if err := client.StopDownload(ctx); err != nil {
		h.logError("DownloadStop", err)
		response := map[string]interface{}{
			"error":   "Failed to stop download",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Stop the service process
	if err := svcManager.StopService(ctx); err != nil {
		h.logError("DownloadStop", err)
		// Log but continue - download is stopped
	}

	response := map[string]interface{}{
		"message": "Download service stop requested",
		"status":  "stopping",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// DownloadStatus handles GET /api/download/status - Get current download state.
func (h *Handlers) DownloadStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	if !svcManager.IsRunning() {
		// Service not running, return idle state
		response := map[string]interface{}{
			"state":   "idle",
			"phase":   "idle",
			"message": "Service not running",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get gRPC client
	client, err := svcManager.GetClient(ctx)
	if err != nil {
		// Service process running but can't connect - return error state
		response := map[string]interface{}{
			"state":   "error",
			"phase":   "error",
			"message": "Failed to connect to download service",
			"error":   err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get status via gRPC
	resp, err := client.GetStatus(ctx)
	if err != nil {
		h.logError("DownloadStatus", err)
		response := map[string]interface{}{
			"state":   "error",
			"phase":   "error",
			"message": "Failed to get status",
			"error":   err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Convert proto status to map for JSON response
	status := map[string]interface{}{
		"state":             resp.State.String(),
		"phase":             resp.Phase.String(),
		"progress_percentage": resp.ProgressPercentage,
		"total_items":        resp.TotalItems,
		"completed_items":    resp.CompletedItems,
		"failed_items":       resp.FailedItems,
		"pending_items":      resp.PendingItems,
		"in_progress_items":  resp.InProgressItems,
	}

	if resp.ErrorMessage != "" {
		status["error"] = resp.ErrorMessage
	}

	if resp.StartedAt > 0 {
		status["started_at"] = resp.StartedAt
	}

	if resp.CompletedAt != nil {
		status["completed_at"] = *resp.CompletedAt
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}

// DownloadReset handles POST /api/download/reset - Reset download state.
func (h *Handlers) DownloadReset(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	// Stop service if running
	if svcManager.IsRunning() {
		// Get client and stop download
		client, err := svcManager.GetClient(ctx)
		if err == nil {
			client.StopDownload(ctx)
		}

		// Stop process
		if err := svcManager.StopService(ctx); err != nil {
			h.logError("DownloadReset", err)
		}
	}

	// Clear pending config updates
	h.configManager.ClearPendingUpdate()

	// Delete plan files
	planFiles := []string{
		filepath.Join(h.planPath, "download_plan_progress.json"),
		filepath.Join(h.planPath, "download_plan.json"),
	}

	for _, file := range planFiles {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			h.logError("DownloadReset", err)
		}
	}

	response := map[string]interface{}{
		"message": "Download state reset successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
