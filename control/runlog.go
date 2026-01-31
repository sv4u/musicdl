package main

import (
	"bufio"
	"io"
	"log"
	"os"
	"strings"
	"sync"
)

// LogTeeWriter writes log output to a file and optionally sends ERROR/WARN lines to a channel for TUI display.
type LogTeeWriter struct {
	file   *os.File
	errors chan<- string
	mu     sync.Mutex
	buf    []byte
}

// NewLogTeeWriter creates a writer that writes to logPath and sends ERROR/WARN lines to errCh (if non-nil).
func NewLogTeeWriter(logPath string, errCh chan<- string) (*LogTeeWriter, error) {
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}
	return &LogTeeWriter{file: f, errors: errCh}, nil
}

// Write implements io.Writer. Lines containing "ERROR" or "WARN" are sent to the errors channel.
func (w *LogTeeWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err = w.file.Write(p)
	if err != nil || w.errors == nil {
		return n, err
	}
	w.buf = append(w.buf, p...)
	for {
		idx := 0
		for i, b := range w.buf {
			if b == '\n' {
				line := string(w.buf[idx : i+1])
				idx = i + 1
				if strings.Contains(line, "ERROR:") || strings.Contains(line, "WARN:") {
					select {
					case w.errors <- strings.TrimSuffix(line, "\n"):
					default:
					}
				}
			}
		}
		if idx > 0 {
			w.buf = w.buf[idx:]
		} else {
			break
		}
	}
	return n, nil
}

// Close closes the underlying file.
func (w *LogTeeWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

// RedirectLogToFile redirects the standard log output to the given writer and returns a restore func.
func RedirectLogToFile(w io.Writer) (restore func()) {
	oldFlags := log.Flags()
	oldPrefix := log.Prefix()
	oldOut := log.Writer()
	log.SetOutput(w)
	log.SetFlags(0)
	log.SetPrefix("")
	return func() {
		log.SetOutput(oldOut)
		log.SetFlags(oldFlags)
		log.SetPrefix(oldPrefix)
	}
}

// BufferLines reads from r line by line and sends lines containing ERROR or WARN to errCh.
func BufferLines(r io.Reader, errCh chan<- string) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "ERROR:") || strings.Contains(line, "WARN:") {
			select {
			case errCh <- line:
			default:
			}
		}
	}
}
