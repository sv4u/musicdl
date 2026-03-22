package mcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerConfigTools(server *mcp.Server, provider DataProvider) {
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_config",
		Description: "Get current musicdl configuration as raw YAML.",
	}, func(_ context.Context, _ *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		raw, err := provider.GetConfigRaw()
		if err != nil {
			return nil, nil, err
		}
		text := fmt.Sprintf("```yaml\n%s\n```", strings.TrimRight(raw, "\n"))
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: text}}}, nil, nil
	})
}
