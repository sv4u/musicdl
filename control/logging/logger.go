// Package logging re-exports the shared logging package.
// The canonical implementation lives in download/logging.
package logging

import (
	dl "github.com/sv4u/musicdl/download/logging"
)

type LogLevel = dl.LogLevel

const (
	LogLevelDebug = dl.LogLevelDebug
	LogLevelInfo  = dl.LogLevelInfo
	LogLevelWarn  = dl.LogLevelWarn
	LogLevelError = dl.LogLevelError
)

type LogEntry = dl.LogEntry
type Logger = dl.Logger

// NewLogger creates a new structured JSON logger.
func NewLogger(logPath, service string) (*Logger, error) {
	return dl.NewLogger(logPath, service)
}
