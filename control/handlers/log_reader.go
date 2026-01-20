package handlers

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// LogEntry represents a parsed log entry.
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Fields    map[string]string `json:"fields,omitempty"`
	Raw       string    `json:"raw"`
}

// LogReader reads and parses log files.
type LogReader struct {
	logPath string
}

// NewLogReader creates a new log reader.
func NewLogReader(logPath string) *LogReader {
	return &LogReader{
		logPath: logPath,
	}
}

// ReadLogs reads logs from the file with optional filtering.
func (lr *LogReader) ReadLogs(level, searchQuery, startTimeStr, endTimeStr string, maxLines int) ([]LogEntry, error) {
	file, err := os.Open(lr.logPath)
	if err != nil {
		if os.IsNotExist(err) {
			return []LogEntry{}, nil
		}
		return nil, fmt.Errorf("failed to open log file: %w", err)
	}
	defer file.Close()

	// Parse time filters
	var startTime, endTime *time.Time
	if startTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = &t
		}
	}
	if endTimeStr != "" {
		if t, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = &t
		}
	}

	// Read file from end (most recent first)
	// For large files, we use a sliding window to avoid loading entire file into memory
	entries := make([]LogEntry, 0)
	scanner := bufio.NewScanner(file)
	
	// Use a ring buffer to store only the last N lines we need
	// We'll read more than maxLines to account for filtering, but limit to prevent memory issues
	readLimit := maxLines * 10 // Read up to 10x maxLines to account for filtering
	if readLimit > 10000 {
		readLimit = 10000 // Cap at 10k lines to prevent excessive memory usage
	}
	
	lines := make([]string, 0, readLimit)
	lineCount := 0
	
	// Read lines, keeping only the most recent ones
	for scanner.Scan() {
		line := scanner.Text()
		lineCount++
		
		// Use sliding window: keep only the last readLimit lines
		if len(lines) >= readLimit {
			// Remove oldest line (shift left)
			copy(lines, lines[1:])
			lines = lines[:readLimit-1]
		}
		lines = append(lines, line)
	}
	
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	// Process lines in reverse (most recent first)
	for i := len(lines) - 1; i >= 0 && len(entries) < maxLines; i-- {
		line := lines[i]
		if line == "" {
			continue
		}

		entry := lr.parseLogLine(line)
		if entry == nil {
			continue
		}

		// Apply filters
		if level != "" && !strings.EqualFold(entry.Level, level) {
			continue
		}

		if searchQuery != "" {
			if !strings.Contains(strings.ToLower(entry.Message), strings.ToLower(searchQuery)) &&
				!strings.Contains(strings.ToLower(entry.Raw), strings.ToLower(searchQuery)) {
				continue
			}
		}

		if startTime != nil && entry.Timestamp.Before(*startTime) {
			continue
		}

		if endTime != nil && entry.Timestamp.After(*endTime) {
			continue
		}

		entries = append(entries, *entry)
	}

	return entries, nil
}

// parseLogLine parses a single log line.
// Go's default log format: "2009/01/23 01:23:23 message"
// Our format: "2009/01/23 01:23:23 LEVEL: key=value key=value"
func (lr *LogReader) parseLogLine(line string) *LogEntry {
	// Go's default log format: "2009/01/23 01:23:23 message"
	// Regex to match: date time message
	// Date format: YYYY/MM/DD
	// Time format: HH:MM:SS
	logPattern := regexp.MustCompile(`^(\d{4}/\d{2}/\d{2})\s+(\d{2}:\d{2}:\d{2})\s+(.+)$`)
	matches := logPattern.FindStringSubmatch(line)
	
	if len(matches) != 4 {
		// Try to parse as simple message without timestamp
		return &LogEntry{
			Timestamp: time.Now(),
			Level:     "INFO",
			Message:   line,
			Raw:       line,
		}
	}

	dateStr := matches[1]
	timeStr := matches[2]
	message := matches[3]

	// Parse timestamp
	timestampStr := dateStr + " " + timeStr
	timestamp, err := time.Parse("2006/01/02 15:04:05", timestampStr)
	if err != nil {
		// If parsing fails, use current time
		timestamp = time.Now()
	}

	// Extract level from message (format: "LEVEL: message" or "LEVEL message")
	level := "INFO"
	fields := make(map[string]string)
	
	// Check for level prefix
	levelPattern := regexp.MustCompile(`^(INFO|ERROR|WARN|WARNING|DEBUG|FATAL):\s*(.+)$`)
	levelMatches := levelPattern.FindStringSubmatch(message)
	
	if len(levelMatches) == 3 {
		level = strings.ToUpper(levelMatches[1])
		if level == "WARNING" {
			level = "WARN"
		}
		message = levelMatches[2]
	}

	// Parse key=value pairs from message
	kvPattern := regexp.MustCompile(`(\w+)=([^\s]+)`)
	kvMatches := kvPattern.FindAllStringSubmatch(message, -1)
	
	for _, match := range kvMatches {
		if len(match) == 3 {
			fields[match[1]] = match[2]
		}
	}

	return &LogEntry{
		Timestamp: timestamp,
		Level:     level,
		Message:   message,
		Fields:    fields,
		Raw:       line,
	}
}
