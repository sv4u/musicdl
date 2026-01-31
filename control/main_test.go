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

func TestDownloadCLICommand_NoPlanFileExits2(t *testing.T) {
	// Valid config but no plan file in cache -> exit 2 (plan not found)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(`version: "1.2"
download:
  client_id: "id"
  client_secret: "secret"
  output: "{title}.mp3"
`), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	// Use empty cache dir so no plan exists
	_ = os.Unsetenv("MUSICDL_CACHE_DIR")
	origWd, _ := os.Getwd()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer func() { _ = os.Chdir(origWd) }()
	// Ensure .cache is empty (no plan)
	_ = os.RemoveAll(".cache")

	code := downloadCLICommand(configPath, true)
	if code != DownloadExitPlanMissing {
		t.Errorf("downloadCLICommand(no plan) = %d, want %d (DownloadExitPlanMissing)", code, DownloadExitPlanMissing)
	}
}

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
