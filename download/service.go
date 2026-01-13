package download

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/plan"
	"github.com/sv4u/musicdl/download/spotify"
	"github.com/sv4u/spotigo"
)

// ServiceState represents the state of the download service.
type ServiceState string

const (
	ServiceStateIdle     ServiceState = "idle"
	ServiceStateRunning  ServiceState = "running"
	ServiceStateStopping ServiceState = "stopping"
	ServiceStateError    ServiceState = "error"
)

// ServicePhase represents the current execution phase.
type ServicePhase string

const (
	ServicePhaseIdle       ServicePhase = "idle"
	ServicePhaseGenerating ServicePhase = "generating"
	ServicePhaseOptimizing ServicePhase = "optimizing"
	ServicePhaseExecuting  ServicePhase = "executing"
	ServicePhaseCompleted  ServicePhase = "completed"
	ServicePhaseError      ServicePhase = "error"
)

// Service is the main download service that orchestrates downloads.
type Service struct {
	config           *config.MusicDLConfig
	spotifyClient    *spotify.SpotifyClient
	audioProvider    *audio.Provider
	metadataEmbedder *metadata.Embedder
	downloader       *Downloader
	executor         *plan.Executor
	generator        *plan.Generator
	optimizer        *plan.Optimizer
	planPath         string

	// State management
	mu           sync.RWMutex
	state        ServiceState
	phase        ServicePhase
	currentPlan  *plan.DownloadPlan
	errorMessage string
	startedAt    *time.Time
	completedAt  *time.Time

	// Progress tracking
	progressPercentage float64
	progressMu         sync.RWMutex

	// Plan persistence throttling
	lastPlanSave time.Time
	planSaveMu   sync.Mutex
}

// NewService creates a new download service.
func NewService(cfg *config.MusicDLConfig, spotifyClient *spotify.SpotifyClient, audioProvider *audio.Provider, metadataEmbedder *metadata.Embedder, planPath string) (*Service, error) {
	// Create downloader
	downloader := NewDownloader(&cfg.Download, spotifyClient, audioProvider, metadataEmbedder)

	// Create executor
	maxWorkers := cfg.Download.Threads
	if maxWorkers == 0 {
		maxWorkers = 4
	}
	executor := plan.NewExecutor(downloader, maxWorkers)

	// Create generator with playlist tracks function
	playlistTracksFunc := func(ctx context.Context, playlistID string, opts *spotigo.PlaylistTracksOptions) (*spotigo.Paging[spotigo.PlaylistTrack], error) {
		return spotifyClient.GetPlaylistTracks(ctx, playlistID, opts)
	}
	// audioProvider implements YouTubeMetadataProvider interface
	generator := plan.NewGenerator(cfg, spotifyClient, playlistTracksFunc, audioProvider)

	// Create optimizer
	optimizer := plan.NewOptimizer(true) // Check file existence

	return &Service{
		config:           cfg,
		spotifyClient:    spotifyClient,
		audioProvider:    audioProvider,
		metadataEmbedder: metadataEmbedder,
		downloader:       downloader,
		executor:         executor,
		generator:        generator,
		optimizer:        optimizer,
		planPath:         planPath,
		state:            ServiceStateIdle,
		phase:            ServicePhaseIdle,
	}, nil
}

// Start starts the download service with the current configuration.
func (s *Service) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == ServiceStateRunning {
		return fmt.Errorf("service is already running")
	}

	if s.state == ServiceStateStopping {
		return fmt.Errorf("service is stopping, please wait")
	}

	oldState := s.state
	oldPhase := s.phase

	// Check for existing plan if persistence is enabled
	var generatedPlan *plan.DownloadPlan
	if s.config.Download.PlanPersistenceEnabled && s.planPath != "" {
		progressPath := filepath.Join(s.planPath, "download_plan_progress.json")
		if existingPlan, err := plan.LoadPlan(progressPath); err == nil {
			log.Printf("INFO: plan_loaded_from_file path=%s items=%d", progressPath, len(existingPlan.Items))
			generatedPlan = existingPlan
			// Skip generation and optimization if plan exists
			s.phase = ServicePhaseExecuting
			s.currentPlan = generatedPlan
			s.state = ServiceStateRunning
			now := time.Now()
			s.startedAt = &now
			s.completedAt = nil
			if oldState != s.state {
				log.Printf("INFO: state_transition state=%s -> %s", oldState, s.state)
			}
			if oldPhase != s.phase {
				log.Printf("INFO: phase_transition phase=%s -> %s", oldPhase, s.phase)
			}
			log.Printf("INFO: plan_execution_resume items=%d", len(generatedPlan.Items))
			// Start execution in a goroutine
			go func() {
				stats, err := s.executor.Execute(ctx, generatedPlan, s.progressCallback)

				// Check if context was cancelled
				if ctx.Err() != nil {
					log.Printf("INFO: plan_execution_cancelled error=%v", ctx.Err())
					return
				}

				s.mu.Lock()
				defer s.mu.Unlock()
				if err != nil {
					s.state = ServiceStateError
					s.phase = ServicePhaseError
					s.errorMessage = err.Error()
					log.Printf("ERROR: plan_execution_failed error=%v", err)
				} else {
					s.state = ServiceStateIdle
					s.phase = ServicePhaseCompleted
					s.errorMessage = ""
					completed := stats["completed"]
					failed := stats["failed"]
					pending := stats["pending"]
					inProgress := stats["in_progress"]
					total := stats["total"]
					log.Printf("INFO: plan_execution_complete completed=%d failed=%d pending=%d in_progress=%d total=%d",
						completed, failed, pending, inProgress, total)
				}
				now := time.Now()
				s.completedAt = &now
				log.Printf("INFO: state_transition state=%s phase=%s", s.state, s.phase)
				// Save final plan state
				s.savePlan()
			}()
			return nil
		} else if !os.IsNotExist(err) {
			log.Printf("WARN: failed_to_load_plan path=%s error=%v", progressPath, err)
		}
	}

	// Generate plan
	log.Printf("INFO: plan_generation_start")
	s.phase = ServicePhaseGenerating
	if oldPhase != s.phase {
		log.Printf("INFO: phase_transition phase=%s -> %s", oldPhase, s.phase)
	}

	var err error
	generatedPlan, err = s.generator.GeneratePlan(ctx)
	if err != nil {
		s.state = ServiceStateError
		s.phase = ServicePhaseError
		s.errorMessage = fmt.Sprintf("failed to generate plan: %v", err)
		log.Printf("ERROR: plan_generation_failed error=%v", err)
		if oldState != s.state {
			log.Printf("INFO: state_transition state=%s -> %s", oldState, s.state)
		}
		return fmt.Errorf("failed to generate plan: %w", err)
	}

	itemCount := len(generatedPlan.Items)
	log.Printf("INFO: plan_generation_complete items=%d", itemCount)

	// Optimize plan
	log.Printf("INFO: plan_optimization_start items=%d", itemCount)
	s.phase = ServicePhaseOptimizing
	if oldPhase != s.phase {
		log.Printf("INFO: phase_transition phase=%s -> %s", ServicePhaseGenerating, s.phase)
	}

	s.optimizer.Optimize(generatedPlan)

	optimizedCount := len(generatedPlan.Items)
	log.Printf("INFO: plan_optimization_complete items=%d", optimizedCount)

	// Set state to running
	s.state = ServiceStateRunning
	s.phase = ServicePhaseExecuting
	s.currentPlan = generatedPlan
	s.errorMessage = ""
	now := time.Now()
	s.startedAt = &now
	s.completedAt = nil

	if oldState != s.state {
		log.Printf("INFO: state_transition state=%s -> %s", oldState, s.state)
	}
	if oldPhase != s.phase {
		log.Printf("INFO: phase_transition phase=%s -> %s", ServicePhaseOptimizing, s.phase)
	}

	log.Printf("INFO: plan_execution_start items=%d", optimizedCount)

	// Start execution in a goroutine
	go func() {
		stats, err := s.executor.Execute(ctx, generatedPlan, s.progressCallback)

		// Check if context was cancelled
		if ctx.Err() != nil {
			log.Printf("INFO: plan_execution_cancelled error=%v", ctx.Err())
			return
		}

		s.mu.Lock()
		defer s.mu.Unlock()

		if err != nil {
			s.state = ServiceStateError
			s.phase = ServicePhaseError
			s.errorMessage = err.Error()
			log.Printf("ERROR: plan_execution_failed error=%v", err)
		} else {
			s.state = ServiceStateIdle
			s.phase = ServicePhaseCompleted
			s.errorMessage = ""
			completed := stats["completed"]
			failed := stats["failed"]
			pending := stats["pending"]
			inProgress := stats["in_progress"]
			total := stats["total"]
			log.Printf("INFO: plan_execution_complete completed=%d failed=%d pending=%d in_progress=%d total=%d",
				completed, failed, pending, inProgress, total)
		}
		now := time.Now()
		s.completedAt = &now

		log.Printf("INFO: state_transition state=%s phase=%s", s.state, s.phase)
		// Save final plan state
		s.savePlan()
	}()

	return nil
}

// Stop stops the download service gracefully.
func (s *Service) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != ServiceStateRunning {
		return fmt.Errorf("service is not running (current state: %s)", s.state)
	}

	oldState := s.state
	s.state = ServiceStateStopping
	log.Printf("INFO: state_transition state=%s -> %s", oldState, s.state)

	// Save plan before shutdown
	s.savePlan()

	s.executor.RequestShutdown()

	// Wait for shutdown (with timeout)
	// Default timeout: 30 seconds
	shutdownTimeout := 30 * time.Second

	// Wait for executor to complete with timeout
	completed := s.executor.WaitForShutdown(shutdownTimeout)
	if !completed {
		log.Printf("WARN: shutdown_timeout_exceeded timeout=%v, forcing cleanup", shutdownTimeout)
		// Force cleanup of resources
		if s.spotifyClient != nil {
			s.spotifyClient.Close()
		}
		if s.audioProvider != nil {
			// Audio provider cleanup if needed
			// (Currently no Close method, but add if needed in future)
		}
		s.state = ServiceStateError
		s.errorMessage = fmt.Sprintf("Shutdown timeout exceeded after %v", shutdownTimeout)
		return fmt.Errorf("shutdown timeout exceeded after %v", shutdownTimeout)
	}

	log.Printf("INFO: shutdown_complete")
	s.state = ServiceStateIdle
	s.phase = ServicePhaseIdle
	return nil
}

// GetStatus returns the current service status.
func (s *Service) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := map[string]interface{}{
		"state": s.state,
		"phase": s.phase,
	}

	if s.errorMessage != "" {
		status["error"] = s.errorMessage
	}

	if s.startedAt != nil {
		status["started_at"] = s.startedAt.Format(time.RFC3339)
	}

	if s.completedAt != nil {
		status["completed_at"] = s.completedAt.Format(time.RFC3339)
	}

	// Add plan statistics if available
	if s.currentPlan != nil {
		execStats := s.currentPlan.GetExecutionStatistics()
		// Convert map[string]int to map[string]interface{} for JSON serialization
		stats := make(map[string]interface{})
		for k, v := range execStats {
			stats[k] = v
		}
		status["plan_stats"] = stats
	}

	// Add progress percentage
	s.progressMu.RLock()
	status["progress_percentage"] = s.progressPercentage
	s.progressMu.RUnlock()

	// Add plan file path if persistence is enabled
	if s.config.Download.PlanPersistenceEnabled && s.planPath != "" {
		progressPath := filepath.Join(s.planPath, "download_plan_progress.json")
		status["plan_file"] = progressPath
	} else {
		status["plan_file"] = nil
	}

	return status
}

// GetPlan returns the current download plan.
func (s *Service) GetPlan() *plan.DownloadPlan {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentPlan
}

// progressCallback is called when plan items are updated.
func (s *Service) progressCallback(item *plan.PlanItem) {
	// Calculate progress percentage
	s.updateProgress()

	// Save plan periodically if persistence is enabled (throttled to every 2 seconds)
	if s.config.Download.PlanPersistenceEnabled {
		s.savePlanThrottled()
	}
}

// updateProgress calculates and updates the current progress percentage.
func (s *Service) updateProgress() {
	s.mu.RLock()
	currentPlan := s.currentPlan
	s.mu.RUnlock()

	if currentPlan == nil {
		s.progressMu.Lock()
		s.progressPercentage = 0.0
		s.progressMu.Unlock()
		return
	}

	// Count track items and their statuses
	totalTracks := 0
	completedTracks := 0

	for _, item := range currentPlan.Items {
		if item.ItemType == plan.PlanItemTypeTrack {
			totalTracks++
			status := item.GetStatus()
			if status == plan.PlanItemStatusCompleted || status == plan.PlanItemStatusSkipped {
				completedTracks++
			}
		}
	}

	// Calculate percentage
	percentage := 0.0
	if totalTracks > 0 {
		percentage = float64(completedTracks) / float64(totalTracks) * 100.0
	}

	s.progressMu.Lock()
	s.progressPercentage = percentage
	s.progressMu.Unlock()

	log.Printf("INFO: progress_update completed=%d total=%d percentage=%.2f%%", completedTracks, totalTracks, percentage)
}

// savePlanThrottled saves the plan with throttling (max once every 2 seconds).
func (s *Service) savePlanThrottled() {
	s.planSaveMu.Lock()
	defer s.planSaveMu.Unlock()

	now := time.Now()
	if now.Sub(s.lastPlanSave) < 2*time.Second {
		return // Skip if less than 2 seconds since last save
	}

	s.lastPlanSave = now
	s.savePlan()
}

// savePlan saves the current plan to disk if persistence is enabled.
func (s *Service) savePlan() {
	if !s.config.Download.PlanPersistenceEnabled || s.planPath == "" {
		return
	}

	s.mu.RLock()
	currentPlan := s.currentPlan
	currentPhase := s.phase
	if currentPlan == nil {
		s.mu.RUnlock()
		return
	}

	// Create a deep copy of metadata to avoid race conditions
	metadataCopy := make(map[string]interface{})
	for k, v := range currentPlan.Metadata {
		metadataCopy[k] = v
	}
	metadataCopy["phase"] = string(currentPhase)
	metadataCopy["phase_updated_at"] = time.Now().Unix()
	metadataCopy["last_saved_at"] = time.Now().Unix()
	s.mu.RUnlock()

	// Create a copy of the plan for saving
	planCopy := &plan.DownloadPlan{
		Items:    currentPlan.Items, // Items are read-only during save
		Metadata: metadataCopy,
	}

	progressPath := filepath.Join(s.planPath, "download_plan_progress.json")
	if err := planCopy.Save(progressPath); err != nil {
		log.Printf("ERROR: plan_save_failed path=%s error=%v", progressPath, err)
	} else {
		log.Printf("INFO: plan_saved path=%s items=%d", progressPath, len(planCopy.Items))
	}
}

// WaitForCompletion waits for the service to complete (reaches idle or error state).
// This is useful for one-shot execution mode.
// Note: This method does not accept a context to maintain backward compatibility.
// For context-aware waiting, use GetStatus() in a loop with context checking.
func (s *Service) WaitForCompletion() {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		s.mu.RLock()
		state := s.state
		phase := s.phase
		s.mu.RUnlock()

		// Service is complete when it's no longer running
		if state != ServiceStateRunning && state != ServiceStateStopping {
			return
		}

		// Also check phase for completion
		if phase == ServicePhaseCompleted || phase == ServicePhaseError {
			return
		}
	}
}

// CacheStats holds aggregated cache statistics from all caches.
type CacheStats struct {
	Spotify        spotify.CacheStats
	SpotifyTTL     int // TTL in seconds
	AudioSearch    spotify.CacheStats
	AudioSearchTTL int // TTL in seconds
	FileExistence  map[string]interface{}
}

// GetCacheStats returns aggregated cache statistics from all caches.
func (s *Service) GetCacheStats() CacheStats {
	stats := CacheStats{}

	// Get Spotify cache stats
	if s.spotifyClient != nil {
		stats.Spotify = s.spotifyClient.GetCacheStats()
	}

	// Get Spotify cache TTL from config
	if s.config != nil {
		stats.SpotifyTTL = s.config.Download.CacheTTL
		if stats.SpotifyTTL == 0 {
			stats.SpotifyTTL = 3600 // Default
		}
		stats.AudioSearchTTL = s.config.Download.AudioSearchCacheTTL
		if stats.AudioSearchTTL == 0 {
			stats.AudioSearchTTL = 86400 // Default (24h)
		}
	}

	// Get audio search cache stats
	if s.audioProvider != nil {
		stats.AudioSearch = s.audioProvider.GetCacheStats()
	}

	// Get file existence cache stats
	if s.downloader != nil {
		stats.FileExistence = s.downloader.GetFileExistenceCacheStats()
	}

	return stats
}
