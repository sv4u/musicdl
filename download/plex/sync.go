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

// FindM3UFiles finds all .m3u files under dir, deduplicates by filename
// (preferring root-level files over subdirectory files), and returns sorted
// absolute paths.
//
// musicdl creates M3U playlist files at the working directory root. Previous
// versions placed them inside artist/album subdirectories. This function finds
// all M3U files recursively but deduplicates so that a root-level file always
// wins over a subdirectory file with the same base name.
func FindM3UFiles(dir string) ([]string, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("plex: failed to resolve directory %s: %w", dir, err)
	}

	// Collect all M3U files with their depth relative to dir.
	type m3uEntry struct {
		path  string
		depth int
	}
	byName := make(map[string]m3uEntry)

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if info.IsDir() || !strings.EqualFold(filepath.Ext(path), ".m3u") {
			return nil
		}
		abs, absErr := filepath.Abs(path)
		if absErr != nil {
			return absErr
		}
		rel, relErr := filepath.Rel(absDir, abs)
		if relErr != nil {
			rel = abs
		}
		depth := strings.Count(rel, string(filepath.Separator))
		baseName := strings.ToLower(filepath.Base(abs))

		existing, exists := byName[baseName]
		if !exists || depth < existing.depth {
			byName[baseName] = m3uEntry{path: abs, depth: depth}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("plex: failed to walk directory %s: %w", dir, err)
	}

	files := make([]string, 0, len(byName))
	for _, entry := range byName {
		files = append(files, entry.path)
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
		titleKey := strings.ToLower(name)
		existing, exists := existingByTitle[titleKey]
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
			delete(existingByTitle, titleKey)
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
		if newPlaylist := client.FindPlaylistByTitle(name); newPlaylist != nil {
			result.TrackCount = newPlaylist.LeafCount
			existingByTitle[titleKey] = newPlaylist
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
