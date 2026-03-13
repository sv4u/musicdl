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
	Name            string                 `json:"name,omitempty"`
	YouTubeURL      string                 `json:"youtube_url"`
	SpotifyURI      string                 `json:"spotify_uri,omitempty"`
	SpotifyMetadata map[string]interface{} `json:"spotify_metadata,omitempty"`
	YouTubeMetadata map[string]interface{} `json:"youtube_metadata,omitempty"`
	OutputPath      string                 `json:"output_path"`
	Status          string                 `json:"status"`
	Error           string                 `json:"error,omitempty"`
	RawOutput       string                 `json:"raw_output,omitempty"`
	SourceContext   map[string]interface{} `json:"source_context,omitempty"`
	TrackMetadata   map[string]interface{} `json:"track_metadata,omitempty"`
}

// SpecContainer is the spec JSON shape for album/artist container items.
type SpecContainer struct {
	ID         string                 `json:"id"`
	Type       string                 `json:"type"`
	Name       string                 `json:"name"`
	SpotifyURI string                 `json:"spotify_uri,omitempty"`
	ParentID   string                 `json:"parent_id,omitempty"`
	ChildIDs   []string               `json:"child_ids,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
}

// SpecPlaylist is the spec JSON shape for a playlist definition.
type SpecPlaylist struct {
	ID        string   `json:"id,omitempty"` // Stable ItemID (e.g. playlist:xyz, playlist:youtube:abc) for parent resolution when loading M3U items
	Name      string   `json:"name"`
	SourceURL string   `json:"source_url"`
	CreateM3U bool     `json:"create_m3u"`
	TrackIDs  []string `json:"track_ids"`
}

// SpecM3UItem is the spec JSON shape for an M3U playlist file (playlist or album).
type SpecM3UItem struct {
	ID           string `json:"id"`
	ParentID     string `json:"parent_id"`
	Name         string `json:"name"`
	PlaylistName string `json:"playlist_name,omitempty"`
	AlbumName    string `json:"album_name,omitempty"`
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
	M3Us            []SpecM3UItem      `json:"m3us,omitempty"`
	Containers      []SpecContainer    `json:"containers,omitempty"`
}

// metadataToSerializable converts a metadata map to a JSON-serializable map.
// Struct values (e.g. *YouTubeVideoMetadata) are converted to map[string]interface{}
// via JSON round-trip so they survive serialization.
func metadataToSerializable(m map[string]interface{}) map[string]interface{} {
	if len(m) == 0 {
		return nil
	}
	data, err := json.Marshal(m)
	if err != nil {
		return m
	}
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return m
	}
	return result
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
		Containers:  make([]SpecContainer, 0),
	}

	trackCount := 0
	for _, item := range plan.Items {
		switch item.ItemType {
		case PlanItemTypeTrack:
			trackCount++
			di := SpecDownloadItem{
				ID:         item.ItemID,
				Name:       item.Name,
				YouTubeURL: item.YouTubeURL,
				OutputPath: item.FilePath,
				Status:     string(item.Status),
				Error:      item.Error,
				RawOutput:  item.RawOutput,
			}
			if item.SpotifyURL != "" {
				di.SpotifyURI = item.SpotifyURL
			} else if item.SpotifyID != "" {
				di.SpotifyURI = "spotify:track:" + item.SpotifyID
			}
			di.TrackMetadata = metadataToSerializable(item.Metadata)
			spec.Downloads = append(spec.Downloads, di)

		case PlanItemTypePlaylist:
			trackIDs := make([]string, 0, len(item.ChildIDs))
			for _, childID := range item.ChildIDs {
				child := plan.GetItem(childID)
				if child != nil && child.ItemType == PlanItemTypeTrack {
					trackIDs = append(trackIDs, childID)
				}
			}
			sourceURL := item.SpotifyURL
			if sourceURL == "" {
				sourceURL = item.YouTubeURL
			}
			sp := SpecPlaylist{
				ID:        item.ItemID,
				Name:      item.Name,
				SourceURL: sourceURL,
				TrackIDs:  trackIDs,
			}
			if item.Metadata != nil {
				if createM3U, ok := item.Metadata["create_m3u"].(bool); ok && createM3U {
					sp.CreateM3U = true
				}
			}
			spec.Playlists = append(spec.Playlists, sp)

		case PlanItemTypeAlbum, PlanItemTypeArtist:
			sourceURL := item.SpotifyURL
			if sourceURL == "" {
				sourceURL = item.YouTubeURL
			}
			sc := SpecContainer{
				ID:       item.ItemID,
				Type:     string(item.ItemType),
				Name:     item.Name,
				ParentID: item.ParentID,
				ChildIDs: append([]string{}, item.ChildIDs...),
				Metadata: metadataToSerializable(item.Metadata),
			}
			if sourceURL != "" {
				sc.SpotifyURI = sourceURL
			}
			spec.Containers = append(spec.Containers, sc)

		case PlanItemTypeM3U:
			if item.ParentID == "" {
				continue
			}
			playlistName := ""
			albumName := ""
			if item.Metadata != nil {
				if n, ok := item.Metadata["playlist_name"].(string); ok {
					playlistName = n
				}
				if n, ok := item.Metadata["album_name"].(string); ok {
					albumName = n
				}
			}
			spec.M3Us = append(spec.M3Us, SpecM3UItem{
				ID:           item.ItemID,
				ParentID:     item.ParentID,
				Name:         item.Name,
				PlaylistName: playlistName,
				AlbumName:    albumName,
			})
		}
	}
	spec.TotalTracks = trackCount

	return spec
}

// parseSpecStatus normalizes a spec status string to a valid PlanItemStatus.
func parseSpecStatus(s string) PlanItemStatus {
	status := PlanItemStatus(s)
	switch status {
	case PlanItemStatusPending, PlanItemStatusSkipped, PlanItemStatusCompleted,
		PlanItemStatusFailed, PlanItemStatusInProgress:
		return status
	default:
		if s == "metadata_only" {
			return PlanItemStatusSkipped
		}
		return PlanItemStatusPending
	}
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
		status := parseSpecStatus(d.Status)

		// Prefer TrackMetadata (full metadata map) over legacy SpotifyMetadata/YouTubeMetadata
		metadata := make(map[string]interface{})
		if len(d.TrackMetadata) > 0 {
			for k, v := range d.TrackMetadata {
				metadata[k] = v
			}
		} else {
			if len(d.SpotifyMetadata) > 0 {
				metadata["spotify_metadata"] = d.SpotifyMetadata
			}
			if len(d.YouTubeMetadata) > 0 {
				metadata["youtube_metadata"] = d.YouTubeMetadata
			}
			if len(d.SourceContext) > 0 {
				metadata["source_context"] = d.SourceContext
			}
		}
		item := &PlanItem{
			ItemID:     d.ID,
			ItemType:   PlanItemTypeTrack,
			Name:       d.Name,
			YouTubeURL: d.YouTubeURL,
			FilePath:   d.OutputPath,
			Status:     status,
			Error:      d.Error,
			RawOutput:  d.RawOutput,
			Metadata:   metadata,
			CreatedAt:  time.Now(),
			Progress:   0,
		}
		if d.SpotifyURI != "" {
			item.SpotifyURL = d.SpotifyURI
		}
		plan.AddItem(item)
	}

	// Restore container items (albums, artists) before playlists so
	// parent references resolve correctly.
	for _, c := range spec.Containers {
		itemType := PlanItemType(c.Type)
		if itemType != PlanItemTypeAlbum && itemType != PlanItemTypeArtist {
			itemType = PlanItemTypeAlbum
		}
		item := &PlanItem{
			ItemID:    c.ID,
			ItemType:  itemType,
			Name:      c.Name,
			ParentID:  c.ParentID,
			ChildIDs:  append([]string{}, c.ChildIDs...),
			Status:    PlanItemStatusPending,
			Metadata:  c.Metadata,
			CreatedAt: time.Now(),
			Progress:  0,
		}
		if c.SpotifyURI != "" {
			item.SpotifyURL = c.SpotifyURI
		}
		plan.AddItem(item)
	}

	for _, p := range spec.Playlists {
		playlistItemID := "playlist:" + p.Name
		if p.ID != "" {
			playlistItemID = p.ID
		}
		item := &PlanItem{
			ItemID:    playlistItemID,
			ItemType:  PlanItemTypePlaylist,
			Name:      p.Name,
			ChildIDs:  append([]string{}, p.TrackIDs...),
			Status:    PlanItemStatusPending,
			Metadata:  map[string]interface{}{"create_m3u": p.CreateM3U},
			CreatedAt: time.Now(),
			Progress:  0,
		}
		if strings.Contains(playlistItemID, "youtube") {
			item.YouTubeURL = p.SourceURL
		} else {
			item.SpotifyURL = p.SourceURL
		}
		plan.AddItem(item)
	}

	for _, m := range spec.M3Us {
		metadata := make(map[string]interface{})
		if m.PlaylistName != "" {
			metadata["playlist_name"] = m.PlaylistName
		}
		if m.AlbumName != "" {
			metadata["album_name"] = m.AlbumName
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
		// Add M3U item to parent's ChildIDs so container status tracking is complete
		if m.ParentID != "" {
			parent := plan.GetItem(m.ParentID)
			if parent != nil {
				parent.ChildIDs = append(parent.ChildIDs, m.ID)
			}
		}
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
