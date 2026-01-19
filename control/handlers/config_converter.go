package handlers

import (
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/proto"
)

// convertConfigToProto converts config.MusicDLConfig to proto.Config.
func convertConfigToProto(cfg *config.MusicDLConfig) *proto.Config {
	if cfg == nil {
		return nil
	}

	pc := &proto.Config{
		Version: cfg.Version,
	}

	// Convert download settings
	pc.Download = &proto.DownloadSettings{
		ClientId:     cfg.Download.ClientID,
		ClientSecret: cfg.Download.ClientSecret,
		Threads:     int32(cfg.Download.Threads),
		MaxRetries:   int32(cfg.Download.MaxRetries),
		Format:       cfg.Download.Format,
		Bitrate:      cfg.Download.Bitrate,
		Output:       cfg.Download.Output,
		AudioProviders: cfg.Download.AudioProviders,
		Overwrite:    string(cfg.Download.Overwrite),
		CacheMaxSize: int32(cfg.Download.CacheMaxSize),
		CacheTtl:     int32(cfg.Download.CacheTTL),
		AudioSearchCacheMaxSize: int32(cfg.Download.AudioSearchCacheMaxSize),
		AudioSearchCacheTtl:     int32(cfg.Download.AudioSearchCacheTTL),
		FileExistenceCacheMaxSize: int32(cfg.Download.FileExistenceCacheMaxSize),
		FileExistenceCacheTtl:    int32(cfg.Download.FileExistenceCacheTTL),
		SpotifyMaxRetries:        int32(cfg.Download.SpotifyMaxRetries),
		SpotifyRetryBaseDelay:    cfg.Download.SpotifyRetryBaseDelay,
		SpotifyRetryMaxDelay:     cfg.Download.SpotifyRetryMaxDelay,
		SpotifyRateLimitEnabled:  cfg.Download.SpotifyRateLimitEnabled,
		SpotifyRateLimitRequests: int32(cfg.Download.SpotifyRateLimitRequests),
		SpotifyRateLimitWindow:   cfg.Download.SpotifyRateLimitWindow,
		DownloadRateLimitEnabled:  cfg.Download.DownloadRateLimitEnabled,
		DownloadRateLimitRequests: int32(cfg.Download.DownloadRateLimitRequests),
		DownloadRateLimitWindow:   cfg.Download.DownloadRateLimitWindow,
		PlanGenerationEnabled:      cfg.Download.PlanGenerationEnabled,
		PlanOptimizationEnabled:    cfg.Download.PlanOptimizationEnabled,
		PlanExecutionEnabled:       cfg.Download.PlanExecutionEnabled,
		PlanPersistenceEnabled:     cfg.Download.PlanPersistenceEnabled,
		PlanStatusReportingEnabled: cfg.Download.PlanStatusReportingEnabled,
	}

	// Handle optional bandwidth limit
	if cfg.Download.DownloadBandwidthLimit != nil {
		limit := int32(*cfg.Download.DownloadBandwidthLimit)
		pc.Download.DownloadBandwidthLimit = &limit
	}

	// Convert UI settings
	pc.Ui = &proto.UISettings{
		HistoryPath:      cfg.UI.HistoryPath,
		HistoryRetention: int32(cfg.UI.HistoryRetention),
		SnapshotInterval: int32(cfg.UI.SnapshotInterval),
		LogPath:          cfg.UI.LogPath,
	}

	// Convert music sources
	pc.Songs = convertMusicSourcesToProto(cfg.Songs)
	pc.Artists = convertMusicSourcesToProto(cfg.Artists)
	pc.Playlists = convertMusicSourcesToProto(cfg.Playlists)
	pc.Albums = convertMusicSourcesToProto(cfg.Albums)

	return pc
}

// convertMusicSourcesToProto converts config.MusicSource slice to proto.MusicSource slice.
func convertMusicSourcesToProto(sources []config.MusicSource) []*proto.MusicSource {
	result := make([]*proto.MusicSource, 0, len(sources))
	for _, s := range sources {
		result = append(result, &proto.MusicSource{
			Name:      s.Name,
			Url:       s.URL,
			CreateM3U: s.CreateM3U,
		})
	}
	return result
}
