package plan

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	ID        string   `json:"id,omitempty"` // Stable ItemID (e.g. playlist:xyz, playlist:youtube:abc) for parent resolution when loading M3U items
	Name      string   `json:"name"`
	SourceURL string   `json:"source_url"`
	CreateM3U bool     `json:"create_m3u"`
	TrackIDs  []string `json:"track_ids"`
}

// SpecM3UItem is the spec JSON shape for an M3U playlist file (playlist M3U only; album M3U is not round-tripped).
type SpecM3UItem struct {
	ID           string `json:"id"`
	ParentID     string `json:"parent_id"`
	Name         string `json:"name"`
	PlaylistName string `json:"playlist_name"`
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
	M3Us            []SpecM3UItem       `json:"m3us,omitempty"`
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
		M3Us:        make([]SpecM3UItem, 0),
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
			trackIDs := make([]string, 0, len(item.ChildIDs))
			for _, childID := range item.ChildIDs {
				child := plan.GetItem(childID)
				if child != nil && child.ItemType == PlanItemTypeTrack {
					trackIDs = append(trackIDs, childID)
				}
			}
			spec.Playlists = append(spec.Playlists, SpecPlaylist{
				ID:        item.ItemID,
				Name:      item.Name,
				SourceURL: item.SpotifyURL,
				TrackIDs:  trackIDs,
			})
			if item.Metadata != nil {
				if createM3U, ok := item.Metadata["create_m3u"].(bool); ok && createM3U {
					spec.Playlists[len(spec.Playlists)-1].CreateM3U = true
				}
			}
		}
		if item.ItemType == PlanItemTypeM3U && item.ParentID != "" && strings.HasPrefix(item.ParentID, "playlist:") {
			playlistName := ""
			if item.Metadata != nil {
				if n, ok := item.Metadata["playlist_name"].(string); ok {
					playlistName = n
				}
			}
			spec.M3Us = append(spec.M3Us, SpecM3UItem{
				ID:           item.ItemID,
				ParentID:     item.ParentID,
				Name:         item.Name,
				PlaylistName: playlistName,
			})
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
		playlistItemID := "playlist:" + p.Name
		if p.ID != "" {
			playlistItemID = p.ID
		}
		item := &PlanItem{
			ItemID:     playlistItemID,
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

	for _, m := range spec.M3Us {
		metadata := make(map[string]interface{})
		if m.PlaylistName != "" {
			metadata["playlist_name"] = m.PlaylistName
		}
		item := &PlanItem{
			ItemID:     m.ID,
			ItemType:   PlanItemTypeM3U,
			ParentID:   m.ParentID,
			Name:       m.Name,
			Status:     PlanItemStatusPending,
			Metadata:   metadata,
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
