package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type getCacheInfoParams struct{}

func registerCacheTools(server *mcp.Server, provider DataProvider) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_cache_info",
		Description: "Get cache directory information",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ getCacheInfoParams) (*mcp.CallToolResult, any, error) {
		info, err := provider.GetCacheInfo()
		if err != nil {
			return nil, nil, err
		}
		if info == nil {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "No cache information available."}}}, nil, nil
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Cache directory: %s\n", info.CacheDir))
		b.WriteString(fmt.Sprintf("Total size: %s (%d bytes)\n", formatHumanBytes(info.TotalSize), info.TotalSize))
		b.WriteString("\nPlan files:\n")
		if len(info.PlanFiles) == 0 {
			b.WriteString("  (none)\n")
		} else {
			for _, p := range info.PlanFiles {
				b.WriteString(fmt.Sprintf("  %s\n", p.Hash))
				b.WriteString(fmt.Sprintf("    path: %s\n", p.Path))
				b.WriteString(fmt.Sprintf("    modified: %s\n", p.ModifiedAt.UTC().Format(time.RFC3339)))
				b.WriteString(fmt.Sprintf("    size: %s (%d bytes)\n", formatHumanBytes(p.SizeBytes), p.SizeBytes))
				b.WriteString(fmt.Sprintf("    tracks: %d\n", p.TrackCount))
			}
		}
		b.WriteString(fmt.Sprintf("\nStats file: %s\n", dashEmpty(info.StatsFile)))
		b.WriteString(fmt.Sprintf("Resume file: %s\n", dashEmpty(info.ResumeFile)))
		b.WriteString(fmt.Sprintf("History directory: %s\n", dashEmpty(info.HistoryDir)))
		b.WriteString(fmt.Sprintf("History entries: %d\n", info.HistoryCount))
		text := strings.TrimRight(b.String(), "\n")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}

func dashEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
