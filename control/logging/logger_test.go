package logging

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestNewLogger(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logPath, "web-server")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Verify file was created
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Fatal("Log file was not created")
	}
}

func TestLoggerLogLevels(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logPath, "web-server")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

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
	defer file.Close()

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

		if entry.Service != "web-server" {
			t.Errorf("Expected service 'web-server', got '%s'", entry.Service)
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

func TestLoggerConcurrency(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	logger, err := NewLogger(logPath, "web-server")
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Close()

	// Write logs concurrently
	done := make(chan bool)
	numGoroutines := 10
	logsPerGoroutine := 10

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
	defer file.Close()

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
