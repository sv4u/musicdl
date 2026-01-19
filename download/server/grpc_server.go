package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/sv4u/musicdl/download"
	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/logging"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/plan"
	"github.com/sv4u/musicdl/download/proto"
	"github.com/sv4u/musicdl/download/spotify"
)

// DownloadServiceServer implements the gRPC DownloadService.
type DownloadServiceServer struct {
	proto.UnimplementedDownloadServiceServer

	// Service management
	service     *download.Service
	serviceMu   sync.RWMutex
	serviceInit sync.Once

	// Configuration
	planPath string
	logPath  string
	version  string

	// Logger
	logger *logging.Logger

	// Dependencies (for service creation)
	spotifyClient    *spotify.SpotifyClient
	audioProvider    *audio.Provider
	metadataEmbedder *metadata.Embedder
}

// NewDownloadServiceServer creates a new gRPC download service server.
func NewDownloadServiceServer(planPath, logPath, version string) (*DownloadServiceServer, error) {
	// Create logger
	logger, err := logging.NewLogger(logPath, "download-service")
	if err != nil {
		return nil, fmt.Errorf("failed to create logger: %w", err)
	}

	return &DownloadServiceServer{
		planPath: planPath,
		logPath:  logPath,
		version:  version,
		logger:   logger,
	}, nil
}

// Close closes the server and cleans up resources.
func (s *DownloadServiceServer) Close() error {
	s.serviceMu.Lock()
	defer s.serviceMu.Unlock()

	if s.service != nil {
		// Stop service if running
		s.service.Stop()
	}

	if s.logger != nil {
		return s.logger.Close()
	}

	return nil
}

// getOrCreateService gets or creates the download service.
func (s *DownloadServiceServer) getOrCreateService(cfg *config.MusicDLConfig) (*download.Service, error) {
	var initErr error
	s.serviceInit.Do(func() {
		// Create Spotify client
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
			initErr = fmt.Errorf("failed to create Spotify client: %w", err)
			return
		}
		s.spotifyClient = spotifyClient

		// Create audio provider
		audioConfig := &audio.Config{
			OutputFormat:   cfg.Download.Format,
			Bitrate:        cfg.Download.Bitrate,
			AudioProviders: cfg.Download.AudioProviders,
			CacheMaxSize:   cfg.Download.AudioSearchCacheMaxSize,
			CacheTTL:       cfg.Download.AudioSearchCacheTTL,
		}
		audioProvider, err := audio.NewProvider(audioConfig)
		if err != nil {
			initErr = fmt.Errorf("failed to create audio provider: %w", err)
			return
		}
		s.audioProvider = audioProvider

		// Create metadata embedder
		metadataEmbedder := metadata.NewEmbedder()

		// Create download service
		service, err := download.NewService(cfg, spotifyClient, audioProvider, metadataEmbedder, s.planPath)
		if err != nil {
			initErr = fmt.Errorf("failed to create download service: %w", err)
			return
		}

		s.serviceMu.Lock()
		s.service = service
		s.serviceMu.Unlock()
	})

	if initErr != nil {
		return nil, initErr
	}

	s.serviceMu.RLock()
	defer s.serviceMu.RUnlock()
	return s.service, nil
}

// checkVersion checks client version compatibility.
func (s *DownloadServiceServer) checkVersion(clientVersion string) error {
	if err := checkVersionCompatibility(clientVersion, s.version); err != nil {
		s.logger.ErrorWithOperation("version_check", "Version mismatch detected", err)
		return status.Error(codes.FailedPrecondition, err.Error())
	}
	return nil
}

// StartDownload starts the download service with the provided configuration.
func (s *DownloadServiceServer) StartDownload(ctx context.Context, req *proto.StartDownloadRequest) (*proto.StartDownloadResponse, error) {
	// Check version compatibility
	if req.ClientVersion != nil {
		if err := s.checkVersion(req.ClientVersion.Version); err != nil {
			return &proto.StartDownloadResponse{
				Success:       false,
				ErrorMessage:  err.Error(),
				ServerVersion: &proto.VersionInfo{Version: s.version},
			}, nil
		}
	}

	s.logger.InfoWithOperation("StartDownload", "Starting download service")

	// Convert proto config to Go config
	cfg, err := convertProtoConfigToConfig(req.Config)
	if err != nil {
		s.logger.ErrorWithOperation("StartDownload", "Failed to convert config", err)
		return &proto.StartDownloadResponse{
			Success:       false,
			ErrorMessage:  fmt.Sprintf("failed to convert config: %v", err),
			ServerVersion: &proto.VersionInfo{Version: s.version},
		}, nil
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		s.logger.ErrorWithOperation("StartDownload", "Config validation failed", err)
		return &proto.StartDownloadResponse{
			Success:       false,
			ErrorMessage:  fmt.Sprintf("config validation failed: %v", err),
			ServerVersion: &proto.VersionInfo{Version: s.version},
		}, nil
	}

	// Set UI defaults
	cfg.UI.SetDefaults(req.PlanPath)

	// Get or create service
	svc, err := s.getOrCreateService(cfg)
	if err != nil {
		s.logger.ErrorWithOperation("StartDownload", "Failed to get or create service", err)
		return &proto.StartDownloadResponse{
			Success:       false,
			ErrorMessage:  fmt.Sprintf("failed to create service: %v", err),
			ServerVersion: &proto.VersionInfo{Version: s.version},
		}, nil
	}

	// Start download
	if err := svc.Start(ctx); err != nil {
		s.logger.ErrorWithOperation("StartDownload", "Failed to start download", err)
		return &proto.StartDownloadResponse{
			Success:       false,
			ErrorMessage:  fmt.Sprintf("failed to start download: %v", err),
			ServerVersion: &proto.VersionInfo{Version: s.version},
		}, nil
	}

	s.logger.InfoWithOperation("StartDownload", "Download service started successfully")
	return &proto.StartDownloadResponse{
		Success:       true,
		ServerVersion: &proto.VersionInfo{Version: s.version},
	}, nil
}

// StopDownload stops the download service.
func (s *DownloadServiceServer) StopDownload(ctx context.Context, req *proto.StopDownloadRequest) (*proto.StopDownloadResponse, error) {
	// Check version compatibility
	if req.ClientVersion != nil {
		if err := s.checkVersion(req.ClientVersion.Version); err != nil {
			return &proto.StopDownloadResponse{
				Success:       false,
				ErrorMessage:  err.Error(),
				ServerVersion: &proto.VersionInfo{Version: s.version},
			}, nil
		}
	}

	s.logger.InfoWithOperation("StopDownload", "Stopping download service")

	s.serviceMu.RLock()
	svc := s.service
	s.serviceMu.RUnlock()

	if svc == nil {
		return &proto.StopDownloadResponse{
			Success:       false,
			ErrorMessage:  "service is not initialized",
			ServerVersion: &proto.VersionInfo{Version: s.version},
		}, nil
	}

	if err := svc.Stop(); err != nil {
		s.logger.ErrorWithOperation("StopDownload", "Failed to stop download", err)
		return &proto.StopDownloadResponse{
			Success:       false,
			ErrorMessage:  fmt.Sprintf("failed to stop download: %v", err),
			ServerVersion: &proto.VersionInfo{Version: s.version},
		}, nil
	}

	s.logger.InfoWithOperation("StopDownload", "Download service stopped successfully")
	return &proto.StopDownloadResponse{
		Success:       true,
		ServerVersion: &proto.VersionInfo{Version: s.version},
	}, nil
}

// GetStatus returns the current status of the download service.
func (s *DownloadServiceServer) GetStatus(ctx context.Context, req *proto.GetStatusRequest) (*proto.GetStatusResponse, error) {
	// Check version compatibility
	if req.ClientVersion != nil {
		if err := s.checkVersion(req.ClientVersion.Version); err != nil {
			return nil, err
		}
	}

	s.serviceMu.RLock()
	svc := s.service
	s.serviceMu.RUnlock()

	resp := &proto.GetStatusResponse{
		ServerVersion: &proto.VersionInfo{Version: s.version},
	}

	if svc == nil {
		resp.State = proto.ServiceState_SERVICE_STATE_IDLE
		resp.Phase = proto.ServicePhase_SERVICE_PHASE_IDLE
		return resp, nil
	}

	// Get status from service
	statusMap := svc.GetStatus()

	// Convert state
	if stateStr, ok := statusMap["state"].(download.ServiceState); ok {
		resp.State = convertServiceStateToProto(stateStr)
	} else {
		resp.State = proto.ServiceState_SERVICE_STATE_IDLE
	}

	// Convert phase
	if phaseStr, ok := statusMap["phase"].(download.ServicePhase); ok {
		resp.Phase = convertServicePhaseToProto(phaseStr)
	} else {
		resp.Phase = proto.ServicePhase_SERVICE_PHASE_IDLE
	}

	// Error message
	if errMsg, ok := statusMap["error"].(string); ok {
		resp.ErrorMessage = errMsg
	}

	// Timestamps
	if startedAtStr, ok := statusMap["started_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, startedAtStr); err == nil {
			resp.StartedAt = t.Unix()
		}
	}
	if completedAtStr, ok := statusMap["completed_at"].(string); ok && completedAtStr != "" {
		if t, err := time.Parse(time.RFC3339, completedAtStr); err == nil {
			ts := t.Unix()
			resp.CompletedAt = &ts
		}
	}

	// Progress percentage
	if progress, ok := statusMap["progress_percentage"].(float64); ok {
		resp.ProgressPercentage = progress
	}

	// Plan statistics
	if planStats, ok := statusMap["plan_stats"].(map[string]interface{}); ok {
		if total, ok := planStats["total"].(int); ok {
			resp.TotalItems = int32(total)
		}
		if completed, ok := planStats["completed"].(int); ok {
			resp.CompletedItems = int32(completed)
		}
		if failed, ok := planStats["failed"].(int); ok {
			resp.FailedItems = int32(failed)
		}
		if pending, ok := planStats["pending"].(int); ok {
			resp.PendingItems = int32(pending)
		}
		if inProgress, ok := planStats["in_progress"].(int); ok {
			resp.InProgressItems = int32(inProgress)
		}
	}

	return resp, nil
}

// GetPlanItems returns plan items with optional filtering.
func (s *DownloadServiceServer) GetPlanItems(ctx context.Context, req *proto.GetPlanItemsRequest) (*proto.GetPlanItemsResponse, error) {
	// Check version compatibility
	if req.ClientVersion != nil {
		if err := s.checkVersion(req.ClientVersion.Version); err != nil {
			return nil, err
		}
	}

	s.serviceMu.RLock()
	svc := s.service
	s.serviceMu.RUnlock()

	resp := &proto.GetPlanItemsResponse{
		ServerVersion: &proto.VersionInfo{Version: s.version},
		Items:         []*proto.PlanItem{},
	}

	if svc == nil {
		return resp, nil
	}

	// Get plan
	plan := svc.GetPlan()
	if plan == nil {
		return resp, nil
	}

	// Filter items
	items := plan.Items
	if req.Filters != nil {
		items = s.filterPlanItems(items, req.Filters)
	}

	// Convert to proto
	for _, item := range items {
		resp.Items = append(resp.Items, convertPlanItemToProto(item))
	}

	return resp, nil
}

// filterPlanItems filters plan items based on the provided filters.
func (s *DownloadServiceServer) filterPlanItems(items []*plan.PlanItem, filters *proto.PlanItemFilters) []*plan.PlanItem {
	result := make([]*plan.PlanItem, 0)

	for _, item := range items {
		// Status filter
		if len(filters.Status) > 0 {
			status := item.GetStatus()
			statusMatch := false
			for _, filterStatus := range filters.Status {
				if convertPlanItemStatusToProto(status) == filterStatus {
					statusMatch = true
					break
				}
			}
			if !statusMatch {
				continue
			}
		}

		// Type filter
		if len(filters.Type) > 0 {
			typeMatch := false
			for _, filterType := range filters.Type {
				if convertPlanItemTypeToProto(item.ItemType) == filterType {
					typeMatch = true
					break
				}
			}
			if !typeMatch {
				continue
			}
		}

		// Search filter
		if filters.Search != "" {
			searchTerm := filters.Search
			if !contains(item.Name, searchTerm) &&
				!contains(item.SpotifyURL, searchTerm) &&
				!contains(item.YouTubeURL, searchTerm) &&
				!contains(item.ItemID, searchTerm) {
				continue
			}
		}

		result = append(result, item)
	}

	return result
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	// Simple case-insensitive contains
	sLower := toLower(s)
	substrLower := toLower(substr)
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// toLower converts a string to lowercase (simple implementation).
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}

// StreamLogs streams log entries to the client.
func (s *DownloadServiceServer) StreamLogs(req *proto.StreamLogsRequest, stream proto.DownloadService_StreamLogsServer) error {
	// Check version compatibility
	if req.ClientVersion != nil {
		if err := s.checkVersion(req.ClientVersion.Version); err != nil {
			return err
		}
	}

	s.logger.InfoWithOperation("StreamLogs", "Log streaming requested")

	// Get log path from server context
	ctx := stream.Context()
	logPath := s.logPath

	// Stream logs from file
	return streamLogsFromFile(ctx, logPath, req, stream)
}

// ValidateConfig validates the provided configuration.
func (s *DownloadServiceServer) ValidateConfig(ctx context.Context, req *proto.ValidateConfigRequest) (*proto.ValidateConfigResponse, error) {
	// Check version compatibility
	if req.ClientVersion != nil {
		if err := s.checkVersion(req.ClientVersion.Version); err != nil {
			return &proto.ValidateConfigResponse{
				Valid:         false,
				Errors:        []*proto.ValidationError{{Field: "version", Message: err.Error()}},
				ServerVersion: &proto.VersionInfo{Version: s.version},
			}, nil
		}
	}

	// Convert proto config to Go config
	cfg, err := convertProtoConfigToConfig(req.Config)
	if err != nil {
		return &proto.ValidateConfigResponse{
			Valid:         false,
			Errors:        []*proto.ValidationError{{Field: "config", Message: fmt.Sprintf("failed to convert config: %v", err)}},
			ServerVersion: &proto.VersionInfo{Version: s.version},
		}, nil
	}

	// Validate config
	validationErr := cfg.Validate()
	if validationErr != nil {
		return &proto.ValidateConfigResponse{
			Valid:         false,
			Errors:        []*proto.ValidationError{{Field: "config", Message: validationErr.Error()}},
			ServerVersion: &proto.VersionInfo{Version: s.version},
		}, nil
	}

	return &proto.ValidateConfigResponse{
		Valid:         true,
		ServerVersion: &proto.VersionInfo{Version: s.version},
	}, nil
}

// HealthCheck returns the health status of the service.
func (s *DownloadServiceServer) HealthCheck(ctx context.Context, req *proto.HealthCheckRequest) (*proto.HealthCheckResponse, error) {
	// Check version compatibility
	if req.ClientVersion != nil {
		if err := s.checkVersion(req.ClientVersion.Version); err != nil {
			return &proto.HealthCheckResponse{
				Liveness:      proto.HealthStatus_HEALTH_STATUS_UNHEALTHY,
				Readiness:     proto.ReadinessStatus_READINESS_STATUS_UNAVAILABLE,
				ServiceHealth: proto.HealthStatus_HEALTH_STATUS_UNHEALTHY,
				ServerVersion:  &proto.VersionInfo{Version: s.version},
			}, nil
		}
	}

	s.serviceMu.RLock()
	svc := s.service
	s.serviceMu.RUnlock()

	resp := &proto.HealthCheckResponse{
		Liveness:      proto.HealthStatus_HEALTH_STATUS_HEALTHY, // Server is always alive if responding
		Readiness:     proto.ReadinessStatus_READINESS_STATUS_READY,
		ServiceHealth: proto.HealthStatus_HEALTH_STATUS_HEALTHY,
		ServerVersion: &proto.VersionInfo{Version: s.version},
	}

	if svc == nil {
		resp.Readiness = proto.ReadinessStatus_READINESS_STATUS_READY // Can accept new downloads
		return resp, nil
	}

	// Check service state
	statusMap := svc.GetStatus()
	if stateStr, ok := statusMap["state"].(download.ServiceState); ok {
		if stateStr == download.ServiceStateError {
			resp.ServiceHealth = proto.HealthStatus_HEALTH_STATUS_UNHEALTHY
			resp.Readiness = proto.ReadinessStatus_READINESS_STATUS_NOT_READY
		} else if stateStr == download.ServiceStateStopping {
			resp.Readiness = proto.ReadinessStatus_READINESS_STATUS_NOT_READY
		}
	}

	return resp, nil
}
