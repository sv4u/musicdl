package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanCommand_InvalidConfigExits1(t *testing.T) {
	// Missing config file -> configuration error, exit 1
	code := planCommand(filepath.Join(t.TempDir(), "nonexistent.yaml"), true)
	if code != PlanExitConfigError {
		t.Errorf("planCommand(nonexistent config) = %d, want %d (PlanExitConfigError)", code, PlanExitConfigError)
	}
}

func TestPlanCommand_InvalidYAMLExits1(t *testing.T) {
	dir := t.TempDir()
	badConfig := filepath.Join(dir, "bad.yaml")
	if err := os.WriteFile(badConfig, []byte("invalid: yaml: ["), 0644); err != nil {
		t.Fatalf("write bad config: %v", err)
	}
	code := planCommand(badConfig, true)
	if code != PlanExitConfigError {
		t.Errorf("planCommand(invalid YAML) = %d, want %d (PlanExitConfigError)", code, PlanExitConfigError)
	}
}

// download command now always runs plan-then-download; no "plan not found" path for the user.

func TestPrintUsage(t *testing.T) {
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	printUsage()

	_ = w.Close()
	os.Stderr = oldStderr

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	expected := []string{
		"musicdl",
		"USAGE",
		"COMMANDS",
		"plan",
		"download",
		"version",
		"EXAMPLES",
	}

	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("printUsage() output should contain %q, got: %s", exp, output)
		}
	}
}
