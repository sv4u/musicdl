package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerHealthTools(server *mcp.Server, provider DataProvider) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_health",
		Description: "Get system health status and version information (musicdl, spotigo, Go, API).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		info, err := provider.GetHealth()
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		b.WriteString("Health\n")
		b.WriteString("------\n")
		b.WriteString(fmt.Sprintf("Status: %s\n", info.Status))
		b.WriteString(fmt.Sprintf("musicdl: %s\n", info.MusicdlVersion))
		b.WriteString(fmt.Sprintf("spotigo: %s\n", info.SpotigoVersion))
		b.WriteString(fmt.Sprintf("Go: %s\n", info.GoVersion))
		b.WriteString(fmt.Sprintf("API running: %v\n", info.APIRunning))
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_recovery_status",
		Description: "Get circuit breaker state and download resume progress (failed items, counts).",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		rec, err := provider.GetRecoveryStatus()
		if err != nil {
			return nil, nil, err
		}
		var b strings.Builder
		cb := rec.CircuitBreaker
		b.WriteString("Circuit breaker\n")
		b.WriteString("---------------\n")
		b.WriteString(fmt.Sprintf("State: %s\n", cb.State))
		b.WriteString(fmt.Sprintf("Failure count: %d\n", cb.FailureCount))
		b.WriteString(fmt.Sprintf("Success count: %d\n", cb.SuccessCount))
		b.WriteString(fmt.Sprintf("Failure threshold: %d\n", cb.FailureThreshold))
		b.WriteString(fmt.Sprintf("Success threshold: %d\n", cb.SuccessThreshold))
		b.WriteString(fmt.Sprintf("Reset timeout: %ds\n", cb.ResetTimeoutSec))
		if cb.LastFailureAt != 0 {
			b.WriteString(fmt.Sprintf("Last failure: %s\n", formatUnixTime(cb.LastFailureAt)))
		} else {
			b.WriteString("Last failure: —\n")
		}
		if cb.LastStateChange != 0 {
			b.WriteString(fmt.Sprintf("Last state change: %s\n", formatUnixTime(cb.LastStateChange)))
		} else {
			b.WriteString("Last state change: —\n")
		}
		b.WriteString(fmt.Sprintf("Can retry: %v\n", cb.CanRetry))

		rs := rec.Resume
		b.WriteString("\nResume\n")
		b.WriteString("------\n")
		b.WriteString(fmt.Sprintf("Has resume data: %v\n", rs.HasResumeData))
		b.WriteString(fmt.Sprintf("Completed: %d\n", rs.CompletedCount))
		b.WriteString(fmt.Sprintf("Failed: %d\n", rs.FailedCount))
		b.WriteString(fmt.Sprintf("Total items: %d\n", rs.TotalItems))
		b.WriteString(fmt.Sprintf("Remaining: %d\n", rs.RemainingCount))
		if len(rs.FailedItems) == 0 {
			b.WriteString("Failed items: (none)\n")
		} else {
			b.WriteString(fmt.Sprintf("Failed items (%d):\n", len(rs.FailedItems)))
			for i, fi := range rs.FailedItems {
				if i > 0 {
					b.WriteByte('\n')
				}
				b.WriteString(fmt.Sprintf("  [%d] %s\n", i+1, fi.Name))
				b.WriteString(fmt.Sprintf("      id: %s\n", fi.ID))
				b.WriteString(fmt.Sprintf("      url: %s\n", fi.URL))
				b.WriteString(fmt.Sprintf("      error: %s\n", fi.Error))
				b.WriteString(fmt.Sprintf("      attempts: %d\n", fi.Attempts))
				if fi.LastAttempt != 0 {
					b.WriteString(fmt.Sprintf("      last attempt: %s\n", formatUnixTime(fi.LastAttempt)))
				} else {
					b.WriteString("      last attempt: —\n")
				}
				b.WriteString(fmt.Sprintf("      retryable: %v\n", fi.Retryable))
			}
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: strings.TrimRight(b.String(), "\n")}}}, nil, nil
	})
}
