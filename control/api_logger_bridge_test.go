package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	spotigo "github.com/sv4u/spotigo/v2"
)

func TestSpotigoLogBridge_Info(t *testing.T) {
	lb := NewLogBroadcaster()
	bridge := &spotigoLogBridge{broadcaster: lb}

	bridge.Info("test message %s", "hello")

	history := lb.GetHistory()
	if len(history) == 0 {
		t.Fatal("expected at least one log message in history")
	}

	last := history[len(history)-1]
	if last.Level != "info" {
		t.Errorf("expected level 'info', got %q", last.Level)
	}
	if last.Message != "test message hello" {
		t.Errorf("expected message 'test message hello', got %q", last.Message)
	}
	if last.Source != "spotify" {
		t.Errorf("expected source 'spotify', got %q", last.Source)
	}
}

func TestSpotigoLogBridge_LogAPICall_Success(t *testing.T) {
	lb := NewLogBroadcaster()
	bridge := &spotigoLogBridge{broadcaster: lb}

	bridge.LogAPICall(spotigo.APICallInfo{
		Method:     "GET",
		URL:        "https://api.spotify.com/v1/tracks/abc",
		StatusCode: 200,
		Duration:   50 * time.Millisecond,
	})

	history := lb.GetHistory()
	if len(history) == 0 {
		t.Fatal("expected at least one log message in history")
	}

	last := history[len(history)-1]
	if last.Level != "debug" {
		t.Errorf("expected level 'debug', got %q", last.Level)
	}
	if !strings.Contains(last.Message, "200") {
		t.Errorf("expected message to contain '200', got %q", last.Message)
	}
}

func TestSpotigoLogBridge_LogAPICall_Error(t *testing.T) {
	lb := NewLogBroadcaster()
	bridge := &spotigoLogBridge{broadcaster: lb}

	bridge.LogAPICall(spotigo.APICallInfo{
		Method:   "GET",
		URL:      "https://api.spotify.com/v1/tracks/abc",
		Duration: 50 * time.Millisecond,
		Error:    fmt.Errorf("connection refused"),
	})

	history := lb.GetHistory()
	if len(history) == 0 {
		t.Fatal("expected at least one log message in history")
	}

	last := history[len(history)-1]
	if !strings.Contains(last.Message, "error") {
		t.Errorf("expected message to contain 'error', got %q", last.Message)
	}
}
