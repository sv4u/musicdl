package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sv4u/musicdl/download/graph"
)

// ServerOptions holds optional dependencies for the MCP server.
type ServerOptions struct {
	GraphClient *graph.Client
	WorkDir     string
}

// NewServer creates an MCP server with all musicdl tools and resources registered.
// The version parameter is set at build time via the musicdl binary.
func NewServer(provider DataProvider, version string, opts *ServerOptions) *mcp.Server {
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

	if opts != nil && opts.GraphClient != nil {
		registerGraphTools(server, opts.GraphClient, opts.WorkDir)
	}

	return server
}
