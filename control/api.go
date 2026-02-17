package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/sv4u/musicdl/download"
	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/plan"
	"github.com/sv4u/musicdl/download/spotify"
	"github.com/sv4u/spotigo"
	"gopkg.in/yaml.v3"
)

const (
	APIExitSuccess = 0
	APIExitError   = 1
)

// apiCommand starts the HTTP API server.
func apiCommand(args []string) int {
	port := 5000

	// Environment variable (lower priority than CLI flag)
	if envPort := os.Getenv("MUSICDL_API_PORT"); envPort != "" {
		if p, err := strconv.Atoi(envPort); err == nil {
			port = p
		}
	}

	// Parse --port flag (highest priority, overrides env var)
	for i, arg := range args {
		if arg == "--port" && i+1 < len(args) {
			if p, err := strconv.Atoi(args[i+1]); err == nil {
				port = p
			}
		}
	}

	server := NewAPIServer(port)
	return server.Run()
}

// APIServer manages the HTTP API server for musicdl.
type APIServer struct {
	port              int
	spotifyClientMu   sync.RWMutex
	spotifyClient     *spotify.SpotifyClient
	runnerLock        sync.Mutex
	currentRunTracker *RunTracker
	logBroadcaster    *LogBroadcaster
	statsTracker      *StatsTracker
	circuitBreaker    *CircuitBreaker
	resumeState       *ResumeState
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
	cacheDir := getCacheDir()
	return &APIServer{
		port:              port,
		currentRunTracker: &RunTracker{},
		logBroadcaster:    NewLogBroadcaster(),
		statsTracker:      NewStatsTracker(cacheDir),
		circuitBreaker:    NewCircuitBreaker(5, 3, 60*time.Second),
		resumeState:       NewResumeState(cacheDir),
	}
}

// completeRun finalizes a running operation, recording the outcome to the stats
// tracker, circuit breaker, and run tracker. Call this exactly once when an
// operation started by planHandler or downloadHandler finishes (successfully or
// with an error). The runID parameter ensures EndRunByID only finalizes the
// stats for the specific run that started it, preventing the race where a stale
// goroutine could finalize a newer run's stats.
func (s *APIServer) completeRun(runID string, err error) {
	s.currentRunTracker.mu.Lock()
	s.currentRunTracker.isRunning = false
	s.currentRunTracker.err = err
	s.currentRunTracker.mu.Unlock()

	s.statsTracker.EndRunByID(runID)

	if err != nil {
		s.circuitBreaker.RecordFailure()
		s.logBroadcaster.BroadcastString("error", fmt.Sprintf("Operation failed: %v", err), "runner")
	} else {
		s.circuitBreaker.RecordSuccess()
		s.logBroadcaster.BroadcastString("info", "Operation completed successfully", "runner")
	}
}

// initClientsFromConfig creates a Spotify client and audio provider from the
// given config. It also stores the Spotify client on the APIServer so that
// rateLimitStatusHandler can query live rate-limit data.
func (s *APIServer) initClientsFromConfig(cfg *config.MusicDLConfig) (*spotify.SpotifyClient, *audio.Provider, error) {
	spotifyConfig := &spotify.Config{
		ClientID:          cfg.Download.ClientID,
		ClientSecret:      cfg.Download.ClientSecret,
		CacheMaxSize:      cfg.Download.CacheMaxSize,
		CacheTTL:          cfg.Download.CacheTTL,
		RateLimitEnabled:  cfg.Download.SpotifyRateLimitEnabled,
		RateLimitRequests: cfg.Download.SpotifyRateLimitRequests,
		RateLimitWindow:   cfg.Download.SpotifyRateLimitWindow,
		MaxRetries:        cfg.Download.SpotifyMaxRetries,
		RetryBaseDelay:    cfg.Download.SpotifyRetryBaseDelay,
		RetryMaxDelay:     cfg.Download.SpotifyRetryMaxDelay,
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("spotify client error: %w", err)
	}
	s.spotifyClientMu.Lock()
	s.spotifyClient = spotifyClient
	s.spotifyClientMu.Unlock()
	audioConfig := &audio.Config{
		OutputFormat:   cfg.Download.Format,
		Bitrate:        cfg.Download.Bitrate,
		AudioProviders: cfg.Download.AudioProviders,
		CacheMaxSize:   cfg.Download.AudioSearchCacheMaxSize,
		CacheTTL:       cfg.Download.AudioSearchCacheTTL,
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("audio provider error: %w", err)
	}
	return spotifyClient, audioProvider, nil
}

// executePlan loads config, generates a download plan, optimizes it, and saves
// it to the cache directory. Progress and log messages are broadcast through the
// logBroadcaster and currentRunTracker so the web UI can display real-time status.
func (s *APIServer) executePlan(configPath string) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	hash, err := config.HashFromPath(configPath)
	if err != nil {
		return fmt.Errorf("error computing config hash: %w", err)
	}
	cacheDir := getCacheDir()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("error creating cache directory: %w", err)
	}
	spotifyClient, audioProvider, err := s.initClientsFromConfig(cfg)
	if err != nil {
		return err
	}
	playlistTracksFunc := func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
		return spotifyClient.GetPlaylistTracks(ctx, playlistID, opts)
	}
	generator := plan.NewGenerator(cfg, spotifyClient, playlistTracksFunc, audioProvider)
	optimizer := plan.NewOptimizer(true)
	s.logBroadcaster.BroadcastString("info", "Generating download plan...", "plan")
	ctx := context.Background()
	generatedPlan, err := generator.GeneratePlan(ctx)
	if err != nil {
		return fmt.Errorf("plan generation failed: %w", err)
	}
	optimizer.Optimize(generatedPlan)
	configFile := filepath.Base(configPath)
	if err := plan.SavePlanByHash(generatedPlan, cacheDir, hash, configFile); err != nil {
		return fmt.Errorf("error saving plan file: %w", err)
	}
	trackCount := 0
	for _, item := range generatedPlan.Items {
		if item.ItemType == plan.PlanItemTypeTrack {
			trackCount++
		}
	}
	// TODO: Plan generation currently has no per-item progress callback, so the
	// progress jumps from 0/0 to N/N once complete. To support a real progress bar
	// during plan generation, GeneratePlan would need an incremental callback.
	s.currentRunTracker.mu.Lock()
	s.currentRunTracker.total = trackCount
	s.currentRunTracker.progress = trackCount
	s.currentRunTracker.mu.Unlock()
	s.logBroadcaster.BroadcastString("info", fmt.Sprintf("Plan generated: %d tracks found", trackCount), "plan")
	return nil
}

// executeDownload loads config, generates a plan, then downloads all tracks.
// It updates currentRunTracker with per-item progress and records statistics for
// each completed, failed, or skipped track. The resume parameter controls whether
// previously completed items (tracked via ResumeState) should be skipped.
func (s *APIServer) executeDownload(configPath string, resume bool) error {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return fmt.Errorf("configuration error: %w", err)
	}
	hash, err := config.HashFromPath(configPath)
	if err != nil {
		return fmt.Errorf("error computing config hash: %w", err)
	}
	cacheDir := getCacheDir()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("error creating cache directory: %w", err)
	}
	spotifyClient, audioProvider, err := s.initClientsFromConfig(cfg)
	if err != nil {
		return err
	}
	// Generate plan
	playlistTracksFunc := func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
		return spotifyClient.GetPlaylistTracks(ctx, playlistID, opts)
	}
	generator := plan.NewGenerator(cfg, spotifyClient, playlistTracksFunc, audioProvider)
	optimizer := plan.NewOptimizer(true)
	s.logBroadcaster.BroadcastString("info", "Generating download plan...", "download")
	ctx := context.Background()
	generatedPlan, err := generator.GeneratePlan(ctx)
	if err != nil {
		return fmt.Errorf("plan generation failed: %w", err)
	}
	optimizer.Optimize(generatedPlan)
	configFile := filepath.Base(configPath)
	if err := plan.SavePlanByHash(generatedPlan, cacheDir, hash, configFile); err != nil {
		return fmt.Errorf("error saving plan file: %w", err)
	}
	s.logBroadcaster.BroadcastString("info", "Plan generated, loading for execution...", "download")
	// Load plan for execution
	loadedPlan, err := plan.LoadPlanByHash(cacheDir, hash)
	if err != nil {
		return fmt.Errorf("error loading plan: %w", err)
	}
	// If resuming, mark previously completed items as skipped so the executor
	// does not re-download them.
	if resume {
		resumeStatus := s.resumeState.GetStatus()
		if resumeStatus.HasResumeData {
			skipped := 0
			for _, item := range loadedPlan.Items {
				if item.ItemType == plan.PlanItemTypeTrack && s.resumeState.IsCompleted(item.ItemID) {
					item.MarkSkipped("")
					skipped++
				}
			}
			s.logBroadcaster.BroadcastString("info", fmt.Sprintf("Resuming: skipping %d previously completed items", skipped), "download")
		}
	}
	totalTracks := countPendingTracks(loadedPlan)
	s.currentRunTracker.mu.Lock()
	s.currentRunTracker.total = totalTracks
	s.currentRunTracker.mu.Unlock()
	s.logBroadcaster.BroadcastString("info", fmt.Sprintf("Starting download of %d tracks", totalTracks), "download")
	// Set up downloader and executor
	metadataEmbedder := metadata.NewEmbedder()
	downloader := download.NewDownloader(&cfg.Download, spotifyClient, audioProvider, metadataEmbedder)
	maxWorkers := cfg.Download.Threads
	if maxWorkers == 0 {
		maxWorkers = 4
	}
	executor := plan.NewExecutor(downloader, maxWorkers)
	var flushMu sync.Mutex
	var itemsSinceFlush int
	progressCallback := func(item *plan.PlanItem) {
		s.currentRunTracker.mu.Lock()
		s.currentRunTracker.progress++
		s.currentRunTracker.mu.Unlock()
		switch item.Status {
		case plan.PlanItemStatusCompleted:
			var fileSize int64
			if item.FilePath != "" {
				if info, err := os.Stat(item.FilePath); err == nil {
					fileSize = info.Size()
				}
			}
			s.statsTracker.RecordDownload(fileSize)
			s.resumeState.MarkCompleted(item.ItemID)
			s.logBroadcaster.BroadcastString("info", fmt.Sprintf("Downloaded: %s", item.Name), "download")
		case plan.PlanItemStatusFailed:
			s.statsTracker.RecordFailure()
			s.resumeState.MarkFailed(item.ItemID, FailedItemInfo{
				URL:         item.SpotifyURL,
				Name:        item.Name,
				Error:       item.Error,
				Attempts:    1,
				LastAttempt: time.Now().Unix(),
				Retryable:   true,
			})
			s.logBroadcaster.BroadcastString("error", fmt.Sprintf("Failed: %s - %s", item.Name, item.Error), "download")
		case plan.PlanItemStatusSkipped:
			s.statsTracker.RecordSkip()
			s.logBroadcaster.BroadcastString("info", fmt.Sprintf("Skipped: %s", item.Name), "download")
		}
		// Batch resume state writes: flush to disk every 10 items instead of
		// every single track to reduce I/O pressure during large downloads.
		// The flush counter is protected by its own mutex because the executor
		// calls this callback from up to maxWorkers concurrent goroutines.
		flushMu.Lock()
		itemsSinceFlush++
		if itemsSinceFlush >= 10 {
			s.resumeState.Flush()
			itemsSinceFlush = 0
		}
		flushMu.Unlock()
	}
	// Set total items in resume state for progress tracking
	s.resumeState.SetTotalItems(totalTracks)
	stats, execErr := executor.Execute(ctx, loadedPlan, progressCallback)
	// Final flush to persist any remaining batched changes.
	s.resumeState.Flush()
	if execErr != nil {
		return fmt.Errorf("download failed: %w", execErr)
	}
	completed := stats["completed"]
	failed := stats["failed"]
	total := stats["total"]
	s.logBroadcaster.BroadcastString("info", fmt.Sprintf("Download complete: %d/%d succeeded, %d failed", completed, total, failed), "download")
	if failed > 0 {
		return fmt.Errorf("partial failure: %d of %d tracks failed", failed, total)
	}
	return nil
}

// @title musicdl API
// @version 1.0
// @description HTTP API for the musicdl music download tool.
// @host localhost:5000
// @BasePath /api
// @schemes http

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

	// Logs endpoint (HTTP + WebSocket)
	mux.HandleFunc("GET /api/logs", s.logsHandler)
	mux.HandleFunc("GET /api/ws/logs", s.wsLogsHandler)

	// Statistics endpoints
	mux.HandleFunc("GET /api/stats", s.statsHandler)
	mux.HandleFunc("POST /api/stats/reset", s.statsResetHandler)

	// Recovery endpoints
	mux.HandleFunc("GET /api/recovery/status", s.recoveryStatusHandler)
	mux.HandleFunc("POST /api/recovery/circuit-breaker/reset", s.circuitBreakerResetHandler)
	mux.HandleFunc("POST /api/recovery/resume/clear", s.resumeClearHandler)
	mux.HandleFunc("POST /api/recovery/resume/retry-failed", s.resumeRetryFailedHandler)

	// Swagger/OpenAPI documentation
	mux.HandleFunc("GET /api/docs", s.swaggerUIHandler)
	mux.HandleFunc("GET /api/docs/swagger.json", s.swaggerSpecHandler)

	// CORS middleware wrapper
	handler := s.corsMiddleware(mux)

	addr := fmt.Sprintf("0.0.0.0:%d", s.port)
	fmt.Printf("Starting musicdl API server on %s\n", addr)
	fmt.Printf("  Swagger UI: http://localhost:%d/api/docs\n", s.port)
	fmt.Printf("  WebSocket:  ws://localhost:%d/api/ws/logs\n", s.port)

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

// jsonError writes a JSON error response with the given status code.
// This ensures all API error responses use a consistent JSON format
// rather than mixing text/plain (from http.Error) and application/json.
func jsonError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}

// corsMiddleware adds CORS headers to all responses.
// TODO: In production, replace "*" with the specific origin(s) of the frontend
// to prevent cross-site request abuse. The wildcard is acceptable for local
// development but should be locked down before any network-exposed deployment.
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
// @Description Check API server health, WebSocket connections, and circuit breaker state
// @Tags system
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/health [get]
func (s *APIServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	cbStatus := s.circuitBreaker.GetStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":             "healthy",
		"time":               time.Now().Unix(),
		"wsClients":          s.logBroadcaster.ClientCount(),
		"circuitBreakerState": cbStatus.State,
	})
}

// getDefaultConfigPath returns the path to config.yaml, using MUSICDL_WORK_DIR
// if set (e.g. /download in Docker) or falling back to the current directory.
func getDefaultConfigPath() string {
	dir := os.Getenv("MUSICDL_WORK_DIR")
	if dir == "" {
		dir = "."
	}
	return filepath.Join(dir, "config.yaml")
}

// getConfigHandler returns the config file content.
// @Summary Get config
// @Description Retrieve the current config.yaml content
// @Tags config
// @Produce json
// @Success 200 {object} map[string]string
// @Failure 404 {object} map[string]string
// @Router /api/config [get]
func (s *APIServer) getConfigHandler(w http.ResponseWriter, r *http.Request) {
	configPath := getDefaultConfigPath()
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
		jsonError(w, fmt.Sprintf("Error reading config: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"config": string(data),
	})
}

// saveConfigHandler saves the config file content.
// @Summary Save config
// @Description Update the config.yaml content
// @Tags config
// @Accept json
// @Produce json
// @Param body body map[string]string true "Config content"
// @Success 200 {object} map[string]string
// @Failure 400 {object} map[string]string
// @Router /api/config [post]
func (s *APIServer) saveConfigHandler(w http.ResponseWriter, r *http.Request) {
	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	configContent, ok := req["config"]
	if !ok {
		jsonError(w, "Missing 'config' field", http.StatusBadRequest)
		return
	}

	// Validate that the content is well-formed YAML before writing to disk.
	var probe interface{}
	if err := yaml.Unmarshal([]byte(configContent), &probe); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error": fmt.Sprintf("Invalid YAML: %v", err),
		})
		return
	}

	configPath := getDefaultConfigPath()
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		jsonError(w, fmt.Sprintf("Error saving config: %v", err), http.StatusInternalServerError)
		return
	}

	s.logBroadcaster.BroadcastString("info", "Configuration updated", "api")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Config saved successfully",
	})
}

// planHandler generates a download plan.
// @Summary Generate plan
// @Description Generate a download plan from config. Checks circuit breaker before starting.
// @Tags download
// @Accept json
// @Produce json
// @Param body body map[string]string true "Config path"
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 503 {object} map[string]interface{} "Circuit breaker is open"
// @Router /api/download/plan [post]
func (s *APIServer) planHandler(w http.ResponseWriter, r *http.Request) {
	// Check circuit breaker
	if !s.circuitBreaker.AllowRequest() {
		cbStatus := s.circuitBreaker.GetStatus()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":          "Circuit breaker is open - too many consecutive failures",
			"circuitBreaker": cbStatus,
			"suggestion":     "Wait for the circuit breaker to reset or manually reset it via POST /api/recovery/circuit-breaker/reset",
		})
		return
	}

	s.runnerLock.Lock()
	defer s.runnerLock.Unlock()

	// Reject if an operation is already running. The lock serializes this
	// check with completeRun and other handler invocations, preventing two
	// goroutines from racing on currentRunTracker and double-recording
	// outcomes in the circuit breaker / stats tracker.
	s.currentRunTracker.mu.RLock()
	alreadyRunning := s.currentRunTracker.isRunning
	s.currentRunTracker.mu.RUnlock()
	if alreadyRunning {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "An operation is already running. Wait for it to complete or check /api/download/status.",
		})
		return
	}

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	configPath, ok := req["configPath"]
	if !ok {
		configPath = getDefaultConfigPath()
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		jsonError(w, "Config file not found", http.StatusNotFound)
		return
	}

	s.currentRunTracker.mu.Lock()
	s.currentRunTracker.isRunning = true
	s.currentRunTracker.operationType = "plan"
	s.currentRunTracker.startedAt = time.Now()
	s.currentRunTracker.err = nil
	s.currentRunTracker.progress = 0
	s.currentRunTracker.total = 0
	s.currentRunTracker.logs = nil
	s.currentRunTracker.mu.Unlock()

	runID := s.statsTracker.StartRun("plan")
	s.logBroadcaster.BroadcastString("info", "Plan generation started", "plan")

	// Run the operation asynchronously. completeRun MUST be called when done
	// to record the outcome in the circuit breaker and stats tracker. The runID
	// is captured so EndRunByID only finalizes this specific run's stats.
	go func() {
		s.completeRun(runID, s.executePlan(configPath))
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "plan_generation_started",
		"message": "Plan generation started. Use /api/download/status to check progress.",
	})
}

// downloadHandler runs the download.
// @Summary Run download
// @Description Execute download from existing plan. Checks circuit breaker and supports resume.
// @Tags download
// @Accept json
// @Produce json
// @Param body body map[string]string true "Config path and options"
// @Success 202 {object} map[string]interface{}
// @Failure 400 {object} map[string]string
// @Failure 503 {object} map[string]interface{} "Circuit breaker is open"
// @Router /api/download/run [post]
func (s *APIServer) downloadHandler(w http.ResponseWriter, r *http.Request) {
	// Check circuit breaker
	if !s.circuitBreaker.AllowRequest() {
		cbStatus := s.circuitBreaker.GetStatus()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":          "Circuit breaker is open - too many consecutive failures",
			"circuitBreaker": cbStatus,
			"suggestion":     "Wait for the circuit breaker to reset or manually reset it via POST /api/recovery/circuit-breaker/reset",
		})
		return
	}

	s.runnerLock.Lock()
	defer s.runnerLock.Unlock()

	// Reject if an operation is already running. The lock serializes this
	// check with completeRun and other handler invocations, preventing two
	// goroutines from racing on currentRunTracker and double-recording
	// outcomes in the circuit breaker / stats tracker.
	s.currentRunTracker.mu.RLock()
	alreadyRunning := s.currentRunTracker.isRunning
	s.currentRunTracker.mu.RUnlock()
	if alreadyRunning {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "An operation is already running. Wait for it to complete or check /api/download/status.",
		})
		return
	}

	var req map[string]string
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	configPath, ok := req["configPath"]
	if !ok {
		configPath = getDefaultConfigPath()
	}

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		jsonError(w, "Config file not found", http.StatusNotFound)
		return
	}

	resume := req["resume"] == "true"
	resumeStatus := s.resumeState.GetStatus()

	s.currentRunTracker.mu.Lock()
	s.currentRunTracker.isRunning = true
	s.currentRunTracker.operationType = "download"
	s.currentRunTracker.startedAt = time.Now()
	s.currentRunTracker.err = nil
	s.currentRunTracker.progress = 0
	s.currentRunTracker.total = 0
	s.currentRunTracker.logs = nil
	s.currentRunTracker.mu.Unlock()

	runID := s.statsTracker.StartRun("download")
	s.logBroadcaster.BroadcastString("info", "Download started", "download")

	// Run the operation asynchronously. completeRun MUST be called when done
	// to record the outcome in the circuit breaker and stats tracker. The runID
	// is captured so EndRunByID only finalizes this specific run's stats.
	go func() {
		s.completeRun(runID, s.executeDownload(configPath, resume))
	}()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "download_started",
		"message":      "Download started. Use /api/download/status to check progress.",
		"resumeActive": resume && resumeStatus.HasResumeData,
		"resumeStatus": resumeStatus,
	})
}

// statusHandler returns the current status of downloads/plans.
// @Summary Get download status
// @Description Get current progress of plan generation or download, including error details
// @Tags download
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/download/status [get]
func (s *APIServer) statusHandler(w http.ResponseWriter, r *http.Request) {
	s.currentRunTracker.mu.RLock()
	defer s.currentRunTracker.mu.RUnlock()

	var errDetail interface{}
	if s.currentRunTracker.err != nil {
		detail := ClassifyError(s.currentRunTracker.err)
		errDetail = detail
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"isRunning":     s.currentRunTracker.isRunning,
		"operationType": s.currentRunTracker.operationType,
		"startedAt":     s.currentRunTracker.startedAt.Unix(),
		"progress":      s.currentRunTracker.progress,
		"total":         s.currentRunTracker.total,
		"logs":          s.currentRunTracker.logs,
		"error":         errDetail,
	})
}

// rateLimitStatusHandler returns rate limit information.
// @Summary Get rate limit status
// @Description Get current Spotify rate limit status with countdown information
// @Tags download
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/rate-limit-status [get]
func (s *APIServer) rateLimitStatusHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	s.spotifyClientMu.RLock()
	client := s.spotifyClient
	s.spotifyClientMu.RUnlock()
	if client != nil {
		info := client.GetRateLimitInfo()
		if info != nil {
			now := time.Now().Unix()
			remaining := info.RetryAfterTimestamp - now
			if remaining < 0 {
				remaining = 0
			}
			json.NewEncoder(w).Encode(map[string]interface{}{
				"active":              true,
				"retryAfterSeconds":   info.RetryAfterSeconds,
				"retryAfterTimestamp": info.RetryAfterTimestamp,
				"detectedAt":         info.DetectedAt,
				"remainingSeconds":   remaining,
			})
			return
		}
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"active":              false,
		"retryAfterSeconds":   0,
		"retryAfterTimestamp": 0,
		"detectedAt":          0,
		"remainingSeconds":    0,
	})
}

// logsHandler returns recent logs via HTTP.
// @Summary Get logs
// @Description Retrieve recent log history. For real-time streaming use the WebSocket endpoint.
// @Tags logs
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/logs [get]
func (s *APIServer) logsHandler(w http.ResponseWriter, r *http.Request) {
	history := s.logBroadcaster.GetHistory()
	// Derive WebSocket URL from the request's Host header so the URL is valid
	// regardless of whether the client connects via localhost, a container
	// hostname, or a remote address.
	wsScheme := "ws"
	if r.TLS != nil {
		wsScheme = "wss"
	}
	wsURL := fmt.Sprintf("%s://%s/api/ws/logs", wsScheme, r.Host)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":   history,
		"wsUrl":  wsURL,
		"wsHint": "Use the WebSocket endpoint for real-time log streaming",
	})
}

// wsLogsHandler upgrades the connection to WebSocket for real-time log streaming.
// @Summary WebSocket log stream
// @Description Real-time log streaming via WebSocket. Sends history on connect, then live updates.
// @Tags logs
// @Router /api/ws/logs [get]
func (s *APIServer) wsLogsHandler(w http.ResponseWriter, r *http.Request) {
	s.logBroadcaster.HandleWebSocket(w, r)
}

// statsHandler returns download statistics.
// @Summary Get statistics
// @Description Get per-run and cumulative download statistics
// @Tags stats
// @Produce json
// @Success 200 {object} StatsResponse
// @Router /api/stats [get]
func (s *APIServer) statsHandler(w http.ResponseWriter, r *http.Request) {
	stats := s.statsTracker.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// statsResetHandler resets cumulative statistics.
// @Summary Reset statistics
// @Description Reset all cumulative statistics to zero
// @Tags stats
// @Produce json
// @Success 200 {object} map[string]string
// @Router /api/stats/reset [post]
func (s *APIServer) statsResetHandler(w http.ResponseWriter, r *http.Request) {
	s.statsTracker.Reset()
	s.logBroadcaster.BroadcastString("info", "Statistics reset", "api")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Statistics reset successfully",
	})
}

// recoveryStatusHandler returns the combined recovery status.
// @Summary Get recovery status
// @Description Get circuit breaker state and resume state for error recovery
// @Tags recovery
// @Produce json
// @Success 200 {object} RecoveryStatus
// @Router /api/recovery/status [get]
func (s *APIServer) recoveryStatusHandler(w http.ResponseWriter, r *http.Request) {
	status := RecoveryStatus{
		CircuitBreaker: s.circuitBreaker.GetStatus(),
		Resume:         s.resumeState.GetStatus(),
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// circuitBreakerResetHandler manually resets the circuit breaker.
// @Summary Reset circuit breaker
// @Description Manually reset the circuit breaker to closed state
// @Tags recovery
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/recovery/circuit-breaker/reset [post]
func (s *APIServer) circuitBreakerResetHandler(w http.ResponseWriter, r *http.Request) {
	s.circuitBreaker.Reset()
	s.logBroadcaster.BroadcastString("info", "Circuit breaker manually reset", "recovery")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":        "Circuit breaker reset to closed state",
		"circuitBreaker": s.circuitBreaker.GetStatus(),
	})
}

// resumeClearHandler clears the resume state for a fresh run.
// @Summary Clear resume state
// @Description Clear all resume/checkpoint data for a fresh download run
// @Tags recovery
// @Produce json
// @Success 200 {object} map[string]string
// @Router /api/recovery/resume/clear [post]
func (s *APIServer) resumeClearHandler(w http.ResponseWriter, r *http.Request) {
	s.resumeState.Clear()
	s.logBroadcaster.BroadcastString("info", "Resume state cleared", "recovery")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Resume state cleared",
	})
}

// resumeRetryFailedHandler retries only the failed items from the last run.
// @Summary Retry failed items
// @Description Get the list of retryable failed items from the last run
// @Tags recovery
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/recovery/resume/retry-failed [post]
func (s *APIServer) resumeRetryFailedHandler(w http.ResponseWriter, r *http.Request) {
	retryable := s.resumeState.RetryableErrors()
	s.logBroadcaster.BroadcastString("info", fmt.Sprintf("Retrying %d failed items", len(retryable)), "recovery")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message":       fmt.Sprintf("Found %d retryable items", len(retryable)),
		"retryableItems": retryable,
	})
}

// swaggerUIHandler serves the Swagger UI page.
func (s *APIServer) swaggerUIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write([]byte(swaggerUIHTML(s.port)))
}

// swaggerSpecHandler returns the OpenAPI specification.
func (s *APIServer) swaggerSpecHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(generateOpenAPISpec(s.port)))
}
