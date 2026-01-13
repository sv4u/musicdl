package handlers

import (
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
	service, err := h.getService()
	if err != nil || service == nil {
		return map[string]interface{}{
			"state":      "idle",
			"phase":      nil,
			"statistics": map[string]interface{}{},
			"plan_file":  nil,
			"message":    "Service not initialized",
		}
	}

	status := service.GetStatus()

	// Get plan if available
	var planData interface{}
	if plan := service.GetPlan(); plan != nil {
		stats := plan.GetExecutionStatistics()
		planData = map[string]interface{}{
			"statistics": stats,
			"item_count": len(plan.Items),
		}
	}

	// Build response
	response := map[string]interface{}{
		"state":      status["state"],
		"phase":      status["phase"],
		"statistics": status["plan_stats"],
		"plan_file":  status["plan_file"],
		"plan":       planData,
	}

	if errorMsg, ok := status["error"].(string); ok && errorMsg != "" {
		response["error"] = errorMsg
	}

	if startedAt, ok := status["started_at"].(string); ok {
		response["started_at"] = startedAt
	}

	if completedAt, ok := status["completed_at"].(string); ok {
		response["completed_at"] = completedAt
	}

	return response
}
