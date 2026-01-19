package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/gorilla/mux"
	"github.com/sv4u/musicdl/control/handlers"
)

// ServerConfig holds configuration for the control platform server.
type ServerConfig struct {
	Port       int
	ConfigPath string
	PlanPath   string
	LogPath    string
	Version    string
}

// Server represents the control platform HTTP server.
type Server struct {
	config     *ServerConfig
	httpServer *http.Server
	router     *mux.Router
	handlers   *handlers.Handlers
	startTime  time.Time
}

// NewServer creates a new control platform server.
func NewServer(config *ServerConfig) (*Server, error) {
	router := mux.NewRouter()

	// Create handlers with server start time
	startTime := time.Now()
	version := config.Version
	if version == "" {
		version = "dev"
	}
	h, err := handlers.NewHandlers(config.ConfigPath, config.PlanPath, config.LogPath, startTime, version)
	if err != nil {
		return nil, fmt.Errorf("failed to create handlers: %w", err)
	}

	server := &Server{
		config:    config,
		router:    router,
		handlers:  h,
		startTime: startTime,
	}

	// Set up routes
	server.setupRoutes()

	// Wrap router with panic recovery middleware
	recoveryHandler := recoveryMiddleware(router)

	// Create HTTP server
	server.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      recoveryHandler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return server, nil
}

// setupRoutes configures all HTTP routes.
func (s *Server) setupRoutes() {
	// API routes
	api := s.router.PathPrefix("/api").Subrouter()

	// Health endpoints
	api.HandleFunc("/health", s.handlers.Health).Methods("GET")
	api.HandleFunc("/health/stats", s.handlers.HealthStats).Methods("GET")

	// Status endpoints
	api.HandleFunc("/status", s.handlers.Status).Methods("GET")
	api.HandleFunc("/status/stream", s.handlers.StatusStream).Methods("GET")
	
	// Plan endpoints
	api.HandleFunc("/plan/items", s.handlers.PlanItems).Methods("GET")

	// Download control endpoints
	api.HandleFunc("/download/start", s.handlers.DownloadStart).Methods("POST")
	api.HandleFunc("/download/stop", s.handlers.DownloadStop).Methods("POST")
	api.HandleFunc("/download/status", s.handlers.DownloadStatus).Methods("GET")
	api.HandleFunc("/download/reset", s.handlers.DownloadReset).Methods("POST")

	// Config management endpoints
	api.HandleFunc("/config", s.handlers.ConfigGet).Methods("GET")
	api.HandleFunc("/config", s.handlers.ConfigPut).Methods("PUT")
	api.HandleFunc("/config/validate", s.handlers.ConfigValidate).Methods("POST")
	api.HandleFunc("/config/digest", s.handlers.ConfigDigest).Methods("GET")

	// Log endpoints
	api.HandleFunc("/logs", s.handlers.Logs).Methods("GET")
	api.HandleFunc("/logs/stream", s.handlers.LogsStream).Methods("GET")

	// Web UI routes
	s.router.HandleFunc("/", s.handlers.Dashboard).Methods("GET")
	s.router.HandleFunc("/status", s.handlers.StatusPage).Methods("GET")
	s.router.HandleFunc("/config", s.handlers.ConfigPage).Methods("GET")
	s.router.HandleFunc("/logs", s.handlers.LogsPage).Methods("GET")

	// Static file serving (for CSS, JS, etc. if needed)
	s.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	log.Printf("Control platform server listening on %s", s.httpServer.Addr)
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server and download service.
func (s *Server) Shutdown(ctx context.Context) error {
	// Stop download service if running
	if err := s.handlers.Shutdown(ctx); err != nil {
		log.Printf("Error shutting down download service: %v", err)
		// Continue with HTTP server shutdown even if download service shutdown fails
	}

	// Shutdown HTTP server
	return s.httpServer.Shutdown(ctx)
}

// recoveryMiddleware wraps an http.Handler to recover from panics and return a proper error response.
func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Log the panic with stack trace
				log.Printf("PANIC: %v\n%s", err, debug.Stack())

				// Try to send a JSON error response
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				
				response := map[string]interface{}{
					"error":   "Internal server error",
					"message": "A panic occurred while processing the request",
				}
				
				// Try to encode response, but if that fails, just write a simple message
				if encErr := json.NewEncoder(w).Encode(response); encErr != nil {
					// If encoding fails, write a simple error message
					w.Write([]byte(`{"error":"Internal server error"}`))
				}
			}
		}()
		next.ServeHTTP(w, r)
	})
}
