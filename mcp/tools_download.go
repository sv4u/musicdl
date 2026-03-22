package mcp

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerDownloadTools(server *mcp.Server, provider DataProvider) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_download_status",
		Description: "Get current download/plan operation status (running state, progress, errors).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		st, err := provider.GetDownloadStatus()
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		if st == nil {
			b.WriteString("No status available.\n")
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: b.String()}}}, nil, nil
		}
		if !st.IsRunning {
			b.WriteString("No operation is currently running.\n\n")
		}
		b.WriteString(fmt.Sprintf("Running: %v\n", st.IsRunning))
		b.WriteString(fmt.Sprintf("Operation: %s\n", st.OperationType))
		if st.Total > 0 {
			b.WriteString(fmt.Sprintf("Progress: %d / %d\n", st.Progress, st.Total))
		} else {
			b.WriteString(fmt.Sprintf("Progress: %d (total unknown)\n", st.Progress))
		}
		if st.StartedAt != 0 {
			b.WriteString(fmt.Sprintf("Started: %s\n", formatUnixTime(st.StartedAt)))
		} else {
			b.WriteString("Started: —\n")
		}
		if st.ErrorMsg != "" {
			b.WriteString(fmt.Sprintf("Error: %s\n", st.ErrorMsg))
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_stats",
		Description: "Get download statistics: cumulative lifetime totals and current run (if any).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		stats, err := provider.GetStats()
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		if stats == nil {
			b.WriteString("No statistics available.\n")
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: b.String()}}}, nil, nil
		}
		if stats.Cumulative != nil {
			c := stats.Cumulative
			b.WriteString("Cumulative\n")
			b.WriteString("---------\n")
			b.WriteString(fmt.Sprintf("Downloaded: %d\n", c.TotalDownloaded))
			b.WriteString(fmt.Sprintf("Failed: %d\n", c.TotalFailed))
			b.WriteString(fmt.Sprintf("Skipped: %d\n", c.TotalSkipped))
			b.WriteString(fmt.Sprintf("Plans generated: %d\n", c.TotalPlansGenerated))
			b.WriteString(fmt.Sprintf("Runs: %d\n", c.TotalRuns))
			b.WriteString(fmt.Sprintf("Rate limits: %d\n", c.TotalRateLimits))
			b.WriteString(fmt.Sprintf("Retries: %d\n", c.TotalRetries))
			b.WriteString(fmt.Sprintf("Bytes written: %s\n", formatHumanBytes(c.TotalBytesWritten)))
			b.WriteString(fmt.Sprintf("Total time: %s\n", formatHumanDurationSec(c.TotalTimeSpentSec)))
			b.WriteString(fmt.Sprintf("Plan time: %s\n", formatHumanDurationSec(c.PlanTimeSpentSec)))
			b.WriteString(fmt.Sprintf("Download time: %s\n", formatHumanDurationSec(c.DownloadTimeSpentSec)))
			if c.FirstRunAt != 0 {
				b.WriteString(fmt.Sprintf("First run: %s\n", formatUnixTime(c.FirstRunAt)))
			}
			if c.LastRunAt != 0 {
				b.WriteString(fmt.Sprintf("Last run: %s\n", formatUnixTime(c.LastRunAt)))
			}
			b.WriteString(fmt.Sprintf("Success rate: %.2f%%\n", c.SuccessRate))
		} else {
			b.WriteString("Cumulative: (none)\n")
		}
		b.WriteString("\n")
		if stats.CurrentRun != nil && stats.CurrentRun.IsRunning {
			r := stats.CurrentRun
			b.WriteString("Current run\n")
			b.WriteString("-----------\n")
			b.WriteString(fmt.Sprintf("Run ID: %s\n", r.RunID))
			b.WriteString(fmt.Sprintf("Operation: %s\n", r.OperationType))
			if r.StartedAt != 0 {
				b.WriteString(fmt.Sprintf("Started: %s\n", formatUnixTime(r.StartedAt)))
			}
			b.WriteString(fmt.Sprintf("Running: %v\n", r.IsRunning))
			b.WriteString(fmt.Sprintf("Downloaded: %d\n", r.Downloaded))
			b.WriteString(fmt.Sprintf("Failed: %d\n", r.Failed))
			b.WriteString(fmt.Sprintf("Skipped: %d\n", r.Skipped))
			b.WriteString(fmt.Sprintf("Retries: %d\n", r.Retries))
			b.WriteString(fmt.Sprintf("Rate limits: %d\n", r.RateLimits))
			b.WriteString(fmt.Sprintf("Bytes written: %s\n", formatHumanBytes(r.BytesWritten)))
			b.WriteString(fmt.Sprintf("Elapsed: %s\n", formatHumanDurationSec(r.ElapsedSec)))
			b.WriteString(fmt.Sprintf("Tracks/hour: %.2f\n", r.TracksPerHour))
		} else {
			b.WriteString("Current run: (none active)\n")
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
	})
}

func formatUnixTime(ts int64) string {
	return time.Unix(ts, 0).UTC().Format(time.RFC3339)
}

func formatHumanBytes(n int64) string {
	if n <= 0 {
		return "0 B"
	}
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)
	switch {
	case n >= gb:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(gb))
	case n >= mb:
		return fmt.Sprintf("%.2f MB", float64(n)/float64(mb))
	case n >= kb:
		return fmt.Sprintf("%.2f KB", float64(n)/float64(kb))
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func formatHumanDurationSec(sec float64) string {
	if sec < 0 {
		sec = 0
	}
	s := int64(sec + 0.5)
	if s == 0 && sec > 0 {
		s = 1
	}
	h := s / 3600
	m := (s % 3600) / 60
	r := s % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm %ds", h, m, r)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, r)
	}
	return fmt.Sprintf("%ds", r)
}
