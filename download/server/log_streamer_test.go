package server

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/sv4u/musicdl/download/logging"
	"github.com/sv4u/musicdl/download/proto"
)

func TestStreamLogsFromFile_Basic(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create logger and write some entries
	logger, err := logging.NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Info("Test message 1")
	logger.Warn("Test warning")
	logger.Error("Test error", nil)
	logger.Debug("Test debug")

	// Create a mock stream
	mockStream := &mockLogStream{
		ctx:    context.Background(),
		entries: make([]*proto.LogEntry, 0),
	}

	// Create request
	req := &proto.StreamLogsRequest{
		Follow: false,
	}

	// Stream logs
	err = streamLogsFromFile(context.Background(), logPath, req, mockStream)
	if err != nil {
		t.Fatalf("streamLogsFromFile failed: %v", err)
	}

	// Verify entries
	if len(mockStream.entries) != 4 {
		t.Errorf("Expected 4 log entries, got %d", len(mockStream.entries))
	}
}

func TestStreamLogsFromFile_LevelFilter(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create logger and write entries
	logger, err := logging.NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Info("Info message")
	logger.Warn("Warning message")
	logger.Error("Error message", nil)
	logger.Debug("Debug message")

	// Create mock stream
	mockStream := &mockLogStream{
		ctx:    context.Background(),
		entries: make([]*proto.LogEntry, 0),
	}

	// Filter by level (only ERROR)
	req := &proto.StreamLogsRequest{
		Levels: []proto.LogLevel{proto.LogLevel_LOG_LEVEL_ERROR},
		Follow: false,
	}

	err = streamLogsFromFile(context.Background(), logPath, req, mockStream)
	if err != nil {
		t.Fatalf("streamLogsFromFile failed: %v", err)
	}

	// Should only get ERROR entries
	if len(mockStream.entries) != 1 {
		t.Errorf("Expected 1 log entry (ERROR), got %d", len(mockStream.entries))
	}

	if mockStream.entries[0].Level != proto.LogLevel_LOG_LEVEL_ERROR {
		t.Errorf("Expected ERROR level, got %v", mockStream.entries[0].Level)
	}
}

func TestStreamLogsFromFile_TimeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create logger
	logger, err := logging.NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write entry at specific time
	logger.Info("Message 1")

	// Wait a bit
	time.Sleep(100 * time.Millisecond)
	startTime := time.Now()

	time.Sleep(100 * time.Millisecond)
	logger.Info("Message 2")

	// Create mock stream
	mockStream := &mockLogStream{
		ctx:    context.Background(),
		entries: make([]*proto.LogEntry, 0),
	}

	// Filter by start time
	startTimeUnix := startTime.Unix()
	req := &proto.StreamLogsRequest{
		StartTime: &startTimeUnix,
		Follow:    false,
	}

	err = streamLogsFromFile(context.Background(), logPath, req, mockStream)
	if err != nil {
		t.Fatalf("streamLogsFromFile failed: %v", err)
	}

	// Should only get entries after startTime
	if len(mockStream.entries) < 1 {
		t.Errorf("Expected at least 1 log entry after start time, got %d", len(mockStream.entries))
	}
}

func TestStreamLogsFromFile_SearchFilter(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create logger
	logger, err := logging.NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	logger.Info("This is a test message")
	logger.Info("Another message")
	logger.Info("Test keyword here")

	// Create mock stream
	mockStream := &mockLogStream{
		ctx:    context.Background(),
		entries: make([]*proto.LogEntry, 0),
	}

	// Filter by search term
	req := &proto.StreamLogsRequest{
		Search: "keyword",
		Follow: false,
	}

	err = streamLogsFromFile(context.Background(), logPath, req, mockStream)
	if err != nil {
		t.Fatalf("streamLogsFromFile failed: %v", err)
	}

	// Should only get entries containing "keyword"
	if len(mockStream.entries) != 1 {
		t.Errorf("Expected 1 log entry with 'keyword', got %d", len(mockStream.entries))
	}

	if !contains(mockStream.entries[0].Message, "keyword") {
		t.Errorf("Expected entry to contain 'keyword', got: %s", mockStream.entries[0].Message)
	}
}

func TestMatchesFilters(t *testing.T) {
	entry := &logging.LogEntry{
		Timestamp: time.Now(),
		Level:     logging.LogLevelInfo,
		Message:   "Test message",
		Service:   "test-service",
		Operation: "test-op",
	}

	// Test level filter
	filterLevels := map[proto.LogLevel]bool{
		proto.LogLevel_LOG_LEVEL_INFO: true,
	}
	if !matchesFilters(entry, filterLevels, nil, nil, "") {
		t.Error("Entry should match INFO level filter")
	}

	filterLevels = map[proto.LogLevel]bool{
		proto.LogLevel_LOG_LEVEL_ERROR: true,
	}
	if matchesFilters(entry, filterLevels, nil, nil, "") {
		t.Error("Entry should not match ERROR level filter")
	}

	// Test time filter
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now().Add(1 * time.Hour)
	if !matchesFilters(entry, nil, &startTime, &endTime, "") {
		t.Error("Entry should match time range filter")
	}

	// Test search filter
	if !matchesFilters(entry, nil, nil, nil, "Test") {
		t.Error("Entry should match search filter")
	}

	if matchesFilters(entry, nil, nil, nil, "NotFound") {
		t.Error("Entry should not match search filter")
	}
}

func TestConvertLogEntryToProto(t *testing.T) {
	entry := &logging.LogEntry{
		Timestamp: time.Unix(1234567890, 0),
		Level:     logging.LogLevelInfo,
		Message:   "Test message",
		Service:   "test-service",
		Operation: "test-op",
		Error:     "test error",
	}

	protoEntry := convertLogEntryToProto(entry)

	if protoEntry.Timestamp != 1234567890 {
		t.Errorf("Expected timestamp 1234567890, got %d", protoEntry.Timestamp)
	}

	if protoEntry.Level != proto.LogLevel_LOG_LEVEL_INFO {
		t.Errorf("Expected INFO level, got %v", protoEntry.Level)
	}

	if protoEntry.Message != "Test message" {
		t.Errorf("Expected message 'Test message', got '%s'", protoEntry.Message)
	}

	if protoEntry.Service != "test-service" {
		t.Errorf("Expected service 'test-service', got '%s'", protoEntry.Service)
	}

	if protoEntry.Operation != "test-op" {
		t.Errorf("Expected operation 'test-op', got '%s'", protoEntry.Operation)
	}

	if protoEntry.Error != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", protoEntry.Error)
	}
}

// mockLogStream is a mock implementation of DownloadService_StreamLogsServer
type mockLogStream struct {
	proto.DownloadService_StreamLogsServer
	ctx     context.Context
	entries []*proto.LogEntry
}

func (m *mockLogStream) Context() context.Context {
	return m.ctx
}

func (m *mockLogStream) Send(entry *proto.LogEntry) error {
	m.entries = append(m.entries, entry)
	return nil
}
