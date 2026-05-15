package audio

import (
	"os"
	"path/filepath"
)

// resolvedYtDlpCacheDir returns the directory used for yt-dlp's --cache-dir (SoundCloud client_id,
// YouTube tokens, etc.). It is placed under MUSICDL_WORK_DIR and MUSICDL_CACHE_DIR so containers
// without a writable HOME never try to use /.cache.
func resolvedYtDlpCacheDir() string {
	work := os.Getenv("MUSICDL_WORK_DIR")
	if work == "" {
		work = "."
	}
	cacheEnv := os.Getenv("MUSICDL_CACHE_DIR")
	if cacheEnv == "" {
		cacheEnv = ".cache"
	}
	var base string
	if filepath.IsAbs(cacheEnv) {
		base = filepath.Clean(cacheEnv)
	} else {
		base = filepath.Join(work, cacheEnv)
	}
	return filepath.Join(base, "yt-dlp")
}

// prependYtDlpWritableCache prepends --cache-dir after creating the directory.
func prependYtDlpWritableCache(args []string) ([]string, error) {
	dir := resolvedYtDlpCacheDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	out := make([]string, 0, 2+len(args))
	out = append(out, "--cache-dir", dir)
	out = append(out, args...)
	return out, nil
}
