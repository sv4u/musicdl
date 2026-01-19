package client

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"

	"github.com/sv4u/musicdl/download/proto"
	"github.com/sv4u/musicdl/download/server"
)

func TestNewDownloadClient(t *testing.T) {
	client := NewDownloadClient("", "v1.0.0")
	if client == nil {
		t.Fatal("NewDownloadClient returned nil")
	}

	if client.address != "localhost:30025" {
		t.Errorf("expected default address 'localhost:30025', got '%s'", client.address)
	}

	if client.version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got '%s'", client.version)
	}
}

func TestDownloadClient_Connect(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	planPath := filepath.Join(tmpDir, "plans")

	// Create and start gRPC server
	grpcServer := grpc.NewServer()
	downloadServer, err := server.NewDownloadServiceServer(planPath, logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer downloadServer.Close()

	proto.RegisterDownloadServiceServer(grpcServer, downloadServer)

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// Create client
	client := NewDownloadClient(lis.Addr().String(), "v1.0.0")

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected after Connect()")
	}

	// Test close
	if err := client.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}

	if client.IsConnected() {
		t.Error("Client should not be connected after Close()")
	}
}

func TestDownloadClient_HealthCheck(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	planPath := filepath.Join(tmpDir, "plans")

	// Create and start gRPC server
	grpcServer := grpc.NewServer()
	downloadServer, err := server.NewDownloadServiceServer(planPath, logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer downloadServer.Close()

	proto.RegisterDownloadServiceServer(grpcServer, downloadServer)

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create and connect client
	client := NewDownloadClient(lis.Addr().String(), "v1.0.0")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Test HealthCheck
	resp, err := client.HealthCheck(ctx)
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	if resp == nil {
		t.Fatal("HealthCheck returned nil response")
	}

	if resp.Liveness != proto.HealthStatus_HEALTH_STATUS_HEALTHY {
		t.Errorf("Expected liveness HEALTHY, got %v", resp.Liveness)
	}
}

func TestDownloadClient_HealthCheck_VersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	planPath := filepath.Join(tmpDir, "plans")

	// Create server with version v1.0.0
	grpcServer := grpc.NewServer()
	downloadServer, err := server.NewDownloadServiceServer(planPath, logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer downloadServer.Close()

	proto.RegisterDownloadServiceServer(grpcServer, downloadServer)

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create client with different version
	client := NewDownloadClient(lis.Addr().String(), "v1.0.1")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Test HealthCheck with version mismatch
	// Note: The server returns a response (not an error) but with unhealthy status
	resp, err := client.HealthCheck(ctx)
	if err != nil {
		// If there's an error, it should be a version mismatch error
		if !contains(err.Error(), "version mismatch") {
			t.Errorf("Expected version mismatch error, got: %v", err)
		}
	} else if resp != nil {
		// If no error, the response should indicate unhealthy status
		if resp.Liveness == proto.HealthStatus_HEALTH_STATUS_HEALTHY {
			t.Error("Expected unhealthy status due to version mismatch")
		}
	}
}

func TestDownloadClient_GetStatus(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	planPath := filepath.Join(tmpDir, "plans")

	// Create and start gRPC server
	grpcServer := grpc.NewServer()
	downloadServer, err := server.NewDownloadServiceServer(planPath, logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer downloadServer.Close()

	proto.RegisterDownloadServiceServer(grpcServer, downloadServer)

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create and connect client
	client := NewDownloadClient(lis.Addr().String(), "v1.0.0")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Test GetStatus
	resp, err := client.GetStatus(ctx)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if resp == nil {
		t.Fatal("GetStatus returned nil response")
	}

	if resp.State != proto.ServiceState_SERVICE_STATE_IDLE {
		t.Errorf("Expected state IDLE, got %v", resp.State)
	}
}

func TestDownloadClient_ValidateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	planPath := filepath.Join(tmpDir, "plans")

	// Create and start gRPC server
	grpcServer := grpc.NewServer()
	downloadServer, err := server.NewDownloadServiceServer(planPath, logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer downloadServer.Close()

	proto.RegisterDownloadServiceServer(grpcServer, downloadServer)

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create and connect client
	client := NewDownloadClient(lis.Addr().String(), "v1.0.0")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer client.Close()

	// Test ValidateConfig with valid config
	validConfig := &proto.Config{
		Version: "1.2",
		Download: &proto.DownloadSettings{
			ClientId:     "test_client_id",
			ClientSecret: "test_client_secret",
			Format:       "mp3",
			Bitrate:      "128k",
			AudioProviders: []string{"youtube-music"},
			Overwrite:    "skip",
		},
	}

	resp, err := client.ValidateConfig(ctx, validConfig)
	if err != nil {
		t.Fatalf("ValidateConfig failed: %v", err)
	}

	if resp == nil {
		t.Fatal("ValidateConfig returned nil response")
	}

	if !resp.Valid {
		t.Errorf("Expected config to be valid, got errors: %v", resp.Errors)
	}
}

func TestDownloadClient_Reconnect(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	planPath := filepath.Join(tmpDir, "plans")

	// Create and start gRPC server
	grpcServer := grpc.NewServer()
	downloadServer, err := server.NewDownloadServiceServer(planPath, logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer downloadServer.Close()

	proto.RegisterDownloadServiceServer(grpcServer, downloadServer)

	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create client
	client := NewDownloadClient(lis.Addr().String(), "v1.0.0")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	// Close connection
	if err := client.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reconnect
	if err := client.Connect(ctx); err != nil {
		t.Fatalf("Reconnect failed: %v", err)
	}

	if !client.IsConnected() {
		t.Error("Client should be connected after reconnect")
	}

	client.Close()
}

// contains checks if a string contains a substring (case-insensitive).
func contains(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) < len(substr) {
		return false
	}
	// Simple case-insensitive contains
	sLower := toLower(s)
	substrLower := toLower(substr)
	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}
	return false
}

// toLower converts a string to lowercase.
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		result[i] = c
	}
	return string(result)
}
