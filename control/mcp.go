package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
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
	server := mcpserver.NewServer(provider, Version)

	if sseMode {
		return runSSE(server, port)
	}
	return runStdio(server)
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
