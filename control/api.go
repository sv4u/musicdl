package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/sv4u/musicdl/download/spotify"
)

const (
	APIExitSuccess = 0
	APIExitError   = 1
)

// apiCommand starts the HTTP API server.
func apiCommand(args []string) int {
	port := 5000

	// Parse --port flag
	for i, arg := range args {
		if arg == "--port" && i+1 < len(args) {
			if p, err := strconv.Atoi(args[i+1]); err == nil {
				port = p
			}
		}
	}

	// Check environment variable override
	if envPort := os.Getenv("MUSICDL_API_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			port = p
		}
	}

	server := NewAPIServer(port)
	return server.Run()
}

// APIServer manages the HTTP API server for musicdl.
type APIServer struct {
	port              int
	spotifyClient     *spotify.SpotifyClient
	runnerLock        sync.Mutex
	isRunning         bool
	currentRunTracker *RunTracker
	configPath        string
}

// RunTracker tracks the current running operation.
type RunTracker struct {
	mu            sync.RWMutex
	isRunning     bool
	operationType string // "plan" or "download"
	startedAt     time.Time
	logs          []string
	progress      int
	total         int
	err           error
}

// NewAPIServer creates a new API server.
func NewAPIServer(port int) *APIServer {
	return &APIServer{
		port:              port,
		currentRunTracker: &RunTracker{},
	}
}

// Run starts the API server.
func (s *APIServer) Run() int {
	mux := http.NewServeMux()

	// Health check
	mux.HandleFunc("GET /api/health", s.healthHandler)

	// Config endpoints
	mux.HandleFunc("GET /api/config", s.getConfigHandler)
	mux.HandleFunc("POST /api/config", s.saveConfigHandler)

	// Plan endpoints
	mux.HandleFunc("POST /api/download/plan", s.planHandler)

	// Download endpoints
	mux.HandleFunc("POST /api/download/run", s.downloadHandler)

	// Status endpoints
	mux.HandleFunc("GET /api/download/status", s.statusHandler)
	mux.HandleFunc("GET /api/rate-limit-status", s.rateLimitStatusHandler)

	// Logs endpoint
	mux.HandleFunc("GET /api/logs", s.logsHandler)

	// CORS middleware wrapper
	handler := s.corsMiddleware(mux)

	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	fmt.Printf("Starting musicdl API server on %s\n", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		fmt.Println("\nShutting down API server...")
		server.Close()
	}()

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		return APIExitError
	}

	return APIExitSuccess
}

// corsMiddleware adds CORS headers to all responses.
func (s *APIServer) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// healthHandler returns health status.
// @Summary Health check
// @Description Check API server health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/health [get]
func (s *APIServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "healthy",
		"time":   time.Now().Unix(),
	})
}

// getConfigHandler returns the config file content.
// @Summary Get config
// @Description Retrieve the current config.yaml
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Failure 404 {object} map[string]string
// @Router /api/config [get]
func (s *APIServer) getConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	configPath := "/download/config.yaml"
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "config.yaml not found",
		})
		return
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		http.Error(w, fmt.Sprintf("Error reading config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"config": string(data),
	})
}

// saveConfigHandler saves the config file content.
// @Summary Save config
// @Description Update the config.yaml
// @Accept json
// @Produce json
// @Param config body map[string]string true "Config content"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/config [post]
func (s *APIServer) saveConfigHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	configContent, ok := req["config"]
	if !ok {
		http.Error(w, "Missing 'config' field", http.StatusBadRequest)
		return
	}

	configPath := "/download/config.yaml"
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		http.Error(w, fmt.Sprintf("Error saving config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Config saved successfully",
	})
}

// planHandler generates a download plan.
// @Summary Generate plan
// @Description Generate a download plan from config
// @Accept json
// @Produce json
// @Param configPath body map[string]string true "Path to config file"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /api/download/plan [post]
func (s *APIServer) planHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.runnerLock.Lock()
	defer s.runnerLock.Unlock()

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	configPath, ok := req["configPath"]
	if !ok {
		configPath = "/download/config.yaml"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		http.Error(w, "Config file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "plan_generation_started",
		"message": "Plan generation started. Use /api/download/status to check progress.",
	})
}

// downloadHandler runs the download.
// @Summary Run download
// @Description Execute download from existing plan
// @Accept json
// @Produce json
// @Param configPath body map[string]string true "Path to config file"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Router /api/download/run [post]
func (s *APIServer) downloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.runnerLock.Lock()
	defer s.runnerLock.Unlock()

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	configPath, ok := req["configPath"]
	if !ok {
		configPath = "/download/config.yaml"
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		http.Error(w, "Config file not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "download_started",
		"message": "Download started. Use /api/download/status to check progress.",
	})
}

// statusHandler returns the current status of downloads/plans.
// @Summary Get download status
// @Description Get current progress of plan generation or download
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/download/status [get]
func (s *APIServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.currentRunTracker.mu.RLock()
	defer s.currentRunTracker.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"isRunning":   s.currentRunTracker.isRunning,
		"operationType": s.currentRunTracker.operationType,
		"startedAt":   s.currentRunTracker.startedAt.Unix(),
		"progress":    s.currentRunTracker.progress,
		"total":       s.currentRunTracker.total,
		"logs":        s.currentRunTracker.logs,
		"error":       s.currentRunTracker.err,
	})
}

// rateLimitStatusHandler returns rate limit information.
// @Summary Get rate limit status
// @Description Get Spotify rate limit status and countdown
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/rate-limit-status [get]
func (s *APIServer) rateLimitStatusHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	// Return empty rate limit info (no active rate limit)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"active":                false,
		"retryAfterSeconds":     0,
		"retryAfterTimestamp":   0,
		"detectedAt":            0,
		"remainingSeconds":      0,
	})
}

// logsHandler returns recent logs.
// @Summary Get logs
// @Description Retrieve recent logs from the current operation
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/logs [get]
func (s *APIServer) logsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	s.currentRunTracker.mu.RLock()
	logs := make([]string, len(s.currentRunTracker.logs))
	copy(logs, s.currentRunTracker.logs)
	s.currentRunTracker.mu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs": logs,
	})
}
