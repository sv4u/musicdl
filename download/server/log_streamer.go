package server

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/sv4u/musicdl/download/logging"
	"github.com/sv4u/musicdl/download/proto"
)

// streamLogsFromFile streams log entries from a JSON log file.
func streamLogsFromFile(
	ctx context.Context,
	logPath string,
	req *proto.StreamLogsRequest,
	stream proto.DownloadService_StreamLogsServer,
) error {
	// Open log file
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Parse filters
	filterLevels := make(map[proto.LogLevel]bool)
	if len(req.Levels) > 0 {
		for _, level := range req.Levels {
			filterLevels[level] = true
		}
	}

	var startTime, endTime *time.Time
	if req.StartTime != nil && *req.StartTime > 0 {
		t := time.Unix(*req.StartTime, 0)
		startTime = &t
	}
	if req.EndTime != nil && *req.EndTime > 0 {
		t := time.Unix(*req.EndTime, 0)
		endTime = &t
	}

	searchTerm := req.Search
	follow := req.Follow

	// Read existing log entries
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line := scanner.Text()
		if line == "" {
			continue
		}

		// Parse JSON log entry
		var entry logging.LogEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			// Skip invalid entries
			continue
		}

		// Apply filters
		if !matchesFilters(&entry, filterLevels, startTime, endTime, searchTerm) {
			continue
		}

		// Convert to proto and send
		protoEntry := convertLogEntryToProto(&entry)
		if err := stream.Send(protoEntry); err != nil {
			return fmt.Errorf("failed to send log entry: %w", err)
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading log file: %w", err)
	}

	// If follow mode, tail the file
	if follow {
		return tailLogFile(ctx, logPath, filterLevels, startTime, endTime, searchTerm, stream)
	}

	return nil
}

// tailLogFile tails a log file and streams new entries.
func tailLogFile(
	ctx context.Context,
	logPath string,
	filterLevels map[proto.LogLevel]bool,
	startTime, endTime *time.Time,
	searchTerm string,
	stream proto.DownloadService_StreamLogsServer,
) error {
	// Get current file position
	file, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Seek to end of file
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat log file: %w", err)
	}
	file.Seek(stat.Size(), 0)

	// Watch for new entries
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Read new lines
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}

				// Parse JSON log entry
				var entry logging.LogEntry
				if err := json.Unmarshal([]byte(line), &entry); err != nil {
					continue
				}

				// Apply filters
				if !matchesFilters(&entry, filterLevels, startTime, endTime, searchTerm) {
					continue
				}

				// Convert to proto and send
				protoEntry := convertLogEntryToProto(&entry)
				if err := stream.Send(protoEntry); err != nil {
					return fmt.Errorf("failed to send log entry: %w", err)
				}
			}

			if err := scanner.Err(); err != nil {
				// File may have been rotated, reopen
				file.Close()
				file, err = os.Open(logPath)
				if err != nil {
					// Log error but continue trying
					time.Sleep(1 * time.Second)
					continue
				}
				stat, err := file.Stat()
				if err == nil {
					file.Seek(stat.Size(), 0)
				}
			}
		}
	}
}

// matchesFilters checks if a log entry matches the filters.
func matchesFilters(
	entry *logging.LogEntry,
	filterLevels map[proto.LogLevel]bool,
	startTime, endTime *time.Time,
	searchTerm string,
) bool {
	// Level filter
	if len(filterLevels) > 0 {
		protoLevel := convertLogLevelToProto(string(entry.Level))
		if !filterLevels[protoLevel] {
			return false
		}
	}

	// Time range filter
	if startTime != nil && entry.Timestamp.Before(*startTime) {
		return false
	}
	if endTime != nil && entry.Timestamp.After(*endTime) {
		return false
	}

	// Search filter
	if searchTerm != "" {
		searchLower := toLower(searchTerm)
		if !contains(toLower(entry.Message), searchLower) &&
			!contains(toLower(entry.Service), searchLower) &&
			!contains(toLower(entry.Operation), searchLower) &&
			!contains(toLower(entry.Error), searchLower) {
			return false
		}
	}

	return true
}

// convertLogEntryToProto converts a logging.LogEntry to proto.LogEntry.
func convertLogEntryToProto(entry *logging.LogEntry) *proto.LogEntry {
	protoEntry := &proto.LogEntry{
		Timestamp: entry.Timestamp.Unix(),
		Level:     convertLogLevelToProto(string(entry.Level)),
		Message:   entry.Message,
		Service:   entry.Service,
	}

	if entry.Operation != "" {
		protoEntry.Operation = entry.Operation
	}

	if entry.Error != "" {
		protoEntry.Error = entry.Error
	}

	return protoEntry
}

