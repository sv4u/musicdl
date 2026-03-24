package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/graph"
	mcpserver "github.com/sv4u/musicdl/mcp"
)

const (
	MCPExitSuccess = 0
	MCPExitError   = 1
)

// mcpCommand starts the MCP server in either stdio or SSE mode.
func mcpCommand(args []string) int {
	sseMode := false
	port := 8090

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--sse":
			sseMode = true
		case "--port":
			if i+1 < len(args) {
				i++
				if p, err := strconv.Atoi(args[i]); err == nil {
					port = p
				}
			}
		}
	}

	workDir := os.Getenv("MUSICDL_WORK_DIR")
	if workDir == "" {
		workDir = "."
	}

	provider := mcpserver.NewFileDataProvider(workDir, getCacheDir(), getLogDir())

	opts := &mcpserver.ServerOptions{WorkDir: workDir}
	graphClient := tryConnectGraph(workDir)
	if graphClient != nil {
		opts.GraphClient = graphClient
		defer func() { _ = graphClient.Close(context.Background()) }()
	}

	server := mcpserver.NewServer(provider, Version, opts)

	if sseMode {
		return runSSE(server, port)
	}
	return runStdio(server)
}

// tryConnectGraph attempts to create a graph client from config + env vars.
// Returns nil if graph is disabled or connection fails (non-fatal).
func tryConnectGraph(workDir string) *graph.Client {
	configPath := filepath.Join(workDir, "config.yaml")
	cfg, err := config.LoadConfig(configPath)
	graphCfg := graph.Config{
		URI:      "bolt://localhost:7687",
		Username: "neo4j",
		Database: "neo4j",
	}

	if err == nil && cfg.Graph.Enabled {
		if cfg.Graph.URI != "" {
			graphCfg.URI = cfg.Graph.URI
		}
		if cfg.Graph.Username != "" {
			graphCfg.Username = cfg.Graph.Username
		}
		graphCfg.Password = cfg.Graph.Password
		if cfg.Graph.Database != "" {
			graphCfg.Database = cfg.Graph.Database
		}
	} else if err != nil {
		// Config load failed; still check env vars
	}

	graphCfg = graph.ConfigFromEnv(graphCfg)

	// Only connect if explicitly enabled via config or env vars provide credentials
	if (cfg == nil || !cfg.Graph.Enabled) && graphCfg.Password == "" {
		return nil
	}

	client, connErr := graph.NewClient(context.Background(), graphCfg)
	if connErr != nil {
		log.Printf("WARN: graph_memory connection failed (non-fatal): %v", connErr)
		return nil
	}
	return client
}

func runStdio(server *gomcp.Server) int {
	if err := server.Run(context.Background(), &gomcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "MCP stdio error: %v\n", err)
		return MCPExitError
	}
	return MCPExitSuccess
}

func runSSE(server *gomcp.Server, port int) int {
	handler := gomcp.NewStreamableHTTPHandler(func(_ *http.Request) *gomcp.Server {
		return server
	}, nil)

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	fmt.Fprintf(os.Stderr, "Starting musicdl MCP server (SSE) on %s\n", addr)
	fmt.Fprintf(os.Stderr, "  Connect via: http://localhost:%d\n", port)

	httpServer := &http.Server{
		Addr:    addr,
		Handler: handler,
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "\nShutting down MCP server...")
		httpServer.Close()
	}()

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		fmt.Fprintf(os.Stderr, "MCP server error: %v\n", err)
		return MCPExitError
	}
	return MCPExitSuccess
}
