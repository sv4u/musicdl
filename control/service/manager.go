package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/sv4u/musicdl/control/client"
	"github.com/sv4u/musicdl/download/proto"
)

// ProcessState represents the state of the download service process.
type ProcessState string

const (
	ProcessStateStopped ProcessState = "stopped"
	ProcessStateStarting ProcessState = "starting"
	ProcessStateRunning  ProcessState = "running"
	ProcessStateStopping ProcessState = "stopping"
	ProcessStateError    ProcessState = "error"
)

// Manager manages the download service process lifecycle.
type Manager struct {
	// Process management
	process     *exec.Cmd
	processMu   sync.RWMutex
	processState ProcessState
	processPID  int

	// Configuration
	clientAddress string
	clientVersion string
	planPath      string
	logPath       string

	// gRPC client
	client *client.DownloadClient
	clientMu sync.RWMutex

	// Monitoring
	monitorCancel context.CancelFunc
	monitorWg     sync.WaitGroup
}

// NewManager creates a new service manager.
func NewManager(clientAddress, clientVersion, planPath, logPath string) *Manager {
	if clientAddress == "" {
		clientAddress = "localhost:30025"
	}

	return &Manager{
		clientAddress: clientAddress,
		clientVersion: clientVersion,
		planPath:      planPath,
		logPath:       logPath,
		processState:  ProcessStateStopped,
	}
}

// StartService starts the download service process.
func (m *Manager) StartService(ctx context.Context) error {
	m.processMu.Lock()
	defer m.processMu.Unlock()

	if m.processState == ProcessStateRunning || m.processState == ProcessStateStarting {
		return fmt.Errorf("service is already running or starting")
	}

	// Get the executable path (assume we're running from the musicdl binary)
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Build command
	cmd := exec.CommandContext(ctx, executable, "download-service",
		"--plan-path", m.planPath,
		"--log-path", m.logPath,
		"--port", "30025",
	)

	// Set environment variables
	cmd.Env = os.Environ()

	// Start process
	if err := cmd.Start(); err != nil {
		m.processState = ProcessStateError
		return fmt.Errorf("failed to start process: %w", err)
	}

	m.process = cmd
	m.processPID = cmd.Process.Pid
	m.processState = ProcessStateStarting

	// Start monitoring goroutine
	monitorCtx, cancel := context.WithCancel(context.Background())
	m.monitorCancel = cancel
	m.monitorWg.Add(1)
	go m.monitorProcess(monitorCtx)

	// Wait for gRPC server to be ready
	if err := m.WaitForReady(ctx, 30*time.Second); err != nil {
		m.processMu.Lock()
		if m.process != nil {
			m.process.Process.Kill()
			m.process = nil
		}
		m.processState = ProcessStateError
		m.processMu.Unlock()
		return fmt.Errorf("service failed to become ready: %w", err)
	}

	m.processMu.Lock()
	m.processState = ProcessStateRunning
	m.processMu.Unlock()

	return nil
}

// StopService stops the download service gracefully.
func (m *Manager) StopService(ctx context.Context) error {
	m.processMu.Lock()
	defer m.processMu.Unlock()

	if m.processState == ProcessStateStopped {
		return nil // Already stopped
	}

	if m.processState == ProcessStateStopping {
		return fmt.Errorf("service is already stopping")
	}

	m.processState = ProcessStateStopping

	// Stop monitoring
	if m.monitorCancel != nil {
		m.monitorCancel()
		m.monitorWg.Wait()
	}

	// Try graceful shutdown via gRPC first
	if m.client != nil {
		clientCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := m.client.StopDownload(clientCtx); err != nil {
			// Log error but continue with process kill
		}

		// Wait a bit for graceful shutdown
		time.Sleep(2 * time.Second)
	}

	// Kill process if still running
	if m.process != nil {
		if err := m.process.Process.Signal(syscall.SIGTERM); err != nil {
			// If SIGTERM fails, try kill
			m.process.Process.Kill()
		}

		// Wait for process to exit (with timeout)
		done := make(chan error, 1)
		go func() {
			done <- m.process.Wait()
		}()

		select {
		case <-done:
			// Process exited
		case <-time.After(5 * time.Second):
			// Timeout, force kill
			m.process.Process.Kill()
			// Wait for the goroutine to complete (it will send to done channel)
			// Use a second timeout to prevent indefinite wait
			select {
			case <-done:
				// Goroutine completed
			case <-time.After(1 * time.Second):
				// Second timeout - goroutine will eventually complete
			}
		}

		m.process = nil
		m.processPID = 0
	}

	// Close client
	if m.client != nil {
		m.client.Close()
		m.client = nil
	}

	m.processState = ProcessStateStopped
	return nil
}

// IsRunning returns whether the service is running.
func (m *Manager) IsRunning() bool {
	m.processMu.RLock()
	defer m.processMu.RUnlock()

	return m.processState == ProcessStateRunning
}

// GetProcessState returns the current process state.
func (m *Manager) GetProcessState() ProcessState {
	m.processMu.RLock()
	defer m.processMu.RUnlock()

	return m.processState
}

// WaitForReady waits for the gRPC server to be ready.
func (m *Manager) WaitForReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	// Create or get client
	m.clientMu.Lock()
	if m.client == nil {
		m.client = client.NewDownloadClient(m.clientAddress, m.clientVersion)
	}
	client := m.client
	m.clientMu.Unlock()

	// Try to connect
	connectCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := client.Connect(connectCtx); err != nil {
		// Will retry in loop
	}

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("timeout waiting for service to be ready")
			}

			// Check health
			healthCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			resp, err := client.HealthCheck(healthCtx)
			cancel()

			if err == nil && resp != nil {
				if resp.Liveness == proto.HealthStatus_HEALTH_STATUS_HEALTHY {
					return nil // Ready!
				}
			}

			// Retry connection if needed
			if !client.IsConnected() {
				connectCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				client.Connect(connectCtx)
				cancel()
			}
		}
	}
}

// monitorProcess monitors the process and detects crashes.
func (m *Manager) monitorProcess(ctx context.Context) {
	defer m.monitorWg.Done()

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			m.processMu.RLock()
			process := m.process
			state := m.processState
			m.processMu.RUnlock()

			if process == nil || state != ProcessStateRunning {
				continue
			}

			// Check if process is still alive
			if err := process.Process.Signal(syscall.Signal(0)); err != nil {
				// Process has died
				m.processMu.Lock()
				m.processState = ProcessStateError
				m.process = nil
				m.processPID = 0
				m.processMu.Unlock()

				// Clear state (plan files, etc.)
				m.clearStateOnCrash()

				return
			}
		}
	}
}

// clearStateOnCrash clears all state when process crashes.
func (m *Manager) clearStateOnCrash() {
	// Delete plan files
	planFiles := []string{
		filepath.Join(m.planPath, "download_plan_progress.json"),
		filepath.Join(m.planPath, "download_plan.json"),
	}

	for _, file := range planFiles {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			// Log error but continue
		}
	}
}

// Cleanup cleans up any orphaned processes.
func (m *Manager) Cleanup() error {
	// Check for orphaned processes by PID file or process name
	// For now, just ensure our tracked process is stopped
	m.processMu.Lock()
	defer m.processMu.Unlock()

	if m.process != nil {
		// Process still exists, try to stop it
		m.process.Process.Kill()
		m.process.Wait()
		m.process = nil
		m.processPID = 0
		m.processState = ProcessStateStopped
	}

	return nil
}

// GetClient returns the gRPC client (creates if needed).
func (m *Manager) GetClient(ctx context.Context) (*client.DownloadClient, error) {
	m.clientMu.Lock()
	defer m.clientMu.Unlock()

	if m.client == nil {
		m.client = client.NewDownloadClient(m.clientAddress, m.clientVersion)
	}

	if !m.client.IsConnected() {
		if err := m.client.Connect(ctx); err != nil {
			return nil, fmt.Errorf("failed to connect client: %w", err)
		}
	}

	return m.client, nil
}

// OnProcessExit handles process exit (crash or normal shutdown).
func (m *Manager) OnProcessExit() error {
	m.processMu.Lock()
	defer m.processMu.Unlock()

	if m.processState == ProcessStateRunning {
		// Process crashed
		m.processState = ProcessStateError
		m.clearStateOnCrash()
	}

	return nil
}
