package server

import (
	"context"
	"net"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/sv4u/musicdl/download/proto"
)

func TestDownloadServiceServer_HealthCheck(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	planPath := filepath.Join(tmpDir, "plans")

	// Create server
	server, err := NewDownloadServiceServer(planPath, logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	// Create gRPC server and register service
	grpcServer := grpc.NewServer()
	proto.RegisterDownloadServiceServer(grpcServer, server)

	// Start server on random port
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Create client
	conn, err := grpc.Dial(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := proto.NewDownloadServiceClient(conn)

	// Test HealthCheck
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &proto.HealthCheckRequest{
		ClientVersion: &proto.VersionInfo{Version: "v1.0.0"},
	}

	resp, err := client.HealthCheck(ctx, req)
	if err != nil {
		t.Fatalf("HealthCheck failed: %v", err)
	}

	if resp == nil {
		t.Fatal("HealthCheck returned nil response")
	}

	if resp.Liveness != proto.HealthStatus_HEALTH_STATUS_HEALTHY {
		t.Errorf("Expected liveness HEALTHY, got %v", resp.Liveness)
	}

	if resp.ServerVersion == nil || resp.ServerVersion.Version != "v1.0.0" {
		t.Errorf("Expected server version v1.0.0, got %v", resp.ServerVersion)
	}
}

func TestDownloadServiceServer_HealthCheck_VersionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	planPath := filepath.Join(tmpDir, "plans")

	// Create server
	server, err := NewDownloadServiceServer(planPath, logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	// Create gRPC server and register service
	grpcServer := grpc.NewServer()
	proto.RegisterDownloadServiceServer(grpcServer, server)

	// Start server
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Create client
	conn, err := grpc.Dial(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := proto.NewDownloadServiceClient(conn)

	// Test HealthCheck with version mismatch
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &proto.HealthCheckRequest{
		ClientVersion: &proto.VersionInfo{Version: "v1.0.1"}, // Different version
	}

	resp, err := client.HealthCheck(ctx, req)
	if err != nil {
		t.Fatalf("HealthCheck should return response even on version mismatch, got error: %v", err)
	}

	// Should return unhealthy status due to version mismatch
	if resp.Liveness == proto.HealthStatus_HEALTH_STATUS_HEALTHY {
		t.Error("Expected unhealthy status due to version mismatch")
	}
}

func TestDownloadServiceServer_GetStatus_NoService(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	planPath := filepath.Join(tmpDir, "plans")

	// Create server
	server, err := NewDownloadServiceServer(planPath, logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	proto.RegisterDownloadServiceServer(grpcServer, server)

	// Start server
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create client
	conn, err := grpc.Dial(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := proto.NewDownloadServiceClient(conn)

	// Test GetStatus when service is not initialized
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req := &proto.GetStatusRequest{
		ClientVersion: &proto.VersionInfo{Version: "v1.0.0"},
	}

	resp, err := client.GetStatus(ctx, req)
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if resp == nil {
		t.Fatal("GetStatus returned nil response")
	}

	if resp.State != proto.ServiceState_SERVICE_STATE_IDLE {
		t.Errorf("Expected state IDLE, got %v", resp.State)
	}

	if resp.Phase != proto.ServicePhase_SERVICE_PHASE_IDLE {
		t.Errorf("Expected phase IDLE, got %v", resp.Phase)
	}
}

func TestDownloadServiceServer_ValidateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	planPath := filepath.Join(tmpDir, "plans")

	// Create server
	server, err := NewDownloadServiceServer(planPath, logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	proto.RegisterDownloadServiceServer(grpcServer, server)

	// Start server
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create client
	conn, err := grpc.Dial(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := proto.NewDownloadServiceClient(conn)

	// Test ValidateConfig with valid config
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

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

	req := &proto.ValidateConfigRequest{
		Config:        validConfig,
		ClientVersion: &proto.VersionInfo{Version: "v1.0.0"},
	}

	resp, err := client.ValidateConfig(ctx, req)
	if err != nil {
		t.Fatalf("ValidateConfig failed: %v", err)
	}

	if resp == nil {
		t.Fatal("ValidateConfig returned nil response")
	}

	if !resp.Valid {
		t.Errorf("Expected config to be valid, but got errors: %v", resp.Errors)
	}
}

func TestDownloadServiceServer_ValidateConfig_Invalid(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")
	planPath := filepath.Join(tmpDir, "plans")

	// Create server
	server, err := NewDownloadServiceServer(planPath, logPath, "v1.0.0")
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}
	defer server.Close()

	// Create gRPC server
	grpcServer := grpc.NewServer()
	proto.RegisterDownloadServiceServer(grpcServer, server)

	// Start server
	lis, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}

	go grpcServer.Serve(lis)
	defer grpcServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Create client
	conn, err := grpc.Dial(
		lis.Addr().String(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := proto.NewDownloadServiceClient(conn)

	// Test ValidateConfig with invalid config (missing credentials)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	invalidConfig := &proto.Config{
		Version: "1.2",
		Download: &proto.DownloadSettings{
			// Missing ClientId and ClientSecret
			Format:        "mp3",
			Bitrate:       "128k",
			AudioProviders: []string{"youtube-music"},
			Overwrite:     "skip",
		},
	}

	req := &proto.ValidateConfigRequest{
		Config:        invalidConfig,
		ClientVersion: &proto.VersionInfo{Version: "v1.0.0"},
	}

	resp, err := client.ValidateConfig(ctx, req)
	if err != nil {
		t.Fatalf("ValidateConfig should return response even for invalid config, got error: %v", err)
	}

	if resp == nil {
		t.Fatal("ValidateConfig returned nil response")
	}

	if resp.Valid {
		t.Error("Expected config to be invalid, but got valid=true")
	}

	if len(resp.Errors) == 0 {
		t.Error("Expected validation errors, but got none")
	}
}
