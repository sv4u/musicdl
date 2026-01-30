package plan

import (
	"path/filepath"
)

// PlanFileNamePrefix is the prefix for plan files on disk (spec: download_plan_<hash>.json).
const PlanFileNamePrefix = "download_plan_"

// PlanFileNameSuffix is the suffix for plan files on disk.
const PlanFileNameSuffix = ".json"

// GetPlanFilePath returns the full path for a plan file given cache directory and config hash.
// Cache dir is typically .cache/ (or MUSICDL_CACHE_DIR); hash is 16-char hex.
// Example: GetPlanFilePath(".cache", "a1b2c3d4e5f67890") => ".cache/download_plan_a1b2c3d4e5f67890.json"
func GetPlanFilePath(cacheDir, configHash string) string {
	return filepath.Join(cacheDir, PlanFileNamePrefix+configHash+PlanFileNameSuffix)
}
