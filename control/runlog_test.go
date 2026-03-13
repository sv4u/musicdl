package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLogTeeWriter_ErrorAndWarnDetection(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "test.log")
	errCh := make(chan string, 10)

	w, err := NewLogTeeWriter(logPath, errCh)
	if err != nil {
		t.Fatalf("NewLogTeeWriter: %v", err)
	}
	defer w.Close()

	input := "INFO: normal line\nERROR: something failed\nDEBUG: debug info\nWARN: careful\n"
	n, err := w.Write([]byte(input))
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if n != len(input) {
		t.Errorf("Write n = %d, want %d", n, len(input))
	}

	var lines []string
	timeout := time.After(100 * time.Millisecond)
	for {
		select {
		case line := <-errCh:
			lines = append(lines, line)
		case <-timeout:
			goto done
		}
	}
done:

	if len(lines) != 2 {
		t.Fatalf("expected 2 error/warn lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "ERROR: something failed" {
		t.Errorf("lines[0] = %q, want ERROR: something failed", lines[0])
	}
	if lines[1] != "WARN: careful" {
		t.Errorf("lines[1] = %q, want WARN: careful", lines[1])
	}
}

func TestLogTeeWriter_PartialLines(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "test.log")
	errCh := make(chan string, 10)

	w, err := NewLogTeeWriter(logPath, errCh)
	if err != nil {
		t.Fatalf("NewLogTeeWriter: %v", err)
	}
	defer w.Close()

	w.Write([]byte("ERROR: partial"))
	time.Sleep(10 * time.Millisecond)

	select {
	case line := <-errCh:
		t.Errorf("should not emit partial line, got %q", line)
	default:
	}

	w.Write([]byte(" message\n"))
	time.Sleep(10 * time.Millisecond)

	select {
	case line := <-errCh:
		if line != "ERROR: partial message" {
			t.Errorf("line = %q, want ERROR: partial message", line)
		}
	default:
		t.Error("expected completed line to be emitted")
	}
}

func TestLogTeeWriter_MultipleNewlinesInSingleWrite(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "test.log")
	errCh := make(chan string, 10)

	w, err := NewLogTeeWriter(logPath, errCh)
	if err != nil {
		t.Fatalf("NewLogTeeWriter: %v", err)
	}
	defer w.Close()

	w.Write([]byte("ERROR: first\nINFO: skip\nWARN: second\nERROR: third\n"))
	time.Sleep(10 * time.Millisecond)

	var lines []string
	for len(errCh) > 0 {
		lines = append(lines, <-errCh)
	}
	if len(lines) != 3 {
		t.Fatalf("expected 3 lines, got %d: %v", len(lines), lines)
	}
	if lines[0] != "ERROR: first" {
		t.Errorf("lines[0] = %q", lines[0])
	}
	if lines[1] != "WARN: second" {
		t.Errorf("lines[1] = %q", lines[1])
	}
	if lines[2] != "ERROR: third" {
		t.Errorf("lines[2] = %q", lines[2])
	}
}

func TestLogTeeWriter_NilErrorChannel(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "test.log")

	w, err := NewLogTeeWriter(logPath, nil)
	if err != nil {
		t.Fatalf("NewLogTeeWriter: %v", err)
	}
	defer w.Close()

	n, err := w.Write([]byte("ERROR: should not panic\n"))
	if err != nil {
		t.Fatalf("Write with nil channel should not error: %v", err)
	}
	if n == 0 {
		t.Error("Write should return non-zero bytes written")
	}

	data, _ := os.ReadFile(logPath)
	if string(data) != "ERROR: should not panic\n" {
		t.Errorf("log file content = %q", string(data))
	}
}

func TestLogTeeWriter_FileWrite(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "test.log")
	errCh := make(chan string, 10)

	w, err := NewLogTeeWriter(logPath, errCh)
	if err != nil {
		t.Fatalf("NewLogTeeWriter: %v", err)
	}

	w.Write([]byte("line 1\nline 2\n"))
	w.Close()

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "line 1\nline 2\n" {
		t.Errorf("log file = %q, want 'line 1\\nline 2\\n'", string(data))
	}
}
