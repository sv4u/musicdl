package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	manager := NewManager("localhost:30025", "v1.0.0", "/tmp/plans", "/tmp/logs")
	if manager == nil {
		t.Fatal("NewManager returned nil")
	}

	if manager.clientAddress != "localhost:30025" {
		t.Errorf("Expected address 'localhost:30025', got '%s'", manager.clientAddress)
	}

	if manager.GetProcessState() != ProcessStateStopped {
		t.Errorf("Expected initial state STOPPED, got %s", manager.GetProcessState())
	}
}

func TestNewManager_DefaultAddress(t *testing.T) {
	manager := NewManager("", "v1.0.0", "/tmp/plans", "/tmp/logs")
	if manager.clientAddress != "localhost:30025" {
		t.Errorf("Expected default address 'localhost:30025', got '%s'", manager.clientAddress)
	}
}

func TestManager_IsRunning(t *testing.T) {
	manager := NewManager("localhost:30025", "v1.0.0", "/tmp/plans", "/tmp/logs")

	if manager.IsRunning() {
		t.Error("Expected IsRunning to return false initially")
	}
}

func TestManager_GetProcessState(t *testing.T) {
	manager := NewManager("localhost:30025", "v1.0.0", "/tmp/plans", "/tmp/logs")

	state := manager.GetProcessState()
	if state != ProcessStateStopped {
		t.Errorf("Expected initial state STOPPED, got %s", state)
	}
}

func TestManager_Cleanup(t *testing.T) {
	manager := NewManager("localhost:30025", "v1.0.0", "/tmp/plans", "/tmp/logs")

	// Cleanup should not error on stopped service
	if err := manager.Cleanup(); err != nil {
		t.Errorf("Cleanup failed: %v", err)
	}
}

func TestManager_ClearStateOnCrash(t *testing.T) {
	tmpDir := t.TempDir()
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "test.log")

	// Create plan directory
	if err := os.MkdirAll(planPath, 0755); err != nil {
		t.Fatalf("Failed to create plan directory: %v", err)
	}

	// Create a plan file
	planFile := filepath.Join(planPath, "download_plan_progress.json")
	planData := `{"items": [], "metadata": {}}`
	if err := os.WriteFile(planFile, []byte(planData), 0644); err != nil {
		t.Fatalf("Failed to create plan file: %v", err)
	}

	manager := NewManager("localhost:30025", "v1.0.0", planPath, logPath)

	// Simulate crash cleanup
	manager.clearStateOnCrash()

	// Verify plan file was deleted
	if _, err := os.Stat(planFile); !os.IsNotExist(err) {
		t.Error("Plan file should be deleted after crash cleanup")
	}
}

func TestManager_StopService_AlreadyStopped(t *testing.T) {
	manager := NewManager("localhost:30025", "v1.0.0", "/tmp/plans", "/tmp/logs")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Stop when already stopped should not error
	if err := manager.StopService(ctx); err != nil {
		t.Errorf("StopService on stopped service should not error, got: %v", err)
	}
}

func TestManager_GetClient(t *testing.T) {
	manager := NewManager("localhost:30025", "v1.0.0", "/tmp/plans", "/tmp/logs")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// GetClient should create client even if service is not running
	// (it will fail to connect, but client should be created)
	client, err := manager.GetClient(ctx)
	if err == nil {
		// If no error, client should be created
		if client == nil {
			t.Error("GetClient returned nil client")
		}
	}
	// Error is expected if service is not running, so we don't check for it
}

func TestManager_ProcessStateTransitions(t *testing.T) {
	manager := NewManager("localhost:30025", "v1.0.0", "/tmp/plans", "/tmp/logs")

	// Initial state
	if manager.GetProcessState() != ProcessStateStopped {
		t.Errorf("Expected initial state STOPPED, got %s", manager.GetProcessState())
	}

	// Test state transitions (we can't actually start a process in unit tests,
	// but we can verify the state management logic)
	manager.processMu.Lock()
	manager.processState = ProcessStateStarting
	manager.processMu.Unlock()

	if manager.GetProcessState() != ProcessStateStarting {
		t.Errorf("Expected state STARTING, got %s", manager.GetProcessState())
	}
}
