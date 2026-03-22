package main

import (
	"time"

	mcpserver "github.com/sv4u/musicdl/mcp"
)

// apiRuntimeProvider bridges live APIServer state to the MCP RuntimeDataProvider interface.
type apiRuntimeProvider struct {
	server *APIServer
}

func (a *apiRuntimeProvider) GetLiveDownloadStatus() *mcpserver.DownloadStatusInfo {
	a.server.currentRunTracker.mu.RLock()
	defer a.server.currentRunTracker.mu.RUnlock()

	errMsg := ""
	if a.server.currentRunTracker.err != nil {
		errMsg = a.server.currentRunTracker.err.Error()
	}

	var startedAt int64
	if !a.server.currentRunTracker.startedAt.IsZero() {
		startedAt = a.server.currentRunTracker.startedAt.Unix()
	}
	return &mcpserver.DownloadStatusInfo{
		IsRunning:     a.server.currentRunTracker.isRunning,
		OperationType: a.server.currentRunTracker.operationType,
		Progress:      a.server.currentRunTracker.progress,
		Total:         a.server.currentRunTracker.total,
		StartedAt:     startedAt,
		ErrorMsg:      errMsg,
	}
}

func (a *apiRuntimeProvider) GetLiveRecentLogs() []mcpserver.LogEntryInfo {
	msgs := a.server.logBroadcaster.GetHistory()
	if len(msgs) == 0 {
		return nil
	}
	entries := make([]mcpserver.LogEntryInfo, 0, len(msgs))
	for _, m := range msgs {
		entries = append(entries, mcpserver.LogEntryInfo{
			Timestamp: time.Unix(m.Timestamp, 0),
			Message:   m.Message,
			Level:     m.Level,
			Service:   m.Source,
		})
	}
	return entries
}

func (a *apiRuntimeProvider) GetLiveRateLimitInfo() *mcpserver.RateLimitInfo {
	a.server.spotifyClientMu.RLock()
	client := a.server.spotifyClient
	a.server.spotifyClientMu.RUnlock()

	if client == nil {
		return nil
	}
	info := client.GetRateLimitInfo()
	if info == nil {
		return nil
	}
	return &mcpserver.RateLimitInfo{
		Active:              true,
		RetryAfterSeconds:   int64(info.RetryAfterSeconds),
		RetryAfterTimestamp: info.RetryAfterTimestamp,
		RemainingSeconds:    max(info.RetryAfterTimestamp-time.Now().Unix(), 0),
	}
}

func (a *apiRuntimeProvider) GetLivePlexSyncStatus() *mcpserver.PlexSyncStatusInfo {
	status := a.server.plexTracker.getStatus()
	results := make([]mcpserver.PlexSyncResultInfo, len(status.Results))
	for i, r := range status.Results {
		results[i] = mcpserver.PlexSyncResultInfo{
			PlaylistName: r.PlaylistName,
			Action:       r.Action,
			Error:        r.Error,
			TrackCount:   r.TrackCount,
		}
	}
	return &mcpserver.PlexSyncStatusInfo{
		IsRunning:   status.IsRunning,
		StartedAt:   status.StartedAt,
		CompletedAt: status.CompletedAt,
		Progress:    status.Progress,
		Total:       status.Total,
		Error:       status.Error,
		Results:     results,
	}
}

