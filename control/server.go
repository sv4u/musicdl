package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
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
	h, err := handlers.NewHandlers(config.ConfigPath, config.PlanPath, config.LogPath, startTime)
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

	// Create HTTP server
	server.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", config.Port),
		Handler:      router,
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

	// Download control endpoints
	api.HandleFunc("/download/start", s.handlers.DownloadStart).Methods("POST")
	api.HandleFunc("/download/stop", s.handlers.DownloadStop).Methods("POST")
	api.HandleFunc("/download/status", s.handlers.DownloadStatus).Methods("GET")

	// Config management endpoints
	api.HandleFunc("/config", s.handlers.ConfigGet).Methods("GET")
	api.HandleFunc("/config", s.handlers.ConfigPut).Methods("PUT")
	api.HandleFunc("/config/validate", s.handlers.ConfigValidate).Methods("POST")

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

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}
