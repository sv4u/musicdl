package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// NewServer creates an MCP server with all musicdl tools and resources registered.
// The version parameter is set at build time via the musicdl binary.
func NewServer(provider DataProvider, version string) *mcp.Server {
	if version == "" {
		version = "dev"
	}

	server := mcp.NewServer(&mcp.Implementation{
		Name:    "musicdl",
		Version: version,
	}, nil)

	registerPlanTools(server, provider)
	registerDownloadTools(server, provider)
	registerLogTools(server, provider)
	registerConfigTools(server, provider)
	registerHistoryTools(server, provider)
	registerHealthTools(server, provider)
	registerCacheTools(server, provider)
	registerLibraryTools(server, provider)
	registerPlexTools(server, provider)

	return server
}
