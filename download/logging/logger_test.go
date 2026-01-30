package logging

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"
	"time"
)

// isRaceDetectorEnabled checks if the race detector is enabled at runtime
func isRaceDetectorEnabled() bool {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return false
	}
	for _, setting := range info.Settings {
		if setting.Key == "-race" {
			return true
		}
	}
	return false
}

func TestNewLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Verify file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatal("Log file was not created")
	}
}

func TestLoggerLogLevels(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Test all log levels
	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warning message")
	logger.Error("error message", nil)

	// Read and verify logs
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	levels := []LogLevel{LogLevelDebug, LogLevelInfo, LogLevelWarn, LogLevelError}
	index := 0

	for scanner.Scan() {
		if index >= len(levels) {
			t.Fatalf("More log entries than expected")
		}

		line := scanner.Text()
		var entry LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			t.Fatalf("Failed to unmarshal log entry: %v", err)
		}

		if entry.Level != levels[index] {
			t.Errorf("Expected level %s, got %s", levels[index], entry.Level)
		}

		if entry.Service != "test-service" {
			t.Errorf("Expected service 'test-service', got '%s'", entry.Service)
		}

		if entry.Timestamp.IsZero() {
			t.Error("Timestamp is zero")
		}

		index++
	}

	if index != len(levels) {
		t.Errorf("Expected %d log entries, got %d", len(levels), index)
	}
}

func TestLoggerWithOperation(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	operation := "test-operation"
	message := "test message"
	logger.InfoWithOperation(operation, message)

	// Read and verify
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("No log entry found")
	}

	var entry LogEntry
	if err := json.Unmarshal([]byte(scanner.Text()), &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	if entry.Operation != operation {
		t.Errorf("Expected operation '%s', got '%s'", operation, entry.Operation)
	}

	if entry.Message != message {
		t.Errorf("Expected message '%s', got '%s'", message, entry.Message)
	}
}

func TestLoggerWithError(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	testErr := &os.PathError{
		Op:   "open",
		Path: "/nonexistent",
		Err:  os.ErrNotExist,
	}

	logger.Error("failed to open file", testErr)

	// Read and verify
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("No log entry found")
	}

	var entry LogEntry
	if err := json.Unmarshal([]byte(scanner.Text()), &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	if entry.Level != LogLevelError {
		t.Errorf("Expected level ERROR, got %s", entry.Level)
	}

	if entry.Error == "" {
		t.Error("Error field is empty")
	}

	if !strings.Contains(entry.Error, "open") {
		t.Errorf("Error message should contain 'open', got '%s'", entry.Error)
	}
}

func TestLoggerFormattedMessages(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	logger.Debugf("debug: %s", "value")
	logger.Infof("info: %d items", 42)
	logger.Warnf("warning: %v", true)
	logger.Errorf("error: %s", "failed")

	// Read and verify
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	expectedMessages := []string{
		"debug: value",
		"info: 42 items",
		"warning: true",
		"error: failed",
	}
	index := 0

	for scanner.Scan() {
		if index >= len(expectedMessages) {
			t.Fatalf("More log entries than expected")
		}

		var entry LogEntry
		if err := json.Unmarshal([]byte(scanner.Text()), &entry); err != nil {
			t.Fatalf("Failed to unmarshal log entry: %v", err)
		}

		if entry.Message != expectedMessages[index] {
			t.Errorf("Expected message '%s', got '%s'", expectedMessages[index], entry.Message)
		}

		index++
	}

	if index != len(expectedMessages) {
		t.Errorf("Expected %d log entries, got %d", len(expectedMessages), index)
	}
}

func TestLoggerConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	// Write logs concurrently
	// Reduce memory usage when running with race detector
	done := make(chan bool)
	numGoroutines := 10
	logsPerGoroutine := 10
	if isRaceDetectorEnabled() {
		numGoroutines = 5
		logsPerGoroutine = 5
	}

	for i := 0; i < numGoroutines; i++ {
		go func(id int) {
			for j := 0; j < logsPerGoroutine; j++ {
				logger.Infof("goroutine %d: log %d", id, j)
			}
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// Verify all logs were written
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal([]byte(scanner.Text()), &entry); err != nil {
			t.Fatalf("Failed to unmarshal log entry: %v", err)
		}
		count++
	}

	expectedCount := numGoroutines * logsPerGoroutine
	if count != expectedCount {
		t.Errorf("Expected %d log entries, got %d", expectedCount, count)
	}
}

func TestLoggerTimestampFormat(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer func() { _ = logger.Close() }()

	before := time.Now()
	logger.Info("test message")
	after := time.Now()

	// Read and verify timestamp
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	if !scanner.Scan() {
		t.Fatal("No log entry found")
	}

	var entry LogEntry
	if err := json.Unmarshal([]byte(scanner.Text()), &entry); err != nil {
		t.Fatalf("Failed to unmarshal log entry: %v", err)
	}

	if entry.Timestamp.Before(before) || entry.Timestamp.After(after) {
		t.Errorf("Timestamp %v is not within expected range [%v, %v]",
			entry.Timestamp, before, after)
	}
}

func TestLoggerAppendMode(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create first logger and write
	logger1, err := NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	logger1.Info("first message")
	_ = logger1.Close()

	// Create second logger and write (should append)
	logger2, err := NewLogger(logPath, "test-service")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	logger2.Info("second message")
	_ = logger2.Close()

	// Verify both messages are present
	file, err := os.Open(logPath)
	if err != nil {
		t.Fatalf("Failed to open log file: %v", err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	messages := []string{}
	for scanner.Scan() {
		var entry LogEntry
		if err := json.Unmarshal([]byte(scanner.Text()), &entry); err != nil {
			t.Fatalf("Failed to unmarshal log entry: %v", err)
		}
		messages = append(messages, entry.Message)
	}

	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}

	if messages[0] != "first message" {
		t.Errorf("Expected first message 'first message', got '%s'", messages[0])
	}

	if messages[1] != "second message" {
		t.Errorf("Expected second message 'second message', got '%s'", messages[1])
	}
}
