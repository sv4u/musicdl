package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// ErrPlanHashMismatch is returned when the plan file's config_hash does not match the expected hash.
var ErrPlanHashMismatch = errors.New("plan file config_hash does not match configuration")

// ErrPlanNotFound is returned when the plan file does not exist.
var ErrPlanNotFound = errors.New("plan file not found")

// SpecDownloadItem is the spec JSON shape for a single download item.
type SpecDownloadItem struct {
	ID              string                 `json:"id"`
	YouTubeURL      string                 `json:"youtube_url"`
	SpotifyURI      string                 `json:"spotify_uri,omitempty"`
	SpotifyMetadata map[string]interface{} `json:"spotify_metadata,omitempty"`
	YouTubeMetadata map[string]interface{} `json:"youtube_metadata,omitempty"`
	OutputPath      string                 `json:"output_path"`
	Status          string                 `json:"status"`
	SourceContext   map[string]interface{} `json:"source_context,omitempty"`
}

// SpecPlaylist is the spec JSON shape for a playlist definition.
type SpecPlaylist struct {
	Name      string   `json:"name"`
	SourceURL string   `json:"source_url"`
	CreateM3U bool     `json:"create_m3u"`
	TrackIDs  []string `json:"track_ids"`
}

// SpecPlan is the spec JSON shape for the full plan file.
type SpecPlan struct {
	ConfigHash      string             `json:"config_hash"`
	ConfigFile      string             `json:"config_file"`
	GeneratedAt     string             `json:"generated_at"`
	TotalTracks     int                `json:"total_tracks"`
	EstimatedSizeMB *float64           `json:"estimated_size_mb,omitempty"`
	Downloads       []SpecDownloadItem `json:"downloads"`
	Playlists       []SpecPlaylist     `json:"playlists"`
}

// PlanToSpec converts a DownloadPlan to the spec JSON shape.
// configHash and configFile are written into the spec; generatedAt is used for generated_at.
func PlanToSpec(plan *DownloadPlan, configHash, configFile string, generatedAt time.Time) *SpecPlan {
	spec := &SpecPlan{
		ConfigHash:  configHash,
		ConfigFile:  configFile,
		GeneratedAt: generatedAt.UTC().Format(time.RFC3339),
		Downloads:   make([]SpecDownloadItem, 0),
		Playlists:   make([]SpecPlaylist, 0),
	}

	trackCount := 0
	for _, item := range plan.Items {
		if item.ItemType == PlanItemTypeTrack {
			trackCount++
			di := SpecDownloadItem{
				ID:         item.ItemID,
				YouTubeURL: item.YouTubeURL,
				OutputPath: item.FilePath,
				Status:     string(item.Status),
			}
			if item.SpotifyURL != "" {
				di.SpotifyURI = item.SpotifyURL
			} else if item.SpotifyID != "" {
				di.SpotifyURI = "spotify:track:" + item.SpotifyID
			}
			if item.Metadata != nil {
				if m, ok := item.Metadata["spotify_metadata"].(map[string]interface{}); ok {
					di.SpotifyMetadata = m
				}
				if m, ok := item.Metadata["youtube_metadata"].(map[string]interface{}); ok {
					di.YouTubeMetadata = m
				}
				if m, ok := item.Metadata["source_context"].(map[string]interface{}); ok {
					di.SourceContext = m
				}
			}
			spec.Downloads = append(spec.Downloads, di)
		}
		if item.ItemType == PlanItemTypePlaylist {
			spec.Playlists = append(spec.Playlists, SpecPlaylist{
				Name:      item.Name,
				SourceURL: item.SpotifyURL,
				TrackIDs:  append([]string{}, item.ChildIDs...),
			})
			if item.Metadata != nil {
				if createM3U, ok := item.Metadata["create_m3u"].(bool); ok && createM3U {
					spec.Playlists[len(spec.Playlists)-1].CreateM3U = true
				}
			}
		}
	}
	spec.TotalTracks = trackCount

	return spec
}

// SpecToPlan converts a SpecPlan to a DownloadPlan for use by the executor.
func SpecToPlan(spec *SpecPlan) (*DownloadPlan, error) {
	if spec == nil {
		return nil, fmt.Errorf("spec plan is nil")
	}
	plan := NewDownloadPlan(map[string]interface{}{
		"config_hash":  spec.ConfigHash,
		"config_file":  spec.ConfigFile,
		"generated_at": spec.GeneratedAt,
		"total_tracks": spec.TotalTracks,
	})

	for _, d := range spec.Downloads {
		status := PlanItemStatus(d.Status)
		switch status {
		case PlanItemStatusPending, PlanItemStatusSkipped, PlanItemStatusCompleted,
			PlanItemStatusFailed, PlanItemStatusInProgress:
			// valid
		default:
			if d.Status == "metadata_only" {
				status = PlanItemStatusSkipped
			} else {
				status = PlanItemStatusPending
			}
		}
		metadata := make(map[string]interface{})
		if len(d.SpotifyMetadata) > 0 {
			metadata["spotify_metadata"] = d.SpotifyMetadata
		}
		if len(d.YouTubeMetadata) > 0 {
			metadata["youtube_metadata"] = d.YouTubeMetadata
		}
		if len(d.SourceContext) > 0 {
			metadata["source_context"] = d.SourceContext
		}
		item := &PlanItem{
			ItemID:     d.ID,
			ItemType:   PlanItemTypeTrack,
			YouTubeURL: d.YouTubeURL,
			FilePath:   d.OutputPath,
			Status:     status,
			Metadata:   metadata,
			CreatedAt:  time.Now(),
			Progress:   0,
		}
		if d.SpotifyURI != "" {
			item.SpotifyURL = d.SpotifyURI
		}
		plan.AddItem(item)
	}

	for _, p := range spec.Playlists {
		item := &PlanItem{
			ItemID:     "playlist:" + p.Name,
			ItemType:   PlanItemTypePlaylist,
			Name:       p.Name,
			SpotifyURL: p.SourceURL,
			ChildIDs:   append([]string{}, p.TrackIDs...),
			Status:     PlanItemStatusPending,
			Metadata:   map[string]interface{}{"create_m3u": p.CreateM3U},
			CreatedAt:  time.Now(),
			Progress:   0,
		}
		plan.AddItem(item)
	}

	return plan, nil
}

// SaveSpecPlan writes a SpecPlan to the given path.
func SaveSpecPlan(spec *SpecPlan, path string) error {
	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadSpecPlan reads a SpecPlan from the given path.
func LoadSpecPlan(path string) (*SpecPlan, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrPlanNotFound
		}
		return nil, err
	}
	var spec SpecPlan
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return &spec, nil
}

// LoadPlanByHash loads the plan file for the given cache dir and config hash,
// validates that plan.config_hash matches, and returns the plan as *DownloadPlan.
func LoadPlanByHash(cacheDir, configHash string) (*DownloadPlan, error) {
	path := GetPlanFilePath(cacheDir, configHash)
	spec, err := LoadSpecPlan(path)
	if err != nil {
		return nil, err
	}
	if spec.ConfigHash != configHash {
		return nil, ErrPlanHashMismatch
	}
	return SpecToPlan(spec)
}

// SavePlanByHash saves the plan to the cache dir using the given config hash.
// Creates cacheDir if it does not exist.
func SavePlanByHash(plan *DownloadPlan, cacheDir, configHash, configFile string) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}
	path := GetPlanFilePath(cacheDir, configHash)
	dir := filepath.Dir(path)
	if dir != cacheDir {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	spec := PlanToSpec(plan, configHash, configFile, time.Now().UTC())
	return SaveSpecPlan(spec, path)
}
