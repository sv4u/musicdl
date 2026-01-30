package plan

import (
	"path/filepath"
	"testing"
)

func TestGetPlanFilePath(t *testing.T) {
	tests := []struct {
		cacheDir   string
		configHash string
		wantSuffix string
	}{
		{".cache", "a1b2c3d4e5f67890", "download_plan_a1b2c3d4e5f67890.json"},
		{"/download/.cache", "abc123", "download_plan_abc123.json"},
		{"", "hash", "download_plan_hash.json"},
	}
	for _, tt := range tests {
		got := GetPlanFilePath(tt.cacheDir, tt.configHash)
		base := filepath.Base(got)
		if base != tt.wantSuffix {
			t.Errorf("GetPlanFilePath(%q, %q) => base %q, want %q", tt.cacheDir, tt.configHash, base, tt.wantSuffix)
		}
		dir := filepath.Dir(got)
		if tt.cacheDir != "" && dir != tt.cacheDir {
			t.Errorf("GetPlanFilePath(%q, %q) => dir %q, want %q", tt.cacheDir, tt.configHash, dir, tt.cacheDir)
		}
	}
}
