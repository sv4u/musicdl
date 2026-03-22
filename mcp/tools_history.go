package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sv4u/musicdl/download/history"
)

type listRunsParams struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum number of past download runs to return; defaults to 20 if omitted or zero"`
}

type getRunDetailsParams struct {
	RunID string `json:"run_id" jsonschema:"unique identifier of the download run"`
}

type getActivityParams struct {
	Limit int `json:"limit,omitempty" jsonschema:"maximum number of activity entries to return; defaults to 50 if omitted or zero"`
}

func registerHistoryTools(server *mcp.Server, provider DataProvider) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_runs",
		Description: "List past download runs with identifiers, timing, state, and statistics.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, params listRunsParams) (*mcp.CallToolResult, any, error) {
		limit := params.Limit
		if limit <= 0 {
			limit = 20
		}
		runs, err := provider.ListRuns(limit)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		if len(runs) == 0 {
			b.WriteString("(no runs)\n")
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
		}
		for i, r := range runs {
			if i > 0 {
				b.WriteString("\n---\n\n")
			}
			b.WriteString(fmt.Sprintf("Run ID: %s\n", r.RunID))
			b.WriteString(fmt.Sprintf("Started: %s\n", r.StartedAt.Format("2006-01-02 15:04:05 MST")))
			if r.CompletedAt != nil {
				b.WriteString(fmt.Sprintf("Completed: %s\n", r.CompletedAt.Format("2006-01-02 15:04:05 MST")))
			} else {
				b.WriteString("Completed: —\n")
			}
			b.WriteString(fmt.Sprintf("State: %s\n", r.State))
			if r.Error != "" {
				b.WriteString(fmt.Sprintf("Error: %s\n", r.Error))
			}
			b.WriteString("Statistics:\n")
			b.WriteString(formatStatsMap(r.Statistics))
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_run_details",
		Description: "Get full details for a single download run, including progress snapshots.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, params getRunDetailsParams) (*mcp.CallToolResult, any, error) {
		var rh *history.RunHistory
		var err error
		rh, err = provider.GetRunDetails(params.RunID)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		b.WriteString(fmt.Sprintf("Run ID: %s\n", rh.RunID))
		b.WriteString(fmt.Sprintf("Started: %s\n", rh.StartedAt.Format("2006-01-02 15:04:05 MST")))
		if rh.CompletedAt != nil {
			b.WriteString(fmt.Sprintf("Completed: %s\n", rh.CompletedAt.Format("2006-01-02 15:04:05 MST")))
		} else {
			b.WriteString("Completed: —\n")
		}
		b.WriteString(fmt.Sprintf("State: %s\n", rh.State))
		b.WriteString(fmt.Sprintf("Phase: %s\n", rh.Phase))
		if rh.Error != "" {
			b.WriteString(fmt.Sprintf("Error: %s\n", rh.Error))
		}
		b.WriteString("Final statistics:\n")
		b.WriteString(formatStatsMap(rh.Statistics))
		b.WriteString("\nSnapshots")
		if len(rh.Snapshots) == 0 {
			b.WriteString(": (none)\n")
		} else {
			b.WriteString(fmt.Sprintf(" (%d):\n", len(rh.Snapshots)))
			for i, snap := range rh.Snapshots {
				b.WriteString(fmt.Sprintf("  [%d] %s — progress %.1f%% — state %s — phase %s\n",
					i+1, snap.Timestamp.Format("2006-01-02 15:04:05 MST"), snap.Progress, snap.State, snap.Phase))
				b.WriteString(formatStatsMapIndented(snap.Statistics, "    "))
			}
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_activity",
		Description: "Get recent activity entries as a chronological timeline.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, params getActivityParams) (*mcp.CallToolResult, any, error) {
		limit := params.Limit
		if limit <= 0 {
			limit = 50
		}
		entries, err := provider.GetActivity(limit)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		if len(entries) == 0 {
			b.WriteString("(no activity)\n")
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
		}
		for i, e := range entries {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(fmt.Sprintf("[%s] %s — %s\n", e.Timestamp.Format("2006-01-02 15:04:05 MST"), e.Type, e.Message))
			b.WriteString(fmt.Sprintf("  id: %s\n", e.ID))
			if len(e.Details) > 0 {
				det, err := json.Marshal(e.Details)
				if err != nil {
					b.WriteString(fmt.Sprintf("  details: %v\n", e.Details))
				} else {
					b.WriteString(fmt.Sprintf("  details: %s\n", string(det)))
				}
			}
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
	})
}

func formatStatsMap(m map[string]interface{}) string {
	if len(m) == 0 {
		return "  (none)\n"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("  %s: %v\n", k, m[k]))
	}
	return b.String()
}

func formatStatsMapIndented(m map[string]interface{}, indent string) string {
	if len(m) == 0 {
		return indent + "(none)\n"
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("%s%s: %v\n", indent, k, m[k]))
	}
	return b.String()
}
