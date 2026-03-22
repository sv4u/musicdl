package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/plex"
)

// PlexSyncTracker tracks the state of a Plex playlist sync operation.
type PlexSyncTracker struct {
	mu          sync.RWMutex
	isRunning   bool
	startedAt   time.Time
	completedAt time.Time
	progress    int
	total       int
	err         error
	results     []plex.SyncResult
	cancelFunc  context.CancelFunc
}

// plexBridgeLogger adapts the LogBroadcaster to the plex.SyncLogger interface
// and pushes real-time progress updates to the PlexSyncTracker.
type plexBridgeLogger struct {
	broadcaster *LogBroadcaster
	tracker     *PlexSyncTracker
}

func (l *plexBridgeLogger) Log(level, message string) {
	l.broadcaster.BroadcastString(level, message, "plex")
}

func (l *plexBridgeLogger) OnProgress(progress, total int, results []plex.SyncResult) {
	cp := make([]plex.SyncResult, len(results))
	copy(cp, results)
	l.tracker.mu.Lock()
	l.tracker.progress = progress
	l.tracker.total = total
	l.tracker.results = cp
	l.tracker.mu.Unlock()
}

// getStatus returns a snapshot of the current sync state.
func (t *PlexSyncTracker) getStatus() plex.SyncStatus {
	t.mu.RLock()
	defer t.mu.RUnlock()
	results := make([]plex.SyncResult, len(t.results))
	copy(results, t.results)
	s := plex.SyncStatus{
		IsRunning: t.isRunning,
		Progress:  t.progress,
		Total:     t.total,
		Results:   results,
	}
	if !t.startedAt.IsZero() {
		s.StartedAt = t.startedAt.Unix()
	}
	if !t.completedAt.IsZero() {
		s.CompletedAt = t.completedAt.Unix()
	}
	if t.err != nil {
		s.Error = t.err.Error()
	}
	if s.Results == nil {
		s.Results = []plex.SyncResult{}
	}
	return s
}

// plexSyncHandler handles POST /api/plex/sync — starts a Plex playlist sync.
func (s *APIServer) plexSyncHandler(w http.ResponseWriter, r *http.Request) {
	s.plexTracker.mu.Lock()
	if s.plexTracker.isRunning {
		s.plexTracker.mu.Unlock()
		http.Error(w, `{"error":"plex sync already running"}`, http.StatusConflict)
		return
	}
	configPath := os.Getenv("MUSICDL_WORK_DIR")
	if configPath == "" {
		configPath = "."
	}
	cfg, err := config.LoadConfig(configPath + "/config.yaml")
	if err != nil {
		s.plexTracker.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": fmt.Sprintf("failed to load config: %v", err)})
		return
	}
	if cfg.Plex.ServerURL == "" || cfg.Plex.Token == "" {
		s.plexTracker.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "plex.server_url and plex.token must be configured in config.yaml"})
		return
	}
	localPath := os.Getenv("MUSICDL_WORK_DIR")
	if localPath == "" {
		localPath = "."
	}
	syncCfg := plex.SyncConfig{
		ServerURL: cfg.Plex.ServerURL,
		Token:     cfg.Plex.Token,
		SectionID: cfg.Plex.SectionID,
		MusicPath: cfg.Plex.MusicPath,
		LocalPath: localPath,
	}
	ctx, cancel := context.WithCancel(context.Background())
	s.plexTracker.isRunning = true
	s.plexTracker.startedAt = time.Now()
	s.plexTracker.completedAt = time.Time{}
	s.plexTracker.progress = 0
	s.plexTracker.total = 0
	s.plexTracker.err = nil
	s.plexTracker.results = nil
	s.plexTracker.cancelFunc = cancel
	s.plexTracker.mu.Unlock()
	s.logBroadcaster.BroadcastString("info", "Starting Plex playlist sync", "plex")
	logger := &plexBridgeLogger{broadcaster: s.logBroadcaster, tracker: s.plexTracker}
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				s.plexTracker.mu.Lock()
				s.plexTracker.isRunning = false
				s.plexTracker.completedAt = time.Now()
				s.plexTracker.err = fmt.Errorf("plex sync panicked: %v", rec)
				s.plexTracker.mu.Unlock()
				s.logBroadcaster.BroadcastString("error", fmt.Sprintf("Plex sync panicked: %v", rec), "plex")
			}
		}()
		status, syncErr := plex.SyncPlaylists(ctx, syncCfg, logger)
		s.plexTracker.mu.Lock()
		s.plexTracker.isRunning = false
		s.plexTracker.completedAt = time.Now()
		if status != nil {
			s.plexTracker.progress = status.Progress
			s.plexTracker.total = status.Total
			s.plexTracker.results = status.Results
		}
		if syncErr != nil {
			s.plexTracker.err = syncErr
		}
		s.plexTracker.mu.Unlock()
	}()
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"status": "sync started"})
}

// plexStatusHandler handles GET /api/plex/status — returns current sync state.
func (s *APIServer) plexStatusHandler(w http.ResponseWriter, _ *http.Request) {
	status := s.plexTracker.getStatus()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// plexStopHandler handles POST /api/plex/stop — cancels a running sync.
func (s *APIServer) plexStopHandler(w http.ResponseWriter, _ *http.Request) {
	s.plexTracker.mu.RLock()
	running := s.plexTracker.isRunning
	cancel := s.plexTracker.cancelFunc
	s.plexTracker.mu.RUnlock()
	if !running {
		http.Error(w, `{"error":"no plex sync running"}`, http.StatusConflict)
		return
	}
	if cancel != nil {
		cancel()
	}
	s.logBroadcaster.BroadcastString("info", "Plex sync cancellation requested", "plex")
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "stop requested"})
}
