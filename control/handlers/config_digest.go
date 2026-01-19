package handlers

import (
	"encoding/json"
	"net/http"
)

// ConfigDigest handles GET /api/config/digest - Get configuration digest and version.
func (h *Handlers) ConfigDigest(w http.ResponseWriter, r *http.Request) {
	// Get config digest
	digest, err := h.configManager.GetDigest()
	if err != nil {
		h.logError("ConfigDigest", err)
		response := map[string]interface{}{
			"error":   "Failed to get config digest",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get config to extract version
	cfg, err := h.configManager.Get()
	if err != nil {
		h.logError("ConfigDigest", err)
		response := map[string]interface{}{
			"error":   "Failed to get config",
			"message": err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Check for pending updates
	_, hasPending := h.configManager.GetPendingUpdate()

	response := map[string]interface{}{
		"digest":       digest,
		"version":      cfg.Version,
		"has_pending":  hasPending,
		"config_stats": map[string]interface{}{
			"songs":     len(cfg.Songs),
			"artists":   len(cfg.Artists),
			"playlists": len(cfg.Playlists),
			"albums":    len(cfg.Albums),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}
