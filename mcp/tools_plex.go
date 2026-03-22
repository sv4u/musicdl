package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerPlexTools(server *mcp.Server, provider DataProvider) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_plex_sync_status",
		Description: "Get the current Plex playlist sync status, including progress, per-playlist results, and any errors.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		status, err := provider.GetPlexSyncStatus()
		if err != nil {
			return nil, nil, err
		}
		var sb strings.Builder
		if status.IsRunning {
			sb.WriteString(fmt.Sprintf("## Plex Sync — In Progress\n\nProgress: %d / %d playlists\n", status.Progress, status.Total))
		} else if status.StartedAt == 0 {
			sb.WriteString("## Plex Sync — Idle\n\nNo sync has been run yet.\n")
		} else {
			sb.WriteString("## Plex Sync — Complete\n\n")
			if status.Error != "" {
				sb.WriteString(fmt.Sprintf("**Error:** %s\n\n", status.Error))
			}
			sb.WriteString(fmt.Sprintf("Playlists processed: %d / %d\n", status.Progress, status.Total))
		}
		if len(status.Results) > 0 {
			sb.WriteString("\n### Results\n\n| Playlist | Action | Tracks | Error |\n|---|---|---|---|\n")
			for _, r := range status.Results {
				errStr := "—"
				if r.Error != "" {
					errStr = r.Error
				}
				tracks := "—"
				if r.TrackCount > 0 {
					tracks = fmt.Sprintf("%d", r.TrackCount)
				}
				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", r.PlaylistName, r.Action, tracks, errStr))
			}
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: sb.String()}}}, nil, nil
	})
}
