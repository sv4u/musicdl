package handlers

import (
	"encoding/json"
	"net/http"
)

// DownloadStart handles POST /api/download/start - Trigger download with config file path.
func (h *Handlers) DownloadStart(w http.ResponseWriter, r *http.Request) {
	// Get service (will initialize if needed)
	service, err := h.getService()
	if err != nil {
		h.logError("getService", err)
		response := map[string]interface{}{
			"error":   "Failed to initialize download service",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Start download service
	ctx := r.Context()
	if err := service.Start(ctx); err != nil {
		h.logError("DownloadStart", err)
		response := map[string]interface{}{
			"error":   "Failed to start download service",
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
	// Get service
	service, err := h.getService()
	if err != nil {
		h.logError("getService", err)
		response := map[string]interface{}{
			"error":   "Failed to get download service",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Stop download service
	if err := service.Stop(); err != nil {
		h.logError("DownloadStop", err)
		response := map[string]interface{}{
			"error":   "Failed to stop download service",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
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
	// Get service (may not be initialized yet)
	service, err := h.getService()
	if err != nil {
		// Service not initialized yet, return idle state
		response := map[string]interface{}{
			"state":   "idle",
			"message": "Service not initialized",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get status from service
	status := service.GetStatus()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(status)
}
