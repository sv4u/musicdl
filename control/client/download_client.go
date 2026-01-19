package client

import (
	"context"
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	"github.com/sv4u/musicdl/download/proto"
)

// DownloadClient is a gRPC client for the download service.
type DownloadClient struct {
	address string
	conn    *grpc.ClientConn
	client  proto.DownloadServiceClient
	mu      sync.RWMutex
	version string
}

// NewDownloadClient creates a new download service client.
// address is the gRPC server address (default: "localhost:30025").
// version is the client version for compatibility checking.
func NewDownloadClient(address, version string) *DownloadClient {
	if address == "" {
		address = "localhost:30025"
	}
	return &DownloadClient{
		address: address,
		version: version,
	}
}

// Connect establishes a connection to the download service.
func (c *DownloadClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		state := c.conn.GetState()
		if state == connectivity.Ready || state == connectivity.Idle {
			return nil // Already connected
		}
		// Connection exists but not ready, close it
		c.conn.Close()
		c.conn = nil
		c.client = nil
	}

	// Create connection with timeout
	connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(
		connCtx,
		c.address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return fmt.Errorf("failed to connect to download service at %s: %w", c.address, err)
	}

	c.conn = conn
	c.client = proto.NewDownloadServiceClient(conn)
	return nil
}

// Close closes the connection to the download service.
func (c *DownloadClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.client = nil
		return err
	}
	return nil
}

// ensureConnected ensures the client is connected, reconnecting if necessary.
func (c *DownloadClient) ensureConnected(ctx context.Context) error {
	c.mu.RLock()
	conn := c.conn
	client := c.client
	c.mu.RUnlock()

	if conn != nil && client != nil {
		state := conn.GetState()
		if state == connectivity.Ready || state == connectivity.Idle {
			return nil
		}
	}

	// Need to reconnect
	return c.Connect(ctx)
}

// getClientVersion returns the client version info.
func (c *DownloadClient) getClientVersion() *proto.VersionInfo {
	return &proto.VersionInfo{Version: c.version}
}

// StartDownload starts a download with the provided configuration.
func (c *DownloadClient) StartDownload(ctx context.Context, cfg *proto.Config, planPath, logPath string) error {
	if err := c.ensureConnected(ctx); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &proto.StartDownloadRequest{
		Config:        cfg,
		PlanPath:      planPath,
		LogPath:       logPath,
		ClientVersion: c.getClientVersion(),
	}

	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	resp, err := client.StartDownload(ctx, req)
	if err != nil {
		// Check if it's a version mismatch error
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			return fmt.Errorf("version mismatch: %s", st.Message())
		}
		return fmt.Errorf("failed to start download: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("start download failed: %s", resp.ErrorMessage)
	}

	// Verify server version matches
	if resp.ServerVersion != nil && resp.ServerVersion.Version != c.version {
		return fmt.Errorf("version mismatch: client version %s, server version %s", c.version, resp.ServerVersion.Version)
	}

	return nil
}

// StopDownload stops the running download.
func (c *DownloadClient) StopDownload(ctx context.Context) error {
	if err := c.ensureConnected(ctx); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	req := &proto.StopDownloadRequest{
		ClientVersion: c.getClientVersion(),
	}

	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	resp, err := client.StopDownload(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			return fmt.Errorf("version mismatch: %s", st.Message())
		}
		return fmt.Errorf("failed to stop download: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("stop download failed: %s", resp.ErrorMessage)
	}

	return nil
}

// GetStatus returns the current status of the download service.
func (c *DownloadClient) GetStatus(ctx context.Context) (*proto.GetStatusResponse, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req := &proto.GetStatusRequest{
		ClientVersion: c.getClientVersion(),
	}

	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	resp, err := client.GetStatus(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			return nil, fmt.Errorf("version mismatch: %s", st.Message())
		}
		return nil, fmt.Errorf("failed to get status: %w", err)
	}

	return resp, nil
}

// GetPlanItems returns plan items with optional filtering.
func (c *DownloadClient) GetPlanItems(ctx context.Context, filters *proto.PlanItemFilters) (*proto.GetPlanItemsResponse, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req := &proto.GetPlanItemsRequest{
		Filters:       filters,
		ClientVersion: c.getClientVersion(),
	}

	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	resp, err := client.GetPlanItems(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			return nil, fmt.Errorf("version mismatch: %s", st.Message())
		}
		return nil, fmt.Errorf("failed to get plan items: %w", err)
	}

	return resp, nil
}

// StreamLogs streams log entries from the download service.
func (c *DownloadClient) StreamLogs(ctx context.Context, filters *proto.StreamLogsRequest) (<-chan *proto.LogEntry, <-chan error) {
	logChan := make(chan *proto.LogEntry, 100)
	errChan := make(chan error, 1)

	go func() {
		defer close(logChan)
		defer close(errChan)

		if err := c.ensureConnected(ctx); err != nil {
			errChan <- err
			return
		}

		req := &proto.StreamLogsRequest{
			Levels:        filters.GetLevels(),
			Search:        filters.GetSearch(),
			Follow:        filters.GetFollow(),
			ClientVersion: c.getClientVersion(),
		}

		// Handle optional int64 fields (copy from filters if present)
		if filters != nil {
			if filters.StartTime != nil {
				req.StartTime = filters.StartTime
			}
			if filters.EndTime != nil {
				req.EndTime = filters.EndTime
			}
		}

		c.mu.RLock()
		client := c.client
		c.mu.RUnlock()

		stream, err := client.StreamLogs(ctx, req)
		if err != nil {
			if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
				errChan <- fmt.Errorf("version mismatch: %s", st.Message())
			} else {
				errChan <- fmt.Errorf("failed to stream logs: %w", err)
			}
			return
		}

		for {
			entry, err := stream.Recv()
			if err != nil {
				if err != context.Canceled && err != context.DeadlineExceeded {
					errChan <- fmt.Errorf("stream error: %w", err)
				}
				return
			}
			select {
			case logChan <- entry:
			case <-ctx.Done():
				return
			}
		}
	}()

	return logChan, errChan
}

// ValidateConfig validates a configuration.
func (c *DownloadClient) ValidateConfig(ctx context.Context, cfg *proto.Config) (*proto.ValidateConfigResponse, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req := &proto.ValidateConfigRequest{
		Config:        cfg,
		ClientVersion: c.getClientVersion(),
	}

	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	resp, err := client.ValidateConfig(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			return nil, fmt.Errorf("version mismatch: %s", st.Message())
		}
		return nil, fmt.Errorf("failed to validate config: %w", err)
	}

	return resp, nil
}

// HealthCheck checks the health of the download service.
func (c *DownloadClient) HealthCheck(ctx context.Context) (*proto.HealthCheckResponse, error) {
	if err := c.ensureConnected(ctx); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req := &proto.HealthCheckRequest{
		ClientVersion: c.getClientVersion(),
	}

	c.mu.RLock()
	client := c.client
	c.mu.RUnlock()

	resp, err := client.HealthCheck(ctx, req)
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() == codes.FailedPrecondition {
			return nil, fmt.Errorf("version mismatch: %s", st.Message())
		}
		return nil, fmt.Errorf("failed to check health: %w", err)
	}

	return resp, nil
}

// IsConnected returns whether the client is connected.
func (c *DownloadClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.conn == nil {
		return false
	}

	state := c.conn.GetState()
	return state == connectivity.Ready || state == connectivity.Idle
}
