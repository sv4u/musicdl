package graph

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/sv4u/musicdl/download/plan"
)

// SyncPlan writes the full plan into the graph: artists, albums, playlists,
// tracks, M3U placeholders, and all relationships.
func (c *Client) SyncPlan(ctx context.Context, dp *plan.DownloadPlan, runID string) error {
	start := time.Now()

	if err := c.upsertRun(ctx, runID, start); err != nil {
		return fmt.Errorf("upsertRun: %w", err)
	}

	for _, item := range dp.Items {
		switch item.ItemType {
		case plan.PlanItemTypeArtist:
			if err := c.upsertArtist(ctx, item); err != nil {
				log.Printf("WARN: graph_sync artist %q: %v", item.Name, err)
			}
		case plan.PlanItemTypeAlbum:
			if err := c.upsertAlbum(ctx, item, dp); err != nil {
				log.Printf("WARN: graph_sync album %q: %v", item.Name, err)
			}
		case plan.PlanItemTypePlaylist:
			if err := c.upsertPlaylist(ctx, item); err != nil {
				log.Printf("WARN: graph_sync playlist %q: %v", item.Name, err)
			}
		case plan.PlanItemTypeTrack:
			if err := c.upsertTrack(ctx, item, dp, runID); err != nil {
				log.Printf("WARN: graph_sync track %q: %v", item.Name, err)
			}
		case plan.PlanItemTypeM3U:
			// M3U items are placeholders during plan; handled after download
		}
	}

	log.Printf("INFO: graph_sync plan synced in %v (%d items)", time.Since(start), len(dp.Items))
	return nil
}

// SyncTrackResult updates a single track node after download completes.
func (c *Client) SyncTrackResult(ctx context.Context, item *plan.PlanItem, runID string) error {
	return c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		params := map[string]any{
			"item_id":   item.ItemID,
			"status":    string(item.GetStatus()),
			"file_path": item.GetFilePath(),
			"error":     item.GetError(),
			"run_id":    runID,
		}

		_, err := tx.Run(ctx, `
			MATCH (t:Track {item_id: $item_id})
			SET t.status = $status,
			    t.file_path = $file_path,
			    t.error = $error,
			    t.updated_at = datetime()
			WITH t
			MATCH (r:DownloadRun {run_id: $run_id})
			MERGE (r)-[rel:PROCESSED]->(t)
			SET rel.status = $status, rel.error = $error
			RETURN t.item_id
		`, params)
		return nil, err
	})
}

// SyncM3U records an M3U file and its track references in the graph.
func (c *Client) SyncM3U(ctx context.Context, m3uItem *plan.PlanItem, tracks []*plan.PlanItem, runID string) error {
	m3uPath := m3uItem.GetFilePath()
	if m3uPath == "" {
		return nil
	}

	containerName := ""
	if v, ok := m3uItem.Metadata["playlist_name"].(string); ok {
		containerName = v
	} else if v, ok := m3uItem.Metadata["album_name"].(string); ok {
		containerName = v
	} else {
		containerName = m3uItem.Name
	}

	return c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `
			MERGE (m:M3UFile {file_path: $file_path})
			SET m.name = $name,
			    m.track_count = $track_count,
			    m.updated_at = datetime()
			ON CREATE SET m.created_at = datetime()
			WITH m
			OPTIONAL MATCH (r:DownloadRun {run_id: $run_id})
			FOREACH (_ IN CASE WHEN r IS NOT NULL THEN [1] ELSE [] END |
				MERGE (r)-[:GENERATED]->(m)
			)
			RETURN m.file_path
		`, map[string]any{
			"file_path":   m3uPath,
			"name":        containerName,
			"track_count": len(tracks),
			"run_id":      runID,
		})
		if err != nil {
			return nil, err
		}

		for i, track := range tracks {
			relPath := ""
			if track.FilePath != "" {
				if abs, absErr := filepath.Abs(track.FilePath); absErr == nil {
					if root, rootErr := filepath.Abs("."); rootErr == nil {
						if rel, relErr := filepath.Rel(root, abs); relErr == nil {
							relPath = rel
						}
					}
				}
			}
			_, err = tx.Run(ctx, `
				MATCH (m:M3UFile {file_path: $m3u_path})
				MATCH (t:Track {item_id: $item_id})
				MERGE (m)-[ref:REFERENCES]->(t)
				SET ref.position = $position, ref.relative_path = $rel_path
			`, map[string]any{
				"m3u_path": m3uPath,
				"item_id":  track.ItemID,
				"position": i + 1,
				"rel_path": relPath,
			})
			if err != nil {
				log.Printf("WARN: graph_sync m3u track ref %q: %v", track.Name, err)
			}
		}
		return nil, nil
	})
}

// SyncLibrary scans the music directory and updates Track nodes with actual
// file system state: existence, size, format.
func (c *Client) SyncLibrary(ctx context.Context, workDir string) error {
	start := time.Now()
	musicExts := map[string]bool{".mp3": true, ".flac": true, ".m4a": true, ".opus": true, ".ogg": true, ".wav": true}
	var fileCount int

	err := filepath.Walk(workDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if !musicExts[ext] {
			return nil
		}

		relPath, relErr := filepath.Rel(workDir, path)
		if relErr != nil {
			return nil
		}
		parts := strings.Split(relPath, string(filepath.Separator))

		artist := ""
		album := ""
		if len(parts) >= 2 {
			artist = parts[0]
		}
		if len(parts) >= 3 {
			album = parts[1]
		}

		syncErr := c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
			_, err := tx.Run(ctx, `
				MERGE (t:Track {file_path: $file_path})
				SET t.file_exists = true,
				    t.file_size = $size,
				    t.format = $format,
				    t.file_name = $file_name,
				    t.library_synced_at = datetime()
				WITH t
				FOREACH (_ IN CASE WHEN $artist <> "" THEN [1] ELSE [] END |
					MERGE (a:Artist {name: $artist})
					MERGE (a)-[:PERFORMED]->(t)
				)
				FOREACH (_ IN CASE WHEN $album <> "" AND $artist <> "" THEN [1] ELSE [] END |
					MERGE (al:Album {name: $album})
					MERGE (ar:Artist {name: $artist})
					MERGE (ar)-[:CREATED]->(al)
					MERGE (al)-[:CONTAINS]->(t)
				)
			`, map[string]any{
				"file_path": relPath,
				"size":      info.Size(),
				"format":    strings.TrimPrefix(ext, "."),
				"file_name": info.Name(),
				"artist":    artist,
				"album":     album,
			})
			return nil, err
		})
		if syncErr != nil {
			log.Printf("WARN: graph_library_sync %s: %v", relPath, syncErr)
		}
		fileCount++
		return nil
	})

	if err != nil {
		return err
	}

	// Mark tracks whose files no longer exist
	_ = c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `
			MATCH (t:Track)
			WHERE t.file_path IS NOT NULL AND t.file_exists = true AND t.library_synced_at < datetime($cutoff)
			SET t.file_exists = false
		`, map[string]any{"cutoff": start.Format(time.RFC3339)})
		return nil, err
	})

	log.Printf("INFO: graph_library_sync scanned %d files in %v", fileCount, time.Since(start))
	return nil
}

// SyncPlexPlaylist records a Plex playlist sync result on an existing M3UFile
// node. It matches by name against nodes already created by SyncM3U (which
// merges on file_path). If no M3UFile with this name exists yet, the update
// is skipped to avoid creating a duplicate node without file_path.
func (c *Client) SyncPlexPlaylist(ctx context.Context, playlistName, action, syncErr string, trackCount int) error {
	return c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `
			MATCH (m:M3UFile)
			WHERE m.name = $name
			SET m.plex_synced = true,
			    m.plex_action = $action,
			    m.plex_error = $error,
			    m.plex_track_count = $track_count,
			    m.plex_synced_at = datetime()
		`, map[string]any{
			"name":        playlistName,
			"action":      action,
			"error":       syncErr,
			"track_count": trackCount,
		})
		return nil, err
	})
}

// upsertRun creates or updates a DownloadRun node.
func (c *Client) upsertRun(ctx context.Context, runID string, startedAt time.Time) error {
	return c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `
			MERGE (r:DownloadRun {run_id: $run_id})
			SET r.started_at = datetime($started_at),
			    r.state = "running"
		`, map[string]any{
			"run_id":     runID,
			"started_at": startedAt.Format(time.RFC3339),
		})
		return nil, err
	})
}

// CompleteRun marks a DownloadRun as finished.
func (c *Client) CompleteRun(ctx context.Context, runID string, stats map[string]int) error {
	return c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		_, err := tx.Run(ctx, `
			MATCH (r:DownloadRun {run_id: $run_id})
			SET r.state = "completed",
			    r.completed_at = datetime(),
			    r.completed_count = $completed,
			    r.failed_count = $failed,
			    r.pending_count = $pending,
			    r.total_count = $total
		`, map[string]any{
			"run_id":    runID,
			"completed": stats["completed"],
			"failed":    stats["failed"],
			"pending":   stats["pending"],
			"total":     stats["total"],
		})
		return nil, err
	})
}

func (c *Client) upsertArtist(ctx context.Context, item *plan.PlanItem) error {
	return c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		spotifyID := item.SpotifyID
		if spotifyID == "" {
			spotifyID = item.ItemID
		}
		_, err := tx.Run(ctx, `
			MERGE (a:Artist {spotify_id: $spotify_id})
			SET a.name = $name,
			    a.spotify_url = $spotify_url,
			    a.source = $source,
			    a.updated_at = datetime()
		`, map[string]any{
			"spotify_id":  spotifyID,
			"name":        item.Name,
			"spotify_url": item.SpotifyURL,
			"source":      string(item.Source),
		})
		return nil, err
	})
}

func (c *Client) upsertAlbum(ctx context.Context, item *plan.PlanItem, dp *plan.DownloadPlan) error {
	return c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		spotifyID := item.SpotifyID
		if spotifyID == "" {
			spotifyID = item.ItemID
		}
		_, err := tx.Run(ctx, `
			MERGE (al:Album {spotify_id: $spotify_id})
			SET al.name = $name,
			    al.spotify_url = $spotify_url,
			    al.source = $source,
			    al.updated_at = datetime()
		`, map[string]any{
			"spotify_id":  spotifyID,
			"name":        item.Name,
			"spotify_url": item.SpotifyURL,
			"source":      string(item.Source),
		})
		if err != nil {
			return nil, err
		}

		// Link album to parent artist if one exists
		if item.ParentID != "" {
			parent := dp.GetItem(item.ParentID)
			if parent != nil && parent.ItemType == plan.PlanItemTypeArtist {
				parentSpotifyID := parent.SpotifyID
				if parentSpotifyID == "" {
					parentSpotifyID = parent.ItemID
				}
				_, err = tx.Run(ctx, `
					MATCH (ar:Artist {spotify_id: $artist_id})
					MATCH (al:Album {spotify_id: $album_id})
					MERGE (ar)-[:CREATED]->(al)
				`, map[string]any{
					"artist_id": parentSpotifyID,
					"album_id":  spotifyID,
				})
			}
		}
		return nil, err
	})
}

func (c *Client) upsertPlaylist(ctx context.Context, item *plan.PlanItem) error {
	return c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		spotifyID := item.SpotifyID
		if spotifyID == "" {
			spotifyID = item.ItemID
		}
		_, err := tx.Run(ctx, `
			MERGE (p:Playlist {spotify_id: $spotify_id})
			SET p.name = $name,
			    p.spotify_url = $spotify_url,
			    p.source = $source,
			    p.create_m3u = $create_m3u,
			    p.updated_at = datetime()
		`, map[string]any{
			"spotify_id":  spotifyID,
			"name":        item.Name,
			"spotify_url": item.SpotifyURL,
			"source":      string(item.Source),
			"create_m3u":  metaBool(item.Metadata, "create_m3u"),
		})
		return nil, err
	})
}

func (c *Client) upsertTrack(ctx context.Context, item *plan.PlanItem, dp *plan.DownloadPlan, runID string) error {
	return c.writeTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		params := map[string]any{
			"item_id":      item.ItemID,
			"name":         item.Name,
			"spotify_id":   item.SpotifyID,
			"spotify_url":  item.SpotifyURL,
			"youtube_url":  item.YouTubeURL,
			"source_url":   item.SourceURL,
			"source":       string(item.Source),
			"status":       string(item.Status),
			"file_path":    item.FilePath,
			"error":        item.Error,
			"duration_ms":  metaFloat(item.Metadata, "duration_ms"),
			"track_number": metaString(item.Metadata, "track_number"),
			"disc_number":  metaString(item.Metadata, "disc_number"),
			"run_id":       runID,
		}

		_, err := tx.Run(ctx, `
			MERGE (t:Track {item_id: $item_id})
			SET t.name = $name,
			    t.spotify_id = $spotify_id,
			    t.spotify_url = $spotify_url,
			    t.youtube_url = $youtube_url,
			    t.source_url = $source_url,
			    t.source = $source,
			    t.status = $status,
			    t.file_path = $file_path,
			    t.error = $error,
			    t.duration_ms = $duration_ms,
			    t.track_number = $track_number,
			    t.disc_number = $disc_number,
			    t.updated_at = datetime()
			WITH t
			MATCH (r:DownloadRun {run_id: $run_id})
			MERGE (r)-[rel:PROCESSED]->(t)
			SET rel.status = $status
			RETURN t.item_id
		`, params)
		if err != nil {
			return nil, err
		}

		// Link to parent (album or playlist) if the parent exists in the plan
		if item.ParentID != "" {
			parent := dp.GetItem(item.ParentID)
			if parent != nil {
				switch parent.ItemType {
				case plan.PlanItemTypeAlbum:
					parentSpotifyID := parent.SpotifyID
					if parentSpotifyID == "" {
						parentSpotifyID = parent.ItemID
					}
					_, err = tx.Run(ctx, `
						MATCH (al:Album {spotify_id: $album_id})
						MATCH (t:Track {item_id: $item_id})
						MERGE (al)-[c:CONTAINS]->(t)
						SET c.track_number = $track_number, c.disc_number = $disc_number
					`, map[string]any{
						"album_id":     parentSpotifyID,
						"item_id":      item.ItemID,
						"track_number": metaString(item.Metadata, "track_number"),
						"disc_number":  metaString(item.Metadata, "disc_number"),
					})
				case plan.PlanItemTypePlaylist:
					parentSpotifyID := parent.SpotifyID
					if parentSpotifyID == "" {
						parentSpotifyID = parent.ItemID
					}
					_, err = tx.Run(ctx, `
						MATCH (p:Playlist {spotify_id: $playlist_id})
						MATCH (t:Track {item_id: $item_id})
						MERGE (p)-[inc:INCLUDES]->(t)
					`, map[string]any{
						"playlist_id": parentSpotifyID,
						"item_id":     item.ItemID,
					})
				}
			}
		}

		// Link track to artist via metadata
		artistName := metaString(item.Metadata, "artist")
		if artistName != "" {
			_, _ = tx.Run(ctx, `
				MATCH (t:Track {item_id: $item_id})
				MERGE (a:Artist {name: $artist_name})
				MERGE (a)-[:PERFORMED]->(t)
			`, map[string]any{
				"item_id":     item.ItemID,
				"artist_name": artistName,
			})
		}

		return nil, err
	})
}

// metaString extracts a string from item metadata.
func metaString(meta map[string]interface{}, key string) string {
	if meta == nil {
		return ""
	}
	v, ok := meta[key]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if ok {
		return s
	}
	return fmt.Sprintf("%v", v)
}

// metaFloat extracts a float64 from item metadata.
func metaFloat(meta map[string]interface{}, key string) float64 {
	if meta == nil {
		return 0
	}
	switch v := meta[key].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}

// metaBool extracts a bool from item metadata.
func metaBool(meta map[string]interface{}, key string) bool {
	if meta == nil {
		return false
	}
	v, ok := meta[key].(bool)
	return ok && v
}
