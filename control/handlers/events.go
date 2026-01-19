package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"
)

// EventStream manages Server-Sent Events (SSE) connections.
type EventStream struct {
	clients map[chan []byte]bool
	mu      sync.RWMutex
}

var globalEventStream = &EventStream{
	clients: make(map[chan []byte]bool),
}

// addClient adds a new SSE client.
func (es *EventStream) addClient(client chan []byte) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.clients[client] = true
}

// removeClient removes an SSE client.
func (es *EventStream) removeClient(client chan []byte) {
	es.mu.Lock()
	defer es.mu.Unlock()
	delete(es.clients, client)
	close(client)
}

// broadcast sends a message to all connected clients.
func (es *EventStream) broadcast(data []byte) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	for client := range es.clients {
		select {
		case client <- data:
		default:
			// Client channel is full, skip
		}
	}
}

// StatusStream handles GET /api/status/stream - SSE stream for real-time status updates.
func (h *Handlers) StatusStream(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Create client channel
	clientChan := make(chan []byte, 10)
	globalEventStream.addClient(clientChan)
	defer globalEventStream.removeClient(clientChan)

	// Send initial status
	status := h.getStatusData()
	if data, err := json.Marshal(status); err == nil {
		fmt.Fprintf(w, "data: %s\n\n", string(data))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}

	// Keep connection alive and send updates
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case data := <-clientChan:
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-ticker.C:
			// Send heartbeat to keep connection alive
			fmt.Fprintf(w, ": heartbeat\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			// Client disconnected
			return
		}
	}
}

// BroadcastStatus broadcasts status update to all SSE clients.
func BroadcastStatus(status map[string]interface{}) {
	data, err := json.Marshal(status)
	if err != nil {
		log.Printf("ERROR: failed to marshal status for broadcast: %v", err)
		return
	}
	globalEventStream.broadcast(data)
}

// getStatusData gets the current status data.
func (h *Handlers) getStatusData() map[string]interface{} {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	if !svcManager.IsRunning() {
		return map[string]interface{}{
			"state":      "idle",
			"phase":      "idle",
			"statistics": map[string]interface{}{},
			"plan_file":  nil,
			"message":    "Service not running",
		}
	}

	// Get gRPC client
	client, err := svcManager.GetClient(ctx)
	if err != nil {
		return map[string]interface{}{
			"state":      "error",
			"phase":      "error",
			"statistics": map[string]interface{}{},
			"plan_file":  nil,
			"message":    "Failed to connect to download service",
			"error":      err.Error(),
		}
	}

	// Get status via gRPC
	statusResp, err := client.GetStatus(ctx)
	if err != nil {
		return map[string]interface{}{
			"state":      "error",
			"phase":      "error",
			"statistics": map[string]interface{}{},
			"plan_file":  nil,
			"message":    "Failed to get status",
			"error":      err.Error(),
		}
	}

	// Get plan items
	var planData interface{}
	planItemsResp, err := client.GetPlanItems(ctx, nil)
	if err == nil && planItemsResp != nil {
		planData = map[string]interface{}{
			"item_count": len(planItemsResp.Items),
		}
	}

	// Build response
	statistics := map[string]interface{}{
		"total":       statusResp.TotalItems,
		"completed":   statusResp.CompletedItems,
		"failed":      statusResp.FailedItems,
		"pending":     statusResp.PendingItems,
		"in_progress": statusResp.InProgressItems,
	}

	response := map[string]interface{}{
		"state":      statusResp.State.String(),
		"phase":      statusResp.Phase.String(),
		"statistics": statistics,
		"plan_file":  nil,
		"plan":       planData,
	}

	if statusResp.ErrorMessage != "" {
		response["error"] = statusResp.ErrorMessage
	}

	if statusResp.StartedAt > 0 {
		response["started_at"] = time.Unix(statusResp.StartedAt, 0).Format(time.RFC3339)
	}

	if statusResp.CompletedAt != nil {
		response["completed_at"] = time.Unix(*statusResp.CompletedAt, 0).Format(time.RFC3339)
	}

	return response
}
