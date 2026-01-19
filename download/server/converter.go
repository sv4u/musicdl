package server

import (
	"fmt"

	"github.com/sv4u/musicdl/download"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/plan"
	"github.com/sv4u/musicdl/download/proto"
)

// convertProtoConfigToConfig converts a proto Config to config.MusicDLConfig.
func convertProtoConfigToConfig(pc *proto.Config) (*config.MusicDLConfig, error) {
	if pc == nil {
		return nil, fmt.Errorf("proto config is nil")
	}

	cfg := &config.MusicDLConfig{
		Version: pc.Version,
	}

	// Convert download settings
	if pc.Download != nil {
		cfg.Download = config.DownloadSettings{
			ClientID:     pc.Download.ClientId,
			ClientSecret: pc.Download.ClientSecret,
			Threads:      int(pc.Download.Threads),
			MaxRetries:   int(pc.Download.MaxRetries),
			Format:       pc.Download.Format,
			Bitrate:      pc.Download.Bitrate,
			Output:       pc.Download.Output,
			Overwrite:    config.OverwriteMode(pc.Download.Overwrite),
		}

		// Audio providers
		cfg.Download.AudioProviders = pc.Download.AudioProviders

		// Cache settings
		cfg.Download.CacheMaxSize = int(pc.Download.CacheMaxSize)
		cfg.Download.CacheTTL = int(pc.Download.CacheTtl)
		cfg.Download.AudioSearchCacheMaxSize = int(pc.Download.AudioSearchCacheMaxSize)
		cfg.Download.AudioSearchCacheTTL = int(pc.Download.AudioSearchCacheTtl)
		cfg.Download.FileExistenceCacheMaxSize = int(pc.Download.FileExistenceCacheMaxSize)
		cfg.Download.FileExistenceCacheTTL = int(pc.Download.FileExistenceCacheTtl)

		// Spotify rate limiting
		cfg.Download.SpotifyMaxRetries = int(pc.Download.SpotifyMaxRetries)
		cfg.Download.SpotifyRetryBaseDelay = pc.Download.SpotifyRetryBaseDelay
		cfg.Download.SpotifyRetryMaxDelay = pc.Download.SpotifyRetryMaxDelay
		cfg.Download.SpotifyRateLimitEnabled = pc.Download.SpotifyRateLimitEnabled
		cfg.Download.SpotifyRateLimitRequests = int(pc.Download.SpotifyRateLimitRequests)
		cfg.Download.SpotifyRateLimitWindow = pc.Download.SpotifyRateLimitWindow

		// Download rate limiting
		cfg.Download.DownloadRateLimitEnabled = pc.Download.DownloadRateLimitEnabled
		cfg.Download.DownloadRateLimitRequests = int(pc.Download.DownloadRateLimitRequests)
		cfg.Download.DownloadRateLimitWindow = pc.Download.DownloadRateLimitWindow
		if pc.Download.DownloadBandwidthLimit != nil {
			limit := int(*pc.Download.DownloadBandwidthLimit)
			cfg.Download.DownloadBandwidthLimit = &limit
		}

		// Plan architecture flags
		cfg.Download.PlanGenerationEnabled = pc.Download.PlanGenerationEnabled
		cfg.Download.PlanOptimizationEnabled = pc.Download.PlanOptimizationEnabled
		cfg.Download.PlanExecutionEnabled = pc.Download.PlanExecutionEnabled
		cfg.Download.PlanPersistenceEnabled = pc.Download.PlanPersistenceEnabled
		cfg.Download.PlanStatusReportingEnabled = pc.Download.PlanStatusReportingEnabled
	}

	// Convert UI settings
	if pc.Ui != nil {
		cfg.UI = config.UISettings{
			HistoryPath:      pc.Ui.HistoryPath,
			HistoryRetention: int(pc.Ui.HistoryRetention),
			SnapshotInterval: int(pc.Ui.SnapshotInterval),
			LogPath:          pc.Ui.LogPath,
		}
	}

	// Convert music sources
	cfg.Songs = convertProtoMusicSources(pc.Songs)
	cfg.Artists = convertProtoMusicSources(pc.Artists)
	cfg.Playlists = convertProtoMusicSources(pc.Playlists)
	cfg.Albums = convertProtoMusicSources(pc.Albums)

	return cfg, nil
}

// convertProtoMusicSources converts proto MusicSource slice to config.MusicSource slice.
func convertProtoMusicSources(pms []*proto.MusicSource) []config.MusicSource {
	result := make([]config.MusicSource, 0, len(pms))
	for _, ms := range pms {
		if ms != nil {
			result = append(result, config.MusicSource{
				Name:      ms.Name,
				URL:       ms.Url,
				CreateM3U: ms.CreateM3U,
			})
		}
	}
	return result
}

// convertPlanItemToProto converts a plan.PlanItem to proto.PlanItem.
func convertPlanItemToProto(item *plan.PlanItem) *proto.PlanItem {
	if item == nil {
		return nil
	}

	// Get thread-safe values
	status := item.GetStatus()
	progress := item.GetProgress()
	errorMsg := item.GetError()
	filePath := item.GetFilePath()
	createdAt, startedAt, completedAt := item.GetTimestamps()
	metadata := item.GetMetadata()

	pi := &proto.PlanItem{
		ItemId:     item.ItemID,
		ItemType:   convertPlanItemTypeToProto(item.ItemType),
		SpotifyId:  item.SpotifyID,
		SpotifyUrl: item.SpotifyURL,
		YoutubeUrl: item.YouTubeURL,
		ParentId:   item.ParentID,
		ChildIds:   item.ChildIDs,
		Name:       item.Name,
		Status:     convertPlanItemStatusToProto(status),
		Error:      errorMsg,
		FilePath:   filePath,
		CreatedAt:  createdAt.Unix(),
		Progress:   progress,
	}

	// Convert timestamps
	if startedAt != nil {
		ts := startedAt.Unix()
		pi.StartedAt = &ts
	}
	if completedAt != nil {
		ts := completedAt.Unix()
		pi.CompletedAt = &ts
	}

	// Convert metadata (simplified: all values as strings)
	pi.Metadata = make(map[string]string)
	for k, v := range metadata {
		pi.Metadata[k] = fmt.Sprintf("%v", v)
	}

	return pi
}

// convertPlanItemTypeToProto converts plan.PlanItemType to proto.PlanItemType.
func convertPlanItemTypeToProto(t plan.PlanItemType) proto.PlanItemType {
	switch t {
	case plan.PlanItemTypeTrack:
		return proto.PlanItemType_PLAN_ITEM_TYPE_TRACK
	case plan.PlanItemTypeAlbum:
		return proto.PlanItemType_PLAN_ITEM_TYPE_ALBUM
	case plan.PlanItemTypeArtist:
		return proto.PlanItemType_PLAN_ITEM_TYPE_ARTIST
	case plan.PlanItemTypePlaylist:
		return proto.PlanItemType_PLAN_ITEM_TYPE_PLAYLIST
	case plan.PlanItemTypeM3U:
		return proto.PlanItemType_PLAN_ITEM_TYPE_M3U
	default:
		return proto.PlanItemType_PLAN_ITEM_TYPE_UNSPECIFIED
	}
}

// convertPlanItemStatusToProto converts plan.PlanItemStatus to proto.PlanItemStatus.
func convertPlanItemStatusToProto(s plan.PlanItemStatus) proto.PlanItemStatus {
	switch s {
	case plan.PlanItemStatusPending:
		return proto.PlanItemStatus_PLAN_ITEM_STATUS_PENDING
	case plan.PlanItemStatusInProgress:
		return proto.PlanItemStatus_PLAN_ITEM_STATUS_IN_PROGRESS
	case plan.PlanItemStatusCompleted:
		return proto.PlanItemStatus_PLAN_ITEM_STATUS_COMPLETED
	case plan.PlanItemStatusFailed:
		return proto.PlanItemStatus_PLAN_ITEM_STATUS_FAILED
	case plan.PlanItemStatusSkipped:
		return proto.PlanItemStatus_PLAN_ITEM_STATUS_SKIPPED
	default:
		return proto.PlanItemStatus_PLAN_ITEM_STATUS_UNSPECIFIED
	}
}

// convertServiceStateToProto converts download.ServiceState to proto.ServiceState.
func convertServiceStateToProto(s download.ServiceState) proto.ServiceState {
	switch s {
	case download.ServiceStateIdle:
		return proto.ServiceState_SERVICE_STATE_IDLE
	case download.ServiceStateRunning:
		return proto.ServiceState_SERVICE_STATE_RUNNING
	case download.ServiceStateStopping:
		return proto.ServiceState_SERVICE_STATE_STOPPING
	case download.ServiceStateError:
		return proto.ServiceState_SERVICE_STATE_ERROR
	default:
		return proto.ServiceState_SERVICE_STATE_UNSPECIFIED
	}
}

// convertServicePhaseToProto converts download.ServicePhase to proto.ServicePhase.
func convertServicePhaseToProto(p download.ServicePhase) proto.ServicePhase {
	switch p {
	case download.ServicePhaseIdle:
		return proto.ServicePhase_SERVICE_PHASE_IDLE
	case download.ServicePhaseGenerating:
		return proto.ServicePhase_SERVICE_PHASE_GENERATING
	case download.ServicePhaseOptimizing:
		return proto.ServicePhase_SERVICE_PHASE_OPTIMIZING
	case download.ServicePhaseExecuting:
		return proto.ServicePhase_SERVICE_PHASE_EXECUTING
	case download.ServicePhaseCompleted:
		return proto.ServicePhase_SERVICE_PHASE_COMPLETED
	case download.ServicePhaseError:
		return proto.ServicePhase_SERVICE_PHASE_ERROR
	default:
		return proto.ServicePhase_SERVICE_PHASE_UNSPECIFIED
	}
}

// convertLogLevelToProto converts logging.LogLevel to proto.LogLevel.
func convertLogLevelToProto(level string) proto.LogLevel {
	switch level {
	case "DEBUG":
		return proto.LogLevel_LOG_LEVEL_DEBUG
	case "INFO":
		return proto.LogLevel_LOG_LEVEL_INFO
	case "WARN":
		return proto.LogLevel_LOG_LEVEL_WARN
	case "ERROR":
		return proto.LogLevel_LOG_LEVEL_ERROR
	default:
		return proto.LogLevel_LOG_LEVEL_UNSPECIFIED
	}
}

// checkVersionCompatibility checks if client and server versions match.
// Returns an error if versions don't match.
func checkVersionCompatibility(clientVersion, serverVersion string) error {
	if clientVersion == "" || serverVersion == "" {
		return fmt.Errorf("version cannot be empty")
	}
	if clientVersion != serverVersion {
		return fmt.Errorf("version mismatch: client version %s, server version %s. Please update to matching versions", clientVersion, serverVersion)
	}
	return nil
}
