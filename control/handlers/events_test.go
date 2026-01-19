package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// threadSafeRecorder wraps httptest.ResponseRecorder with thread-safe access.
// This is needed for testing SSE streams where the handler writes concurrently
// while the test reads from the response body.
type threadSafeRecorder struct {
	recorder *httptest.ResponseRecorder
	mu       sync.RWMutex
}

// newThreadSafeRecorder creates a new thread-safe recorder.
func newThreadSafeRecorder() *threadSafeRecorder {
	return &threadSafeRecorder{
		recorder: httptest.NewRecorder(),
	}
}

// Header returns the response headers (thread-safe).
func (tsr *threadSafeRecorder) Header() http.Header {
	tsr.mu.RLock()
	defer tsr.mu.RUnlock()
	return tsr.recorder.Header()
}

// Write writes data to the response body (thread-safe).
func (tsr *threadSafeRecorder) Write(b []byte) (int, error) {
	tsr.mu.Lock()
	defer tsr.mu.Unlock()
	return tsr.recorder.Write(b)
}

// WriteHeader writes the status code (thread-safe).
func (tsr *threadSafeRecorder) WriteHeader(statusCode int) {
	tsr.mu.Lock()
	defer tsr.mu.Unlock()
	tsr.recorder.WriteHeader(statusCode)
}

// Flush flushes the response (thread-safe).
func (tsr *threadSafeRecorder) Flush() {
	tsr.mu.Lock()
	defer tsr.mu.Unlock()
	tsr.recorder.Flush()
}

// BodyString returns the response body as a string (thread-safe).
func (tsr *threadSafeRecorder) BodyString() string {
	tsr.mu.RLock()
	defer tsr.mu.RUnlock()
	return tsr.recorder.Body.String()
}

func TestStatusStream(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	// Create a cancellable context for the request
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	req := httptest.NewRequest("GET", "/api/status/stream", nil).WithContext(ctx)
	w := httptest.NewRecorder()

	// Start SSE stream in goroutine
	done := make(chan bool)
	go func() {
		handlers.StatusStream(w, req)
		done <- true
	}()

	// Wait a bit for initial data
	time.Sleep(100 * time.Millisecond)

	// Cancel request context to stop stream
	cancel()

	// Wait for stream to finish
	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Error("StatusStream did not complete")
	}

	// Verify headers
	if w.Header().Get("Content-Type") != "text/event-stream" {
		t.Errorf("Expected Content-Type 'text/event-stream', got %s", w.Header().Get("Content-Type"))
	}
	if w.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("Expected Cache-Control 'no-cache', got %s", w.Header().Get("Cache-Control"))
	}
	if w.Header().Get("Connection") != "keep-alive" {
		t.Errorf("Expected Connection 'keep-alive', got %s", w.Header().Get("Connection"))
	}
}

func TestBroadcastStatus(t *testing.T) {
	// Create a test client channel
	clientChan := make(chan []byte, 10)
	globalEventStream.addClient(clientChan)
	defer globalEventStream.removeClient(clientChan)

	status := map[string]interface{}{
		"state": "running",
		"phase": "executing",
	}

	// Broadcast status
	BroadcastStatus(status)

	// Verify message was received
	select {
	case data := <-clientChan:
		if !strings.Contains(string(data), "running") {
			t.Errorf("Expected broadcast to contain 'running', got %s", string(data))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Broadcast message not received")
	}
}

func TestStatusStream_Heartbeat(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/status/stream", nil)
	w := newThreadSafeRecorder()

	// Start SSE stream
	go handlers.StatusStream(w, req)

	// Wait for heartbeat
	time.Sleep(2 * time.Second)

	// Check response body for heartbeat (thread-safe read)
	body := w.BodyString()
	if !strings.Contains(body, "heartbeat") {
		t.Error("Expected heartbeat in SSE stream")
	}
}

func TestStatusStream_InitialData(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	req := httptest.NewRequest("GET", "/api/status/stream", nil)
	w := newThreadSafeRecorder()

	// Start SSE stream
	go handlers.StatusStream(w, req)

	// Wait for initial data
	time.Sleep(500 * time.Millisecond)

	// Check response body for initial status data (thread-safe read)
	body := w.BodyString()
	if !strings.Contains(body, "data:") {
		t.Error("Expected initial data in SSE stream")
	}
}

func TestEventStream_AddRemoveClient(t *testing.T) {
	es := &EventStream{
		clients: make(map[chan []byte]bool),
	}

	client1 := make(chan []byte, 10)
	client2 := make(chan []byte, 10)

	es.addClient(client1)
	es.addClient(client2)

	// Verify clients are added
	if len(es.clients) != 2 {
		t.Errorf("Expected 2 clients, got %d", len(es.clients))
	}

	// Remove client
	es.removeClient(client1)

	// Verify client is removed
	if len(es.clients) != 1 {
		t.Errorf("Expected 1 client after removal, got %d", len(es.clients))
	}

	// Verify channel is closed
	select {
	case _, ok := <-client1:
		if ok {
			t.Error("Client channel should be closed after removal")
		}
	default:
		t.Error("Client channel should be closed")
	}
}

func TestEventStream_Broadcast(t *testing.T) {
	es := &EventStream{
		clients: make(map[chan []byte]bool),
	}

	client1 := make(chan []byte, 10)
	client2 := make(chan []byte, 10)

	es.addClient(client1)
	es.addClient(client2)

	message := []byte("test message")
	es.broadcast(message)

	// Verify both clients received the message
	select {
	case data := <-client1:
		if string(data) != string(message) {
			t.Errorf("Client1 received wrong message: %s", string(data))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Client1 did not receive broadcast")
	}

	select {
	case data := <-client2:
		if string(data) != string(message) {
			t.Errorf("Client2 received wrong message: %s", string(data))
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("Client2 did not receive broadcast")
	}

	es.removeClient(client1)
	es.removeClient(client2)
}

func TestGetStatusData(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	planPath := filepath.Join(tmpDir, "plans")
	logPath := filepath.Join(tmpDir, "logs", "musicdl.log")

	cfg := `version: "1.2"
download:
  client_id: "test_id"
  client_secret: "test_secret"
`
	if err := os.WriteFile(configPath, []byte(cfg), 0644); err != nil {
		t.Fatalf("Failed to create config file: %v", err)
	}

	handlers, err := NewHandlers(configPath, planPath, logPath, time.Now(), "v1.0.0")
	if err != nil {
		t.Fatalf("NewHandlers() failed: %v", err)
	}

	status := handlers.getStatusData()

	if status == nil {
		t.Fatal("getStatusData() returned nil")
	}

	// Verify required fields
	if _, ok := status["state"]; !ok {
		t.Error("Status should contain 'state' field")
	}
	if _, ok := status["statistics"]; !ok {
		t.Error("Status should contain 'statistics' field")
	}
}
