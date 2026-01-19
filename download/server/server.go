package server

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"

	"github.com/sv4u/musicdl/download/logging"
	"github.com/sv4u/musicdl/download/proto"
)

// RunServer starts the gRPC server and blocks until shutdown.
func RunServer(port string, planPath, logPath, version string) error {
	// Create logger
	logger, err := logging.NewLogger(logPath, "download-service")
	if err != nil {
		return fmt.Errorf("failed to create logger: %w", err)
	}
	defer logger.Close()

	logger.InfoWithOperation("server_startup", fmt.Sprintf("Starting gRPC server on port %s", port))

	// Create server instance
	server, err := NewDownloadServiceServer(planPath, logPath, version)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	defer server.Close()

	// Create gRPC server with keepalive settings
	grpcServer := grpc.NewServer(
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Second,
			MaxConnectionAge:       30 * time.Second,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  5 * time.Second,
			Timeout:               1 * time.Second,
		}),
		grpc.MaxConcurrentStreams(100),
	)

	// Register service
	proto.RegisterDownloadServiceServer(grpcServer, server)

	// Listen on port
	lis, err := net.Listen("tcp", fmt.Sprintf(":%s", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %s: %w", port, err)
	}

	logger.InfoWithOperation("server_startup", fmt.Sprintf("gRPC server listening on port %s", port))

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			errChan <- fmt.Errorf("gRPC server error: %w", err)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	select {
	case err := <-errChan:
		return err
	case sig := <-sigChan:
		logger.InfoWithOperation("server_shutdown", fmt.Sprintf("Received signal: %v, shutting down gracefully", sig))

		// Graceful shutdown
		// Stop accepting new connections
		grpcServer.GracefulStop()

		// Stop download service
		server.serviceMu.RLock()
		svc := server.service
		server.serviceMu.RUnlock()

		if svc != nil {
			if err := svc.Stop(); err != nil {
				logger.WarnWithOperation("server_shutdown", fmt.Sprintf("Error stopping download service: %v", err))
			}
		}

		logger.InfoWithOperation("server_shutdown", "Server shutdown complete")
		return nil
	}
}
