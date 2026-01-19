package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/sv4u/musicdl/download"
	"github.com/sv4u/musicdl/download/audio"
	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/metadata"
	"github.com/sv4u/musicdl/download/server"
	"github.com/sv4u/musicdl/download/spotify"
)

var (
	// Version is set at build time via ldflags
	// Example: go build -ldflags="-X main.Version=v1.2.3"
	Version = "dev"
)

const (
	// Default port for the control platform
	defaultPort = 8080
	// Default config path
	defaultConfigPath = "/scripts/config.yaml"
	// Default plan path
	defaultPlanPath = "/var/lib/musicdl/plans"
	// Default log path
	defaultLogPath = "/var/lib/musicdl/logs/musicdl.log"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	// Handle version command
	if command == "version" || command == "--version" || command == "-v" {
		fmt.Printf("musicdl version %s\n", Version)
		os.Exit(0)
	}

	switch command {
	case "serve":
		serveCommand()
	case "download":
		downloadCommand()
	case "download-service":
		downloadServiceCommand()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `musicdl - Control platform for music downloads

USAGE:
    musicdl <command> [flags]

COMMANDS:
    serve           Start the control platform web server
    download        Run download service (one-shot or daemon mode)
    download-service  Start the download service gRPC server
    version         Show version information

FLAGS:
    -h, --help    Show this help message

EXAMPLES:
    musicdl serve
    musicdl download --config config.yaml
    musicdl download --config config.yaml --daemon

For more information, see https://github.com/sv4u/musicdl
`)
}

func serveCommand() {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", defaultPort, "HTTP server port")
	configPath := fs.String("config", defaultConfigPath, "Path to configuration file")
	planPath := fs.String("plan-path", defaultPlanPath, "Path to plan files directory")
	logPath := fs.String("log-path", defaultLogPath, "Path to log file")

	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Error parsing flags: %v", err)
	}

	// Create server configuration
	config := &ServerConfig{
		Port:       *port,
		ConfigPath: *configPath,
		PlanPath:   *planPath,
		LogPath:    *logPath,
		Version:    Version,
	}

	// Create and start server
	server, err := NewServer(config)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Set up graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		log.Printf("musicdl version %s", Version)
		log.Printf("Starting control platform server on port %d", *port)
		if err := server.Start(); err != nil {
			errChan <- err
		}
	}()

	// Wait for signal or error
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v, shutting down gracefully...", sig)
		shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Error during shutdown: %v", err)
		} else {
			log.Println("Server shut down gracefully")
		}
	case err := <-errChan:
		log.Fatalf("Server error: %v", err)
	}
}

func downloadCommand() {
	fs := flag.NewFlagSet("download", flag.ExitOnError)
	configPath := fs.String("config", defaultConfigPath, "Path to configuration file")
	planPath := fs.String("plan-path", defaultPlanPath, "Path to plan files directory")
	logPath := fs.String("log-path", defaultLogPath, "Path to log file")
	daemon := fs.Bool("daemon", false, "Run as long-running daemon (default: one-shot mode)")

	if err := fs.Parse(os.Args[2:]); err != nil {
		log.Fatalf("Error parsing flags: %v", err)
	}

	// Validate paths
	if err := validateDownloadPaths(*configPath, *planPath, *logPath); err != nil {
		log.Fatalf("Path validation failed: %v", err)
	}

	// Load config
	cfg, err := loadDownloadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create service
	service, err := createDownloadService(cfg, *planPath)
	if err != nil {
		log.Fatalf("Failed to create download service: %v", err)
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start download service
	log.Printf("musicdl version %s", Version)
	log.Printf("Starting download service...")
	if err := service.Start(ctx); err != nil {
		log.Fatalf("Failed to start download service: %v", err)
	}

	if *daemon {
		// Daemon mode: run until interrupted
		log.Printf("Running in daemon mode. Press Ctrl+C to stop.")
		sig := <-sigChan
		log.Printf("Received signal: %v, stopping download service...", sig)
		if err := service.Stop(); err != nil {
			log.Printf("Error stopping service: %v", err)
			os.Exit(1)
		}
		log.Println("Download service stopped")
	} else {
		// One-shot mode: wait for completion
		log.Printf("Running in one-shot mode. Waiting for completion...")

		// Wait for completion or signal
		done := make(chan bool, 1)
		go func() {
			service.WaitForCompletion()
			done <- true
		}()

		select {
		case <-done:
			// Service completed
			status := service.GetStatus()
			phase, ok := status["phase"].(string)
			if !ok {
				phase = "unknown"
				log.Printf("WARN: invalid phase type in status: %T", status["phase"])
			}

			if phase == "completed" {
				if stats, ok := status["plan_stats"].(map[string]interface{}); ok {
					completed := stats["completed"]
					failed := stats["failed"]
					total := stats["total"]
					log.Printf("Download completed: %v successful, %v failed, %v total", completed, failed, total)

					// Print cache statistics
					printCacheStats(service)

					// Exit with error code if any downloads failed
					if failedInt, ok := failed.(int); ok && failedInt > 0 {
						os.Exit(1)
					}
				} else {
					log.Println("Download completed")
				}
			} else if phase == "error" {
				if errMsg, ok := status["error"].(string); ok {
					log.Printf("Download failed: %s", errMsg)
				} else {
					log.Println("Download failed")
				}
				os.Exit(1)
			}
		case sig := <-sigChan:
			log.Printf("Received signal: %v, stopping download service...", sig)
			if err := service.Stop(); err != nil {
				log.Printf("Error stopping service: %v", err)
				os.Exit(1)
			}
			log.Println("Download service stopped")
			os.Exit(130) // Exit code 130 for SIGINT
		}
	}
}

// validateDownloadPaths ensures required directories exist.
func validateDownloadPaths(configPath, planPath, logPath string) error {
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

// loadDownloadConfig loads configuration from file.
func loadDownloadConfig(configPath string) (*config.MusicDLConfig, error) {
	return config.LoadConfig(configPath)
}

// createDownloadService creates a download service with the given configuration.
func createDownloadService(cfg *config.MusicDLConfig, planPath string) (*download.Service, error) {
	// Create Spotify client
	spotifyConfig := &spotify.Config{
		ClientID:          cfg.Download.ClientID,
		ClientSecret:      cfg.Download.ClientSecret,
		CacheMaxSize:      cfg.Download.CacheMaxSize,
		CacheTTL:          cfg.Download.CacheTTL,
		RateLimitEnabled:  cfg.Download.SpotifyRateLimitEnabled,
		RateLimitRequests: cfg.Download.SpotifyRateLimitRequests,
		RateLimitWindow:   cfg.Download.SpotifyRateLimitWindow,
		MaxRetries:        cfg.Download.SpotifyMaxRetries,
		RetryBaseDelay:    cfg.Download.SpotifyRetryBaseDelay,
		RetryMaxDelay:     cfg.Download.SpotifyRetryMaxDelay,
	}
	spotifyClient, err := spotify.NewSpotifyClient(spotifyConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Spotify client: %w", err)
	}

	// Create audio provider
	audioConfig := &audio.Config{
		OutputFormat:   cfg.Download.Format,
		Bitrate:        cfg.Download.Bitrate,
		AudioProviders: cfg.Download.AudioProviders,
		CacheMaxSize:   cfg.Download.AudioSearchCacheMaxSize,
		CacheTTL:       cfg.Download.AudioSearchCacheTTL,
	}
	audioProvider, err := audio.NewProvider(audioConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create audio provider: %w", err)
	}

	// Create metadata embedder
	metadataEmbedder := metadata.NewEmbedder()

	// Create download service
	service, err := download.NewService(cfg, spotifyClient, audioProvider, metadataEmbedder, planPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create download service: %w", err)
	}

	return service, nil
}

// printCacheStats prints cache statistics for all caches.
func printCacheStats(service *download.Service) {
	stats := service.GetCacheStats()

	log.Println("")
	log.Println("================================================================================")
	log.Println("CACHE STATISTICS")
	log.Println("================================================================================")

	// Spotify API cache
	log.Println("Spotify API Cache:")
	log.Printf("  Size: %d/%d entries", stats.Spotify.Size, stats.Spotify.MaxSize)
	ttlHours := stats.SpotifyTTL / 3600
	log.Printf("  TTL: %ds (%dh)", stats.SpotifyTTL, ttlHours)
	log.Printf("  Hits: %d, Misses: %d", stats.Spotify.Hits, stats.Spotify.Misses)
	log.Printf("  Hit Rate: %.2f%%", stats.Spotify.HitRate*100)
	if stats.Spotify.Evictions > 0 {
		log.Printf("  Evictions: %d", stats.Spotify.Evictions)
	}

	// Audio search cache
	log.Println("Audio Search Cache:")
	log.Printf("  Size: %d/%d entries", stats.AudioSearch.Size, stats.AudioSearch.MaxSize)
	audioTTLHours := stats.AudioSearchTTL / 3600
	log.Printf("  TTL: %ds (%dh)", stats.AudioSearchTTL, audioTTLHours)
	log.Printf("  Hits: %d, Misses: %d", stats.AudioSearch.Hits, stats.AudioSearch.Misses)
	log.Printf("  Hit Rate: %.2f%%", stats.AudioSearch.HitRate*100)
	if stats.AudioSearch.Evictions > 0 {
		log.Printf("  Evictions: %d", stats.AudioSearch.Evictions)
	}

	// File existence cache
	if stats.FileExistence != nil {
		log.Println("File Existence Cache:")
		if size, ok := stats.FileExistence["size"].(int); ok {
			if maxSize, ok := stats.FileExistence["max_size"].(int); ok {
				log.Printf("  Size: %d/%d entries", size, maxSize)
			} else {
				log.Printf("  Size: %d entries", size)
			}
		}
		// File existence cache doesn't track hits/misses (simple map)
		log.Println("  Note: Simple map cache (no hit/miss tracking)")
	}

	log.Println("================================================================================")
	log.Println("")
}

// downloadServiceCommand starts the download service gRPC server.
func downloadServiceCommand() {
	fs := flag.NewFlagSet("download-service", flag.ExitOnError)
	planPath := fs.String("plan-path", defaultPlanPath, "Path to plan directory")
	logPath := fs.String("log-path", defaultLogPath, "Path to log file")
	port := fs.String("port", "30025", "gRPC server port")
	fs.Parse(os.Args[2:])

	log.Printf("musicdl version %s", Version)
	log.Printf("Starting download service gRPC server on port %s", *port)
	log.Printf("Plan path: %s", *planPath)
	log.Printf("Log path: %s", *logPath)

	// Import server package
	// Note: We need to import the server package from download/server
	// Since we're in control package, we'll need to call it differently
	// For now, we'll create a wrapper or move the server code
	// Actually, the server should be in download/server, so we can call it directly
	// But we need to import it properly

	// For now, let's create a simple implementation that calls the server
	// We'll need to import the server package
	if err := runDownloadService(*port, *planPath, *logPath); err != nil {
		log.Fatalf("Failed to start download service: %v", err)
	}
}

// runDownloadService runs the download service gRPC server.
func runDownloadService(port, planPath, logPath string) error {
	return server.RunServer(port, planPath, logPath, Version)
}
