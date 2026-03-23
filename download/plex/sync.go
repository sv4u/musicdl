package plex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SyncLogger defines the interface for logging and progress reporting during sync.
type SyncLogger interface {
	Log(level, message string)
	OnProgress(progress, total int, results []SyncResult)
}

// SyncConfig holds the configuration for a Plex playlist sync run.
type SyncConfig struct {
	ServerURL string
	Token     string
	SectionID string // empty = auto-detect
	MusicPath string // how Plex sees the library (e.g. "/data/Music")
	LocalPath string // how musicdl sees the library (e.g. "/download")
}

// SyncStatus tracks the state and results of a playlist sync operation.
type SyncStatus struct {
	IsRunning   bool         `json:"isRunning"`
	StartedAt   int64        `json:"startedAt,omitempty"`
	CompletedAt int64        `json:"completedAt,omitempty"`
	Progress    int          `json:"progress"`
	Total       int          `json:"total"`
	Error       string       `json:"error,omitempty"`
	Results     []SyncResult `json:"results"`
}

// FindM3UFiles recursively finds all .m3u files under dir, returning sorted absolute paths.
func FindM3UFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.EqualFold(filepath.Ext(path), ".m3u") {
			abs, absErr := filepath.Abs(path)
			if absErr != nil {
				return absErr
			}
			files = append(files, abs)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("plex: failed to walk directory %s: %w", dir, err)
	}
	sort.Strings(files)
	return files, nil
}

// TranslatePath converts a local file path to the corresponding Plex-side path
// by stripping localBase and prepending plexBase.
func TranslatePath(localPath, localBase, plexBase string) string {
	rel := strings.TrimPrefix(localPath, localBase)
	return filepath.Join(plexBase, rel)
}

// PlaylistNameFromM3U extracts a playlist name from an M3U file path
// by returning the base filename without the .m3u extension.
func PlaylistNameFromM3U(m3uPath string) string {
	base := filepath.Base(m3uPath)
	ext := filepath.Ext(base)
	return strings.TrimSuffix(base, ext)
}

// SyncPlaylists synchronizes local M3U playlist files to a Plex server.
// It creates or updates audio playlists and returns aggregate status.
func SyncPlaylists(ctx context.Context, cfg SyncConfig, logger SyncLogger) (*SyncStatus, error) {
	status := &SyncStatus{
		IsRunning: true,
		StartedAt: time.Now().Unix(),
		Results:   []SyncResult{},
	}
	client := NewClient(cfg.ServerURL, cfg.Token)
	logger.Log("info", fmt.Sprintf("Connecting to Plex server at %s", cfg.ServerURL))
	if err := client.TestConnection(); err != nil {
		status.IsRunning = false
		status.CompletedAt = time.Now().Unix()
		status.Error = err.Error()
		return status, err
	}
	sectionID := cfg.SectionID
	if sectionID == "" {
		detected, err := client.FindMusicSectionID()
		if err != nil {
			status.IsRunning = false
			status.CompletedAt = time.Now().Unix()
			status.Error = err.Error()
			return status, err
		}
		sectionID = detected
	}
	logger.Log("info", fmt.Sprintf("Using music library section ID: %s", sectionID))
	m3uFiles, err := FindM3UFiles(cfg.LocalPath)
	if err != nil {
		status.IsRunning = false
		status.CompletedAt = time.Now().Unix()
		status.Error = err.Error()
		return status, err
	}
	logger.Log("info", fmt.Sprintf("Found %d playlist files", len(m3uFiles)))
	status.Total = len(m3uFiles)
	logger.OnProgress(status.Progress, status.Total, status.Results)
	existingPlaylists, err := client.GetPlaylists()
	if err != nil {
		status.IsRunning = false
		status.CompletedAt = time.Now().Unix()
		status.Error = err.Error()
		return status, err
	}
	existingByTitle := make(map[string]*Playlist)
	for i := range existingPlaylists {
		p := &existingPlaylists[i]
		if p.PlaylistType == "audio" {
			existingByTitle[strings.ToLower(p.Title)] = p
		}
	}
	var created, updated, failed int
	for _, m3uPath := range m3uFiles {
		if err := ctx.Err(); err != nil {
			status.IsRunning = false
			status.CompletedAt = time.Now().Unix()
			status.Error = "sync cancelled"
			return status, err
		}
		name := PlaylistNameFromM3U(m3uPath)
		plexPath := TranslatePath(m3uPath, cfg.LocalPath, cfg.MusicPath)
		result := SyncResult{
			PlaylistName: name,
			M3UPath:      m3uPath,
		}
		existing, exists := existingByTitle[strings.ToLower(name)]
		if exists {
			logger.Log("info", fmt.Sprintf("Updating playlist: %s", name))
			if err := client.DeletePlaylist(existing.RatingKey); err != nil {
				logger.Log("error", fmt.Sprintf("Failed to delete old playlist %s: %v", name, err))
				result.Action = "failed"
				result.Error = err.Error()
				failed++
				status.Results = append(status.Results, result)
				status.Progress++
				logger.OnProgress(status.Progress, status.Total, status.Results)
				continue
			}
			if err := client.UploadPlaylist(sectionID, plexPath); err != nil {
				logger.Log("error", fmt.Sprintf("Failed to upload playlist %s: %v", name, err))
				result.Action = "failed"
				result.Error = err.Error()
				failed++
				status.Results = append(status.Results, result)
				status.Progress++
				logger.OnProgress(status.Progress, status.Total, status.Results)
				continue
			}
			result.Action = "updated"
			updated++
		} else {
			logger.Log("info", fmt.Sprintf("Creating playlist: %s", name))
			if err := client.UploadPlaylist(sectionID, plexPath); err != nil {
				logger.Log("error", fmt.Sprintf("Failed to create playlist %s: %v", name, err))
				result.Action = "failed"
				result.Error = err.Error()
				failed++
				status.Results = append(status.Results, result)
				status.Progress++
				logger.OnProgress(status.Progress, status.Total, status.Results)
				continue
			}
			result.Action = "created"
			created++
		}
		// Fetch the actual track count from the newly created/updated playlist
		if newPlaylist := client.FindPlaylistByTitle(name); newPlaylist != nil {
			result.TrackCount = newPlaylist.LeafCount
		}
		logger.Log("info", fmt.Sprintf("Playlist %s: %s (tracks: %d)", name, result.Action, result.TrackCount))
		status.Results = append(status.Results, result)
		status.Progress++
		logger.OnProgress(status.Progress, status.Total, status.Results)
	}
	status.IsRunning = false
	status.CompletedAt = time.Now().Unix()
	logger.Log("info", fmt.Sprintf("Plex sync complete: %d created, %d updated, %d failed", created, updated, failed))
	return status, nil
}
