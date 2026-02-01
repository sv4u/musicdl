package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// getLogDir returns MUSICDL_LOG_DIR or ".logs" under current dir.
func getLogDir() string {
	if d := os.Getenv("MUSICDL_LOG_DIR"); d != "" {
		return d
	}
	return ".logs"
}

// runDirKind is the type of run (plan or download).
type runDirKind string

const (
	RunDirPlan     runDirKind = "plan"
	RunDirDownload runDirKind = "download"
)

// CreateRunDir creates a per-run directory under the log dir (.logs/run_<timestamp>_<nanos>/)
// and returns the run directory path and the path to the log file (plan.log or download.log).
// Nanosecond suffix avoids collision when multiple runs start in the same second.
func CreateRunDir(kind runDirKind) (runDir, logPath string, err error) {
	base := getLogDir()
	if err := os.MkdirAll(base, 0755); err != nil {
		return "", "", fmt.Errorf("create log base dir: %w", err)
	}
	now := time.Now()
	ts := strings.ReplaceAll(now.Format(time.RFC3339), ":", "-")
	runDir = filepath.Join(base, "run_"+ts+"_"+strconv.FormatInt(now.UnixNano(), 10))
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return "", "", fmt.Errorf("create run dir: %w", err)
	}
	logName := string(kind) + ".log"
	logPath = filepath.Join(runDir, logName)
	return runDir, logPath, nil
}
