package handlers

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/sv4u/musicdl/control/client"
	"github.com/sv4u/musicdl/control/config"
	"github.com/sv4u/musicdl/control/service"
)

// Handlers holds all HTTP handlers for the control platform.
type Handlers struct {
	configPath string
	planPath   string
	logPath    string
	startTime  time.Time

	// Configuration manager
	configManager *config.ConfigManager
	configMu      sync.RWMutex

	// Service manager (process lifecycle)
	serviceManager *service.Manager
	serviceMu      sync.RWMutex
}

// NewHandlers creates a new handlers instance.
func NewHandlers(configPath, planPath, logPath string, startTime time.Time, version string) (*Handlers, error) {
	// Validate paths exist or can be created
	if err := validatePaths(configPath, planPath, logPath); err != nil {
		return nil, fmt.Errorf("path validation failed: %w", err)
	}

	// Create configuration manager
	configManager, err := config.NewConfigManager(configPath, planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create config manager: %w", err)
	}

	// Create service manager
	clientAddress := "localhost:30025"
	clientVersion := version
	if clientVersion == "" {
		clientVersion = "dev"
	}
	serviceManager := service.NewManager(clientAddress, clientVersion, planPath, logPath)

	// Cleanup any orphaned processes on startup
	if err := serviceManager.Cleanup(); err != nil {
		log.Printf("Warning: failed to cleanup orphaned processes: %v", err)
	}

	return &Handlers{
		configPath:    configPath,
		planPath:      planPath,
		logPath:       logPath,
		startTime:     startTime,
		configManager: configManager,
		serviceManager: serviceManager,
	}, nil
}

// getDownloadClient returns the gRPC client for the download service.
func (h *Handlers) getDownloadClient(ctx context.Context) (*client.DownloadClient, error) {
	h.serviceMu.RLock()
	svcManager := h.serviceManager
	h.serviceMu.RUnlock()

	return svcManager.GetClient(ctx)
}

// validatePaths ensures required directories exist.
func validatePaths(configPath, planPath, logPath string) error {
	// Check config file exists
	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("config file not found: %s", configPath)
	}

	// Ensure plan directory exists
	if err := os.MkdirAll(planPath, 0755); err != nil {
		return fmt.Errorf("failed to create plan directory: %w", err)
	}

	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("failed to create log directory: %w", err)
	}

	return nil
}

// logError logs an error with context.
func (h *Handlers) logError(operation string, err error) {
	log.Printf("Error in %s: %v", operation, err)
}
