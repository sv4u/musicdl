package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogLevel represents the log level.
type LogLevel string

const (
	LogLevelDebug LogLevel = "DEBUG"
	LogLevelInfo  LogLevel = "INFO"
	LogLevelWarn  LogLevel = "WARN"
	LogLevelError LogLevel = "ERROR"
)

// LogEntry represents a structured log entry.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     LogLevel  `json:"level"`
	Message   string    `json:"message"`
	Service   string    `json:"service"`
	Operation string    `json:"operation,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// Logger is a structured JSON logger.
type Logger struct {
	logPath string
	file    *os.File
	mu      sync.Mutex
	service string
}

// NewLogger creates a new structured JSON logger.
// logPath is the path to the log file.
// service is the service name (e.g., "download-service" or "web-server").
func NewLogger(logPath, service string) (*Logger, error) {
	// Ensure log directory exists
	logDir := filepath.Dir(logPath)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Open log file in append mode
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}

	return &Logger{
		logPath: logPath,
		file:    file,
		service: service,
	}, nil
}

// Close closes the log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// log writes a log entry.
func (l *Logger) log(level LogLevel, message, operation string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry := LogEntry{
		Timestamp: time.Now(),
		Level:     level,
		Message:   message,
		Service:   l.service,
	}

	if operation != "" {
		entry.Operation = operation
	}

	if err != nil {
		entry.Error = err.Error()
	}

	// Marshal to JSON
	jsonData, marshalErr := json.Marshal(entry)
	if marshalErr != nil {
		// Fallback to simple format if JSON marshaling fails
		_, _ = fmt.Fprintf(l.file, "{\"timestamp\":\"%s\",\"level\":\"%s\",\"message\":\"%s\",\"service\":\"%s\"}\n",
			time.Now().Format(time.RFC3339), level, message, l.service)
		return
	}

	// Write JSON line
	_, _ = fmt.Fprintln(l.file, string(jsonData))
}

// Debug logs a debug message.
func (l *Logger) Debug(message string) {
	l.log(LogLevelDebug, message, "", nil)
}

// Debugf logs a formatted debug message.
func (l *Logger) Debugf(format string, args ...interface{}) {
	l.Debug(fmt.Sprintf(format, args...))
}

// DebugWithOperation logs a debug message with operation context.
func (l *Logger) DebugWithOperation(operation, message string) {
	l.log(LogLevelDebug, message, operation, nil)
}

// Info logs an info message.
func (l *Logger) Info(message string) {
	l.log(LogLevelInfo, message, "", nil)
}

// Infof logs a formatted info message.
func (l *Logger) Infof(format string, args ...interface{}) {
	l.Info(fmt.Sprintf(format, args...))
}

// InfoWithOperation logs an info message with operation context.
func (l *Logger) InfoWithOperation(operation, message string) {
	l.log(LogLevelInfo, message, operation, nil)
}

// Warn logs a warning message.
func (l *Logger) Warn(message string) {
	l.log(LogLevelWarn, message, "", nil)
}

// Warnf logs a formatted warning message.
func (l *Logger) Warnf(format string, args ...interface{}) {
	l.Warn(fmt.Sprintf(format, args...))
}

// WarnWithOperation logs a warning message with operation context.
func (l *Logger) WarnWithOperation(operation, message string) {
	l.log(LogLevelWarn, message, operation, nil)
}

// Error logs an error message.
func (l *Logger) Error(message string, err error) {
	l.log(LogLevelError, message, "", err)
}

// Errorf logs a formatted error message.
func (l *Logger) Errorf(format string, args ...interface{}) {
	l.Error(fmt.Sprintf(format, args...), nil)
}

// ErrorWithOperation logs an error message with operation context.
func (l *Logger) ErrorWithOperation(operation, message string, err error) {
	l.log(LogLevelError, message, operation, err)
}
