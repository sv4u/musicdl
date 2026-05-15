package audio

import (
	"path/filepath"
	"testing"
)

func TestResolvedYtDlpCacheDir(t *testing.T) {
	t.Run("default relative cache under work dir", func(t *testing.T) {
		t.Setenv("MUSICDL_WORK_DIR", "/download")
		t.Setenv("MUSICDL_CACHE_DIR", ".cache")
		got := resolvedYtDlpCacheDir()
		want := filepath.Join("/download", ".cache", "yt-dlp")
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
	t.Run("absolute cache dir", func(t *testing.T) {
		t.Setenv("MUSICDL_WORK_DIR", "/download")
		t.Setenv("MUSICDL_CACHE_DIR", "/var/lib/foo")
		got := resolvedYtDlpCacheDir()
		want := filepath.Join("/var/lib/foo", "yt-dlp")
		if got != want {
			t.Fatalf("got %q want %q", got, want)
		}
	})
}
