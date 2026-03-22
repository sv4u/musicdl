package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type browseLibraryParams struct {
	Path string `json:"path,omitempty" jsonschema:"optional subdirectory path to browse; empty means the library root"`
}

type searchLibraryParams struct {
	Query string `json:"query" jsonschema:"search term matched against file names"`
}

type getLibraryStatsParams struct{}

func registerLibraryTools(server *mcp.Server, provider DataProvider) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "browse_library",
		Description: "Browse downloaded music files under an optional subdirectory",
	}, func(_ context.Context, _ *mcp.CallToolRequest, params browseLibraryParams) (*mcp.CallToolResult, any, error) {
		entries, err := provider.BrowseLibrary(params.Path)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		if len(entries) == 0 {
			b.WriteString("(empty)\n")
		} else {
			for _, e := range entries {
				if e.IsDir {
					b.WriteString(fmt.Sprintf("[DIR] %s\t%s\n", e.Name, e.Path))
				} else {
					b.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\n", e.Name, formatHumanBytes(e.SizeBytes), e.Format, e.Path))
				}
			}
		}
		text := strings.TrimSuffix(b.String(), "\n")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_library",
		Description: "Search downloaded music files by name",
	}, func(_ context.Context, _ *mcp.CallToolRequest, params searchLibraryParams) (*mcp.CallToolResult, any, error) {
		q := strings.TrimSpace(params.Query)
		if q == "" {
			return nil, nil, fmt.Errorf("query is required")
		}
		entries, err := provider.SearchLibrary(q)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		if len(entries) == 0 {
			b.WriteString("No matches.\n")
		} else {
			for _, e := range entries {
				if e.IsDir {
					b.WriteString(fmt.Sprintf("[DIR] %s\t%s\n", e.Name, e.Path))
				} else {
					b.WriteString(fmt.Sprintf("%s\t%s\t%s\t%s\n", e.Name, formatHumanBytes(e.SizeBytes), e.Format, e.Path))
				}
			}
		}
		text := strings.TrimSuffix(b.String(), "\n")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_library_stats",
		Description: "Get music library statistics",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ getLibraryStatsParams) (*mcp.CallToolResult, any, error) {
		stats, err := provider.GetLibraryStats()
		if err != nil {
			return nil, nil, err
		}
		if stats == nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "No library statistics available."}}}, nil, nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Total files: %d\n", stats.TotalFiles))
		b.WriteString(fmt.Sprintf("Total size: %s (%d bytes)\n", formatHumanBytes(stats.TotalSize), stats.TotalSize))
		b.WriteString("\nBy format:\n")
		keys := make([]string, 0, len(stats.ByFormat))
		for k := range stats.ByFormat {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if len(keys) == 0 {
			b.WriteString("  (none)\n")
		} else {
			for _, k := range keys {
				b.WriteString(fmt.Sprintf("  %s: %d\n", k, stats.ByFormat[k]))
			}
		}
		b.WriteString(fmt.Sprintf("\nArtist count: %d\n", stats.ArtistCount))
		b.WriteString(fmt.Sprintf("Album count: %d\n", stats.AlbumCount))
		text := strings.TrimRight(b.String(), "\n")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}
