package handlers

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogReader_ParseLogLine(t *testing.T) {
	reader := NewLogReader("")

	tests := []struct {
		name     string
		line     string
		wantLevel string
		wantMsg  string
	}{
		{
			name:     "INFO log with key-value pairs",
			line:     "2024/01/15 10:30:45 INFO: download_start spotify_id=abc123 track=Song",
			wantLevel: "INFO",
			wantMsg:  "download_start spotify_id=abc123 track=Song",
		},
		{
			name:     "ERROR log",
			line:     "2024/01/15 10:30:45 ERROR: download_failed spotify_url=test error=network",
			wantLevel: "ERROR",
			wantMsg:  "download_failed spotify_url=test error=network",
		},
		{
			name:     "WARN log",
			line:     "2024/01/15 10:30:45 WARN: metadata_embed_failed file=test.mp3",
			wantLevel: "WARN",
			wantMsg:  "metadata_embed_failed file=test.mp3",
		},
		{
			name:     "Simple message without level",
			line:     "2024/01/15 10:30:45 Simple message",
			wantLevel: "INFO",
			wantMsg:  "Simple message",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := reader.parseLogLine(tt.line)
			if entry == nil {
				t.Fatal("parseLogLine returned nil")
			}
			if entry.Level != tt.wantLevel {
				t.Errorf("Level = %v, want %v", entry.Level, tt.wantLevel)
			}
			if !strings.Contains(entry.Message, strings.Split(tt.wantMsg, " ")[0]) {
				t.Errorf("Message = %v, want to contain %v", entry.Message, tt.wantMsg)
			}
		})
	}
}

func TestLogReader_ReadLogs(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create test log file
	logContent := `2024/01/15 10:30:45 INFO: download_start spotify_id=abc123 track=Song
2024/01/15 10:30:46 ERROR: download_failed spotify_url=test error=network
2024/01/15 10:30:47 WARN: metadata_embed_failed file=test.mp3
2024/01/15 10:30:48 INFO: download_complete spotify_id=xyz789 track=Another
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	reader := NewLogReader(logPath)

	tests := []struct {
		name       string
		level      string
		search     string
		wantCount  int
	}{
		{
			name:      "Read all logs",
			level:     "",
			search:    "",
			wantCount: 4,
		},
		{
			name:      "Filter by level INFO",
			level:     "INFO",
			search:    "",
			wantCount: 2,
		},
		{
			name:      "Filter by level ERROR",
			level:     "ERROR",
			search:    "",
			wantCount: 1,
		},
		{
			name:      "Search by query",
			level:     "",
			search:    "download",
			wantCount: 3,
		},
		{
			name:      "Filter by level and search",
			level:     "INFO",
			search:    "download",
			wantCount: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := reader.ReadLogs(tt.level, tt.search, "", "", 1000)
			if err != nil {
				t.Fatalf("ReadLogs() error = %v", err)
			}
			if len(entries) != tt.wantCount {
				t.Errorf("ReadLogs() count = %v, want %v", len(entries), tt.wantCount)
			}
		})
	}
}

func TestLogReader_ReadLogs_TimeFilter(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Create test log file with specific timestamps
	logContent := `2024/01/15 10:30:45 INFO: message1
2024/01/15 10:30:46 INFO: message2
2024/01/15 10:30:47 INFO: message3
2024/01/15 10:30:48 INFO: message4
`
	if err := os.WriteFile(logPath, []byte(logContent), 0644); err != nil {
		t.Fatalf("Failed to create test log file: %v", err)
	}

	reader := NewLogReader(logPath)

	// Test time range filtering
	startTime := time.Date(2024, 1, 15, 10, 30, 46, 0, time.UTC).Format(time.RFC3339)
	endTime := time.Date(2024, 1, 15, 10, 30, 47, 0, time.UTC).Format(time.RFC3339)

	entries, err := reader.ReadLogs("", "", startTime, endTime, 1000)
	if err != nil {
		t.Fatalf("ReadLogs() error = %v", err)
	}
	
	// Should get message2 and message3 (within time range)
	if len(entries) != 2 {
		t.Errorf("ReadLogs() with time filter count = %v, want 2", len(entries))
	}
}

func TestLogReader_ReadLogs_NonExistentFile(t *testing.T) {
	reader := NewLogReader("/nonexistent/file.log")
	entries, err := reader.ReadLogs("", "", "", "", 1000)
	if err != nil {
		t.Fatalf("ReadLogs() should not error on non-existent file, got %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("ReadLogs() should return empty slice for non-existent file, got %v", len(entries))
	}
}
