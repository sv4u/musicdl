package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sv4u/musicdl/download/graph"
)

type graphSearchParams struct {
	Query string `json:"query" jsonschema:"search text matched against names, file paths, URLs, and errors across all entity types"`
	Limit int    `json:"limit,omitempty" jsonschema:"max results to return (default 20)"`
}

type graphTraverseParams struct {
	Label string `json:"label" jsonschema:"node label: Track, Artist, Album, Playlist, M3UFile, DownloadRun"`
	Key   string `json:"key" jsonschema:"property name to match on (e.g. name, item_id, spotify_id, file_path)"`
	Value string `json:"value" jsonschema:"property value to match"`
	Depth int    `json:"depth,omitempty" jsonschema:"traversal depth 1-3 (default 1)"`
}

type graphQueryParams struct {
	NodeType     string `json:"node_type" jsonschema:"node label: Track, Artist, Album, Playlist, M3UFile, DownloadRun"`
	Status       string `json:"status,omitempty" jsonschema:"filter by status (pending, completed, failed, skipped)"`
	Relationship string `json:"relationship,omitempty" jsonschema:"relationship type to filter on (CONTAINS, INCLUDES, PERFORMED, CREATED, REFERENCES, PROCESSED, GENERATED)"`
	RelatedTo    string `json:"related_to,omitempty" jsonschema:"name of related entity to filter by (used with relationship)"`
	Limit        int    `json:"limit,omitempty" jsonschema:"max results (default 30)"`
}

type graphStatsParams struct{}

type graphM3UValidateParams struct {
	Name string `json:"name" jsonschema:"M3U file name or path to validate"`
}

type graphDebugParams struct {
	Query string `json:"query" jsonschema:"name, path, or ID of entity to debug"`
}

type graphLibrarySyncParams struct{}

func registerGraphTools(server *mcp.Server, graphClient *graph.Client, workDir string) {
	if graphClient == nil {
		return
	}

	mcp.AddTool(server, &mcp.Tool{
		Name:        "graph_search",
		Description: "Search the music knowledge graph across all entity types (tracks, artists, albums, playlists, M3U files). Matches against names, file paths, URLs, and error messages.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params graphSearchParams) (*mcp.CallToolResult, any, error) {
		q := strings.TrimSpace(params.Query)
		if q == "" {
			return nil, nil, fmt.Errorf("query is required")
		}
		nodes, err := graphClient.Search(ctx, q, params.Limit)
		if err != nil {
			return nil, nil, err
		}
		return formatNodeResults(nodes), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "graph_traverse",
		Description: "Traverse relationships from a specific node in the music knowledge graph. Start from any entity and follow connections to discover related tracks, albums, artists, playlists, and M3U files.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params graphTraverseParams) (*mcp.CallToolResult, any, error) {
		if params.Label == "" || params.Key == "" || params.Value == "" {
			return nil, nil, fmt.Errorf("label, key, and value are required")
		}
		result, err := graphClient.Traverse(ctx, params.Label, params.Key, params.Value, params.Depth)
		if err != nil {
			return nil, nil, err
		}
		return formatTraversalResult(result), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "graph_query",
		Description: "Query the music knowledge graph with structured filters. Find entities by type, status, and relationship patterns. Examples: failed tracks in a playlist, all albums by an artist, tracks referenced by an M3U file.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params graphQueryParams) (*mcp.CallToolResult, any, error) {
		if params.NodeType == "" {
			return nil, nil, fmt.Errorf("node_type is required")
		}
		nodes, err := graphClient.Query(ctx, params.NodeType, params.Status, params.Relationship, params.RelatedTo, params.Limit)
		if err != nil {
			return nil, nil, err
		}
		return formatNodeResults(nodes), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "graph_stats",
		Description: "Get aggregate statistics about the music knowledge graph: entity counts, track status breakdown, and relationship totals.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ graphStatsParams) (*mcp.CallToolResult, any, error) {
		stats, err := graphClient.Stats(ctx)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Artists: %d\n", stats.Artists))
		b.WriteString(fmt.Sprintf("Albums: %d\n", stats.Albums))
		b.WriteString(fmt.Sprintf("Tracks: %d\n", stats.Tracks))
		b.WriteString(fmt.Sprintf("Playlists: %d\n", stats.Playlists))
		b.WriteString(fmt.Sprintf("M3U Files: %d\n", stats.M3UFiles))
		b.WriteString(fmt.Sprintf("Download Runs: %d\n", stats.DownloadRuns))
		b.WriteString(fmt.Sprintf("Relationships: %d\n", stats.Relationships))
		if len(stats.TracksByStatus) > 0 {
			b.WriteString("\nTracks by status:\n")
			for status, count := range stats.TracksByStatus {
				b.WriteString(fmt.Sprintf("  %s: %d\n", status, count))
			}
		}
		text := strings.TrimRight(b.String(), "\n")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "graph_m3u_validate",
		Description: "Validate an M3U playlist file against the knowledge graph. Checks for missing files, failed tracks, and orphaned references to help build correct M3U files.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params graphM3UValidateParams) (*mcp.CallToolResult, any, error) {
		if params.Name == "" {
			return nil, nil, fmt.Errorf("name is required")
		}
		v, err := graphClient.ValidateM3U(ctx, params.Name)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("M3U: %s (%s)\n", v.M3UName, v.M3UPath))
		b.WriteString(fmt.Sprintf("Valid: %v\n", v.IsValid))
		b.WriteString(fmt.Sprintf("Total tracks: %d\n", v.TotalTracks))
		b.WriteString(fmt.Sprintf("Valid tracks: %d\n", v.ValidTracks))
		if len(v.MissingFiles) > 0 {
			b.WriteString(fmt.Sprintf("\nMissing files (%d):\n", len(v.MissingFiles)))
			for _, f := range v.MissingFiles {
				b.WriteString(fmt.Sprintf("  - %s\n", f))
			}
		}
		if len(v.FailedTracks) > 0 {
			b.WriteString(fmt.Sprintf("\nFailed tracks (%d):\n", len(v.FailedTracks)))
			for _, f := range v.FailedTracks {
				b.WriteString(fmt.Sprintf("  - %s\n", f))
			}
		}
		text := strings.TrimRight(b.String(), "\n")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "graph_debug",
		Description: "Debug a specific entity in the music knowledge graph. Returns the entity's properties, all relationships, and detected issues (missing files, download failures, broken references).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, params graphDebugParams) (*mcp.CallToolResult, any, error) {
		if params.Query == "" {
			return nil, nil, fmt.Errorf("query is required")
		}
		info, err := graphClient.Debug(ctx, params.Query)
		if err != nil {
			return nil, nil, err
		}
		return formatDebugInfo(info), nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "graph_library_sync",
		Description: "Scan the music library directory and sync file state into the knowledge graph. Updates file existence, sizes, and infers artist/album relationships from directory structure.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ graphLibrarySyncParams) (*mcp.CallToolResult, any, error) {
		err := graphClient.SyncLibrary(ctx, workDir)
		if err != nil {
			return nil, nil, err
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "Library sync complete."}}}, nil, nil
	})
}

func formatNodeResults(nodes []graph.NodeResult) *mcp.CallToolResult {
	if len(nodes) == 0 {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "No results found."}}}
	}
	var b strings.Builder
	b.WriteString(fmt.Sprintf("%d result(s):\n\n", len(nodes)))
	for i, n := range nodes {
		b.WriteString(fmt.Sprintf("[%d] %s\n", i+1, strings.Join(n.Labels, ":")))
		data, _ := json.MarshalIndent(n.Properties, "  ", "  ")
		b.WriteString(fmt.Sprintf("  %s\n\n", string(data)))
	}
	text := strings.TrimRight(b.String(), "\n")
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func formatTraversalResult(tr *graph.TraversalResult) *mcp.CallToolResult {
	var b strings.Builder
	b.WriteString("Origin: ")
	b.WriteString(strings.Join(tr.Origin.Labels, ":"))
	b.WriteString("\n")
	data, _ := json.MarshalIndent(tr.Origin.Properties, "  ", "  ")
	b.WriteString(fmt.Sprintf("  %s\n\n", string(data)))

	if len(tr.Relationships) > 0 {
		b.WriteString(fmt.Sprintf("Relationships (%d):\n", len(tr.Relationships)))
		for _, rel := range tr.Relationships {
			b.WriteString(fmt.Sprintf("  -[%s]-> %s\n", rel.Type, rel.To))
		}
		b.WriteString("\n")
	}

	if len(tr.Related) > 0 {
		b.WriteString(fmt.Sprintf("Related nodes (%d):\n", len(tr.Related)))
		for _, n := range tr.Related {
			name := ""
			if v, ok := n.Properties["name"].(string); ok {
				name = v
			}
			b.WriteString(fmt.Sprintf("  %s: %s\n", strings.Join(n.Labels, ":"), name))
		}
	}

	text := strings.TrimRight(b.String(), "\n")
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}

func formatDebugInfo(info *graph.DebugInfo) *mcp.CallToolResult {
	var b strings.Builder
	b.WriteString("Entity: ")
	b.WriteString(strings.Join(info.Entity.Labels, ":"))
	b.WriteString("\n")
	data, _ := json.MarshalIndent(info.Entity.Properties, "  ", "  ")
	b.WriteString(fmt.Sprintf("  %s\n\n", string(data)))

	if len(info.Issues) > 0 {
		b.WriteString(fmt.Sprintf("Issues (%d):\n", len(info.Issues)))
		for _, issue := range info.Issues {
			b.WriteString(fmt.Sprintf("  ⚠ %s\n", issue))
		}
		b.WriteString("\n")
	}

	if len(info.Relationships) > 0 {
		b.WriteString(fmt.Sprintf("Relationships (%d):\n", len(info.Relationships)))
		for _, rel := range info.Relationships {
			b.WriteString(fmt.Sprintf("  -[%s]-> %s\n", rel.Type, rel.To))
		}
		b.WriteString("\n")
	}

	if len(info.Related) > 0 {
		b.WriteString(fmt.Sprintf("Related (%d):\n", len(info.Related)))
		for _, n := range info.Related {
			name := ""
			if v, ok := n.Properties["name"].(string); ok {
				name = v
			}
			b.WriteString(fmt.Sprintf("  %s: %s\n", strings.Join(n.Labels, ":"), name))
		}
	}

	text := strings.TrimRight(b.String(), "\n")
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}
}
