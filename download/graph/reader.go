package graph

import (
	"context"
	"fmt"
	"strings"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// NodeResult represents a generic graph node returned from queries.
type NodeResult struct {
	Labels     []string       `json:"labels"`
	Properties map[string]any `json:"properties"`
}

// RelationshipResult represents a graph relationship returned from queries.
type RelationshipResult struct {
	Type       string         `json:"type"`
	From       string         `json:"from"`
	To         string         `json:"to"`
	Properties map[string]any `json:"properties,omitempty"`
}

// TraversalResult holds nodes and relationships from a traversal.
type TraversalResult struct {
	Origin        NodeResult           `json:"origin"`
	Related       []NodeResult         `json:"related"`
	Relationships []RelationshipResult `json:"relationships"`
}

// GraphStats holds aggregate statistics about the graph.
type GraphStats struct {
	Artists       int            `json:"artists"`
	Albums        int            `json:"albums"`
	Tracks        int            `json:"tracks"`
	Playlists     int            `json:"playlists"`
	M3UFiles      int            `json:"m3u_files"`
	DownloadRuns  int            `json:"download_runs"`
	Relationships int            `json:"relationships"`
	TracksByStatus map[string]int `json:"tracks_by_status"`
}

// M3UValidation holds the result of validating an M3U file against the graph.
type M3UValidation struct {
	M3UName       string   `json:"m3u_name"`
	M3UPath       string   `json:"m3u_path"`
	TotalTracks   int      `json:"total_tracks"`
	ValidTracks   int      `json:"valid_tracks"`
	MissingFiles  []string `json:"missing_files,omitempty"`
	FailedTracks  []string `json:"failed_tracks,omitempty"`
	OrphanedPaths []string `json:"orphaned_paths,omitempty"`
	IsValid       bool     `json:"is_valid"`
}

// DebugInfo holds diagnostic information for a specific entity.
type DebugInfo struct {
	Entity        NodeResult           `json:"entity"`
	Related       []NodeResult         `json:"related"`
	Relationships []RelationshipResult `json:"relationships"`
	Issues        []string             `json:"issues,omitempty"`
}

var allowedLabels = map[string]bool{
	"Track": true, "Artist": true, "Album": true,
	"Playlist": true, "M3UFile": true, "DownloadRun": true,
}

var allowedPropertyKeys = map[string]bool{
	"name": true, "item_id": true, "spotify_id": true,
	"file_path": true, "run_id": true, "spotify_url": true,
}

var allowedRelationships = map[string]bool{
	"CREATED": true, "PERFORMED": true, "CONTAINS": true,
	"INCLUDES": true, "REFERENCES": true, "PROCESSED": true,
	"GENERATED": true,
}

// validateIdentifier checks that a string is a safe Cypher identifier
// (letters, digits, underscores only) to prevent injection.
func validateIdentifier(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !((r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_') {
			return false
		}
	}
	return true
}

// Search performs full-text or substring search across all node types.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]NodeResult, error) {
	if limit <= 0 {
		limit = 20
	}

	result, err := c.readTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		// Use CONTAINS matching across all node types
		res, err := tx.Run(ctx, `
			CALL {
				MATCH (t:Track) WHERE toLower(t.name) CONTAINS toLower($query)
					OR toLower(t.file_path) CONTAINS toLower($query)
					OR toLower(t.spotify_url) CONTAINS toLower($query)
					OR toLower(t.error) CONTAINS toLower($query)
				RETURN t AS node, labels(t) AS lbls, 1 AS score
				UNION ALL
				MATCH (a:Artist) WHERE toLower(a.name) CONTAINS toLower($query)
				RETURN a AS node, labels(a) AS lbls, 2 AS score
				UNION ALL
				MATCH (al:Album) WHERE toLower(al.name) CONTAINS toLower($query)
				RETURN al AS node, labels(al) AS lbls, 3 AS score
				UNION ALL
				MATCH (p:Playlist) WHERE toLower(p.name) CONTAINS toLower($query)
				RETURN p AS node, labels(p) AS lbls, 4 AS score
				UNION ALL
				MATCH (m:M3UFile) WHERE toLower(m.name) CONTAINS toLower($query)
					OR toLower(m.file_path) CONTAINS toLower($query)
				RETURN m AS node, labels(m) AS lbls, 5 AS score
			}
			RETURN node, lbls
			LIMIT $limit
		`, map[string]any{"query": query, "limit": limit})
		if err != nil {
			return nil, err
		}

		var nodes []NodeResult
		for res.Next(ctx) {
			record := res.Record()
			nodeVal, _ := record.Get("node")
			lblsVal, _ := record.Get("lbls")

			n, ok := nodeVal.(neo4j.Node)
			if !ok {
				continue
			}
			lbls, _ := lblsVal.([]any)
			labels := make([]string, 0, len(lbls))
			for _, l := range lbls {
				if s, ok := l.(string); ok {
					labels = append(labels, s)
				}
			}
			nodes = append(nodes, NodeResult{
				Labels:     labels,
				Properties: n.Props,
			})
		}
		return nodes, res.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.([]NodeResult), nil
}

// Traverse follows relationships from a node identified by label + property.
func (c *Client) Traverse(ctx context.Context, label, key, value string, depth int) (*TraversalResult, error) {
	if !allowedLabels[label] || !validateIdentifier(label) {
		return nil, fmt.Errorf("invalid label %q: must be one of Track, Artist, Album, Playlist, M3UFile, DownloadRun", label)
	}
	if !allowedPropertyKeys[key] || !validateIdentifier(key) {
		return nil, fmt.Errorf("invalid property key %q: must be one of name, item_id, spotify_id, file_path, run_id, spotify_url", key)
	}
	if depth <= 0 {
		depth = 1
	}
	if depth > 3 {
		depth = 3
	}

	result, err := c.readTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		cypher := fmt.Sprintf(`
			MATCH (origin:%s {%s: $value})
			OPTIONAL MATCH path = (origin)-[r*1..%d]-(related)
			WITH origin, relationships(path) AS rels, nodes(path) AS pathNodes
			RETURN origin, rels, pathNodes
		`, label, key, depth)

		res, err := tx.Run(ctx, cypher, map[string]any{"value": value})
		if err != nil {
			return nil, err
		}

		tr := &TraversalResult{}
		seen := make(map[int64]bool)
		seenRels := make(map[string]bool)

		for res.Next(ctx) {
			record := res.Record()

			if tr.Origin.Properties == nil {
				originVal, _ := record.Get("origin")
				if n, ok := originVal.(neo4j.Node); ok {
					tr.Origin = NodeResult{Labels: n.Labels, Properties: n.Props}
					seen[n.Id] = true
				}
			}

			relsVal, _ := record.Get("rels")
			if rels, ok := relsVal.([]any); ok {
				for _, relRaw := range rels {
					if rel, ok := relRaw.(neo4j.Relationship); ok {
						relKey := fmt.Sprintf("%d-%d-%s", rel.StartId, rel.EndId, rel.Type)
						if seenRels[relKey] {
							continue
						}
						seenRels[relKey] = true
						tr.Relationships = append(tr.Relationships, RelationshipResult{
							Type:       rel.Type,
							From:       fmt.Sprintf("node/%d", rel.StartId),
							To:         fmt.Sprintf("node/%d", rel.EndId),
							Properties: rel.Props,
						})
					}
				}
			}

			nodesVal, _ := record.Get("pathNodes")
			if nodes, ok := nodesVal.([]any); ok {
				for _, nodeRaw := range nodes {
					if n, ok := nodeRaw.(neo4j.Node); ok {
						if seen[n.Id] {
							continue
						}
						seen[n.Id] = true
						tr.Related = append(tr.Related, NodeResult{
							Labels:     n.Labels,
							Properties: n.Props,
						})
					}
				}
			}
		}
		return tr, res.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.(*TraversalResult), nil
}

// Query executes a structured query against the graph. Supports filtering
// by node type, status, and relationship patterns.
func (c *Client) Query(ctx context.Context, nodeType, status, relationship, relatedTo string, limit int) ([]NodeResult, error) {
	if !allowedLabels[nodeType] || !validateIdentifier(nodeType) {
		return nil, fmt.Errorf("invalid node_type %q: must be one of Track, Artist, Album, Playlist, M3UFile, DownloadRun", nodeType)
	}
	if limit <= 0 {
		limit = 30
	}

	var conditions []string
	params := map[string]any{"limit": limit}

	matchClause := fmt.Sprintf("MATCH (n:%s)", nodeType)

	if status != "" {
		conditions = append(conditions, "n.status = $status")
		params["status"] = status
	}

	if relatedTo != "" && relationship != "" {
		if !allowedRelationships[relationship] || !validateIdentifier(relationship) {
			return nil, fmt.Errorf("invalid relationship %q: must be one of CREATED, PERFORMED, CONTAINS, INCLUDES, REFERENCES, PROCESSED, GENERATED", relationship)
		}
		matchClause = fmt.Sprintf("MATCH (n:%s)-[:%s]-(related)", nodeType, relationship)
		conditions = append(conditions, "toLower(related.name) CONTAINS toLower($related_to)")
		params["related_to"] = relatedTo
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	cypher := fmt.Sprintf("%s %s RETURN n, labels(n) AS lbls LIMIT $limit", matchClause, whereClause)

	result, err := c.readTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, cypher, params)
		if err != nil {
			return nil, err
		}

		var nodes []NodeResult
		for res.Next(ctx) {
			record := res.Record()
			nodeVal, _ := record.Get("n")
			lblsVal, _ := record.Get("lbls")

			n, ok := nodeVal.(neo4j.Node)
			if !ok {
				continue
			}
			lbls, _ := lblsVal.([]any)
			labels := make([]string, 0, len(lbls))
			for _, l := range lbls {
				if s, ok := l.(string); ok {
					labels = append(labels, s)
				}
			}
			nodes = append(nodes, NodeResult{Labels: labels, Properties: n.Props})
		}
		return nodes, res.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.([]NodeResult), nil
}

// Stats returns aggregate statistics about the graph.
// Each count runs as an independent subquery so a label with zero nodes
// cannot collapse the entire result to zero rows.
func (c *Client) Stats(ctx context.Context) (*GraphStats, error) {
	result, err := c.readTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, `
			CALL { MATCH (a:Artist) RETURN count(a) AS artists }
			CALL { MATCH (al:Album) RETURN count(al) AS albums }
			CALL { MATCH (t:Track) RETURN count(t) AS tracks }
			CALL { MATCH (p:Playlist) RETURN count(p) AS playlists }
			CALL { MATCH (m:M3UFile) RETURN count(m) AS m3us }
			CALL { MATCH (r:DownloadRun) RETURN count(r) AS runs }
			RETURN artists, albums, tracks, playlists, m3us, runs
		`, nil)
		if err != nil {
			return nil, err
		}

		stats := &GraphStats{TracksByStatus: make(map[string]int)}
		if res.Next(ctx) {
			record := res.Record()
			stats.Artists = intVal(record, "artists")
			stats.Albums = intVal(record, "albums")
			stats.Tracks = intVal(record, "tracks")
			stats.Playlists = intVal(record, "playlists")
			stats.M3UFiles = intVal(record, "m3us")
			stats.DownloadRuns = intVal(record, "runs")
		}

		// Track status breakdown
		statusRes, err := tx.Run(ctx, `
			MATCH (t:Track) WHERE t.status IS NOT NULL
			RETURN t.status AS status, count(t) AS cnt
		`, nil)
		if err == nil {
			for statusRes.Next(ctx) {
				record := statusRes.Record()
				s, _ := record.Get("status")
				cnt, _ := record.Get("cnt")
				if sStr, ok := s.(string); ok {
					if cInt, ok := cnt.(int64); ok {
						stats.TracksByStatus[sStr] = int(cInt)
					}
				}
			}
		}

		// Relationship count
		relRes, err := tx.Run(ctx, `MATCH ()-[r]->() RETURN count(r) AS total`, nil)
		if err == nil && relRes.Next(ctx) {
			stats.Relationships = intVal(relRes.Record(), "total")
		}

		return stats, nil
	})
	if err != nil {
		return nil, err
	}
	return result.(*GraphStats), nil
}

// ValidateM3U checks an M3U file's tracks against the graph for correctness.
// The query pins to a single M3UFile node (the best substring match) so
// results from unrelated M3U files cannot bleed into the validation.
func (c *Client) ValidateM3U(ctx context.Context, m3uName string) (*M3UValidation, error) {
	result, err := c.readTransaction(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		res, err := tx.Run(ctx, `
			MATCH (m:M3UFile)
			WHERE toLower(m.name) CONTAINS toLower($name) OR toLower(m.file_path) CONTAINS toLower($name)
			WITH m ORDER BY m.name LIMIT 1
			OPTIONAL MATCH (m)-[ref:REFERENCES]->(t:Track)
			RETURN m.name AS m3u_name, m.file_path AS m3u_path,
			       t.name AS track_name, t.file_path AS track_path,
			       t.status AS track_status, t.file_exists AS file_exists,
			       ref.relative_path AS ref_path
			ORDER BY ref.position
		`, map[string]any{"name": m3uName})
		if err != nil {
			return nil, err
		}

		v := &M3UValidation{IsValid: true}
		for res.Next(ctx) {
			record := res.Record()
			if v.M3UName == "" {
				nameVal, _ := record.Get("m3u_name")
				if s, ok := nameVal.(string); ok {
					v.M3UName = s
				}
				pathVal, _ := record.Get("m3u_path")
				if s, ok := pathVal.(string); ok {
					v.M3UPath = s
				}
			}

			trackNameVal, _ := record.Get("track_name")
			if trackNameVal == nil {
				continue
			}
			trackName, _ := trackNameVal.(string)

			v.TotalTracks++

			statusVal, _ := record.Get("track_status")
			status, _ := statusVal.(string)

			existsVal, _ := record.Get("file_exists")
			fileExists, hasFileExistsProp := existsVal.(bool)

			if status == "failed" {
				v.FailedTracks = append(v.FailedTracks, trackName)
				v.IsValid = false
			} else if hasFileExistsProp && !fileExists && status == "completed" {
				v.MissingFiles = append(v.MissingFiles, trackName)
				v.IsValid = false
			} else {
				v.ValidTracks++
			}
		}
		return v, res.Err()
	})
	if err != nil {
		return nil, err
	}
	return result.(*M3UValidation), nil
}

// Debug returns diagnostic info for a specific entity (track, playlist, etc.).
func (c *Client) Debug(ctx context.Context, query string) (*DebugInfo, error) {
	// Find the entity first
	nodes, err := c.Search(ctx, query, 1)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no entity found matching %q", query)
	}

	entity := nodes[0]
	info := &DebugInfo{Entity: entity}

	// Determine label and key for traversal
	label := ""
	key := ""
	value := ""
	if len(entity.Labels) > 0 {
		label = entity.Labels[0]
	}

	switch label {
	case "Track":
		key = "item_id"
		if v, ok := entity.Properties["item_id"].(string); ok {
			value = v
		}
	case "Artist":
		key = "name"
		if v, ok := entity.Properties["name"].(string); ok {
			value = v
		}
	case "Album":
		key = "spotify_id"
		if v, ok := entity.Properties["spotify_id"].(string); ok {
			value = v
		}
	case "Playlist":
		key = "spotify_id"
		if v, ok := entity.Properties["spotify_id"].(string); ok {
			value = v
		}
	case "M3UFile":
		key = "file_path"
		if v, ok := entity.Properties["file_path"].(string); ok {
			value = v
		}
	default:
		key = "name"
		if v, ok := entity.Properties["name"].(string); ok {
			value = v
		}
	}

	if value != "" {
		tr, trErr := c.Traverse(ctx, label, key, value, 2)
		if trErr == nil {
			info.Related = tr.Related
			info.Relationships = tr.Relationships
		}
	}

	// Generate diagnostic issues
	info.Issues = c.detectIssues(entity)

	return info, nil
}

// detectIssues analyzes a node and returns potential problems.
func (c *Client) detectIssues(node NodeResult) []string {
	var issues []string
	props := node.Properties

	if len(node.Labels) > 0 && node.Labels[0] == "Track" {
		if status, ok := props["status"].(string); ok && status == "failed" {
			errMsg, _ := props["error"].(string)
			issues = append(issues, fmt.Sprintf("Track download failed: %s", errMsg))
		}
		if _, ok := props["file_path"]; !ok {
			if status, ok := props["status"].(string); ok && status == "completed" {
				issues = append(issues, "Track marked completed but has no file_path")
			}
		}
		if exists, ok := props["file_exists"].(bool); ok && !exists {
			issues = append(issues, "File no longer exists on disk")
		}
		if _, ok := props["spotify_url"]; !ok {
			if _, ok := props["youtube_url"]; !ok {
				if _, ok := props["source_url"]; !ok {
					issues = append(issues, "Track has no source URL (Spotify, YouTube, or direct)")
				}
			}
		}
	}

	if len(node.Labels) > 0 && node.Labels[0] == "M3UFile" {
		if _, ok := props["plex_error"].(string); ok {
			issues = append(issues, fmt.Sprintf("Plex sync error: %s", props["plex_error"]))
		}
	}

	return issues
}

// intVal extracts an int from a Neo4j record value.
func intVal(record *neo4j.Record, key string) int {
	v, _ := record.Get(key)
	switch val := v.(type) {
	case int64:
		return int(val)
	case int:
		return val
	default:
		return 0
	}
}
