package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type searchLogsParams struct {
	Level   string `json:"level,omitempty" jsonschema:"optional log level filter: DEBUG, INFO, WARN, or ERROR"`
	Keyword string `json:"keyword,omitempty" jsonschema:"optional substring filter for log messages"`
	RunDir  string `json:"run_dir,omitempty" jsonschema:"optional run directory name (e.g. run_*) to search within"`
	Limit   int    `json:"limit,omitempty" jsonschema:"max entries to return; defaults to 100 if omitted or zero"`
}

type getRecentLogsParams struct {
	Count int `json:"count,omitempty" jsonschema:"number of most recent entries; defaults to 50 if omitted or zero"`
}

type listLogDirsParams struct{}

func registerLogTools(server *mcp.Server, provider DataProvider) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "search_logs",
		Description: "Search and filter structured logs",
	}, func(_ context.Context, _ *mcp.CallToolRequest, params searchLogsParams) (*mcp.CallToolResult, any, error) {
		limit := params.Limit
		if limit <= 0 {
			limit = 100
		}
		entries, err := provider.SearchLogs(LogFilter{
			Level:   params.Level,
			Keyword: params.Keyword,
			RunDir:  params.RunDir,
			Limit:   limit,
		})
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		for _, e := range entries {
			b.WriteString(formatLogEntryLine(e))
			b.WriteByte('\n')
		}
		text := strings.TrimSuffix(b.String(), "\n")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_recent_logs",
		Description: "Get the N most recent log entries",
	}, func(_ context.Context, _ *mcp.CallToolRequest, params getRecentLogsParams) (*mcp.CallToolResult, any, error) {
		count := params.Count
		if count <= 0 {
			count = 50
		}
		entries, err := provider.GetRecentLogs(count)
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		for _, e := range entries {
			b.WriteString(formatLogEntryLine(e))
			b.WriteByte('\n')
		}
		text := strings.TrimSuffix(b.String(), "\n")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_log_dirs",
		Description: "List available log directories (one per run)",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ listLogDirsParams) (*mcp.CallToolResult, any, error) {
		dirs, err := provider.ListRunLogDirs()
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		for _, d := range dirs {
			b.WriteString(fmt.Sprintf("%s — created %s\n", d.Name, d.CreatedAt.Format("2006-01-02 15:04:05")))
		}
		text := strings.TrimSuffix(b.String(), "\n")
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}

func formatLogEntryLine(e LogEntryInfo) string {
	ts := e.Timestamp.Format(time.RFC3339)
	level := strings.ToUpper(strings.TrimSpace(e.Level))
	if level == "" {
		level = "INFO"
	}
	line := fmt.Sprintf("[%s] [%s] %s", ts, level, e.Message)
	if strings.TrimSpace(e.Service) != "" {
		line += fmt.Sprintf(" (%s)", e.Service)
	}
	if strings.TrimSpace(e.Error) != "" {
		line += fmt.Sprintf(" {%s}", e.Error)
	}
	return line
}
