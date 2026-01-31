package main

import (
	"os"
	"strings"

	"golang.org/x/term"
)

// WantTUI returns true if the CLI should show the TUI: stdout is a terminal and --no-tui was not set.
func WantTUI(noTUIFlag bool) bool {
	if noTUIFlag {
		return false
	}
	if os.Getenv("MUSICDL_NO_TUI") != "" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// ParsePlanArgs parses args after "plan" and returns configPath and noTUI.
// Usage: musicdl plan [--no-tui] <config-file>
func ParsePlanArgs(args []string) (configPath string, noTUI bool) {
	for _, a := range args {
		if a == "--no-tui" {
			noTUI = true
			continue
		}
		if !strings.HasPrefix(a, "-") && configPath == "" {
			configPath = a
		}
	}
	return configPath, noTUI
}

// ParseDownloadArgs parses args after "download" and returns configPath and noTUI.
func ParseDownloadArgs(args []string) (configPath string, noTUI bool) {
	return ParsePlanArgs(args)
}
