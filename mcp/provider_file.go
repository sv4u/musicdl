package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/sv4u/musicdl/download/config"
	"github.com/sv4u/musicdl/download/history"
	"github.com/sv4u/musicdl/download/plan"
)

// FileDataProvider reads all data from disk.
// It works in standalone mode without the API server running.
type FileDataProvider struct {
	workDir  string
	cacheDir string
	logDir   string

	// Optional runtime state for embedded mode
	runtime RuntimeDataProvider
}

// NewFileDataProvider creates a file-based data provider.
func NewFileDataProvider(workDir, cacheDir, logDir string) *FileDataProvider {
	return &FileDataProvider{
		workDir:  workDir,
		cacheDir: cacheDir,
		logDir:   logDir,
	}
}

// SetRuntime attaches a runtime provider for live data (embedded mode).
func (f *FileDataProvider) SetRuntime(rt RuntimeDataProvider) {
	f.runtime = rt
}

func (f *FileDataProvider) GetCurrentPlan() (*plan.DownloadPlan, string, error) {
	plans, err := f.ListPlanFiles()
	if err != nil {
		return nil, "", err
	}
	if len(plans) == 0 {
		return nil, "", fmt.Errorf("no plan files found in %s", f.cacheDir)
	}
	sort.Slice(plans, func(i, j int) bool {
		return plans[i].ModifiedAt.After(plans[j].ModifiedAt)
	})
	latest := plans[0]
	dp, err := plan.LoadPlan(latest.Path)
	if err != nil {
		return nil, latest.Hash, err
	}
	return dp, latest.Hash, nil
}

func (f *FileDataProvider) ListPlanFiles() ([]PlanFileSummary, error) {
	entries, err := os.ReadDir(f.cacheDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []PlanFileSummary
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "download_plan_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		hash := strings.TrimSuffix(strings.TrimPrefix(name, "download_plan_"), ".json")
		fullPath := filepath.Join(f.cacheDir, name)
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		summary := PlanFileSummary{
			Hash:       hash,
			Path:       fullPath,
			ModifiedAt: info.ModTime(),
			SizeBytes:  info.Size(),
		}
		if dp, loadErr := plan.LoadPlan(fullPath); loadErr == nil {
			for _, item := range dp.Items {
				if item.ItemType == plan.PlanItemTypeTrack {
					summary.TrackCount++
				}
			}
		}
		result = append(result, summary)
	}
	return result, nil
}

func (f *FileDataProvider) GetDownloadStatus() (*DownloadStatusInfo, error) {
	if f.runtime != nil {
		if status := f.runtime.GetLiveDownloadStatus(); status != nil {
			return status, nil
		}
	}
	return &DownloadStatusInfo{IsRunning: false, OperationType: "idle"}, nil
}

func (f *FileDataProvider) GetStats() (*StatsInfo, error) {
	statsPath := filepath.Join(f.cacheDir, "stats.json")
	data, err := os.ReadFile(statsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &StatsInfo{
				Cumulative: &CumulativeStatsInfo{},
				CurrentRun: &RunStatsInfo{},
			}, nil
		}
		return nil, err
	}
	var cumulative CumulativeStatsInfo
	if err := json.Unmarshal(data, &cumulative); err != nil {
		return nil, fmt.Errorf("failed to parse stats.json: %w", err)
	}
	return &StatsInfo{Cumulative: &cumulative, CurrentRun: &RunStatsInfo{}}, nil
}

func (f *FileDataProvider) SearchLogs(filter LogFilter) ([]LogEntryInfo, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	var logFiles []string
	if filter.RunDir != "" {
		dir := filepath.Join(f.logDir, filter.RunDir)
		logFiles = collectLogFiles(dir)
	} else {
		logFiles = f.findAllLogFiles()
	}
	var results []LogEntryInfo
	for _, lf := range logFiles {
		entries, err := parseLogFile(lf)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if filter.Level != "" && !strings.EqualFold(entry.Level, filter.Level) {
				continue
			}
			if filter.Keyword != "" && !strings.Contains(strings.ToLower(entry.Message), strings.ToLower(filter.Keyword)) {
				continue
			}
			results = append(results, entry)
			if len(results) >= limit {
				return results, nil
			}
		}
	}
	return results, nil
}

func (f *FileDataProvider) GetRecentLogs(count int) ([]LogEntryInfo, error) {
	if f.runtime != nil {
		if logs := f.runtime.GetLiveRecentLogs(); len(logs) > 0 {
			if count > 0 && count < len(logs) {
				return logs[len(logs)-count:], nil
			}
			return logs, nil
		}
	}
	if count <= 0 {
		count = 50
	}
	logFiles := f.findAllLogFiles()
	var all []LogEntryInfo
	for i := len(logFiles) - 1; i >= 0 && len(all) < count; i-- {
		entries, err := parseLogFile(logFiles[i])
		if err != nil {
			continue
		}
		all = append(entries, all...)
	}
	if len(all) > count {
		all = all[len(all)-count:]
	}
	return all, nil
}

func (f *FileDataProvider) ListRunLogDirs() ([]RunLogDir, error) {
	entries, err := os.ReadDir(f.logDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var dirs []RunLogDir
	for _, entry := range entries {
		if !entry.IsDir() || !strings.HasPrefix(entry.Name(), "run_") {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		dirs = append(dirs, RunLogDir{
			Name:      entry.Name(),
			Path:      filepath.Join(f.logDir, entry.Name()),
			CreatedAt: info.ModTime(),
		})
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].CreatedAt.After(dirs[j].CreatedAt)
	})
	return dirs, nil
}

func (f *FileDataProvider) GetConfigRaw() (string, error) {
	configPath := filepath.Join(f.workDir, "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (f *FileDataProvider) ListRuns(limit int) ([]RunSummaryInfo, error) {
	historyPath := filepath.Join(f.cacheDir, "history")
	entries, err := os.ReadDir(historyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	type runWithTime struct {
		summary RunSummaryInfo
		started time.Time
	}
	var runs []runWithTime
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, "run_") || !strings.HasSuffix(name, ".json") {
			continue
		}
		runID := name[4 : len(name)-5]
		fullPath := filepath.Join(historyPath, name)
		data, readErr := os.ReadFile(fullPath)
		if readErr != nil {
			continue
		}
		var rh history.RunHistory
		if jsonErr := rh.FromJSON(data); jsonErr != nil {
			continue
		}
		runs = append(runs, runWithTime{
			summary: RunSummaryInfo{
				RunID:       runID,
				StartedAt:   rh.StartedAt,
				CompletedAt: rh.CompletedAt,
				State:       rh.State,
				Statistics:  rh.Statistics,
				Error:       rh.Error,
			},
			started: rh.StartedAt,
		})
	}
	sort.Slice(runs, func(i, j int) bool {
		return runs[i].started.After(runs[j].started)
	})
	if limit > 0 && limit < len(runs) {
		runs = runs[:limit]
	}
	result := make([]RunSummaryInfo, len(runs))
	for i, r := range runs {
		result[i] = r.summary
	}
	return result, nil
}

func (f *FileDataProvider) GetRunDetails(runID string) (*history.RunHistory, error) {
	runPath := filepath.Join(f.cacheDir, "history", fmt.Sprintf("run_%s.json", runID))
	data, err := os.ReadFile(runPath)
	if err != nil {
		return nil, err
	}
	var rh history.RunHistory
	if err := rh.FromJSON(data); err != nil {
		return nil, err
	}
	return &rh, nil
}

func (f *FileDataProvider) GetActivity(limit int) ([]history.ActivityEntry, error) {
	activityPath := filepath.Join(f.cacheDir, "history", "activity.json")
	data, err := os.ReadFile(activityPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var ah history.ActivityHistory
	if err := ah.FromJSON(data); err != nil {
		return nil, err
	}
	entries := ah.Entries
	if limit > 0 && limit < len(entries) {
		entries = entries[len(entries)-limit:]
	}
	return entries, nil
}

func (f *FileDataProvider) GetHealth() (*HealthInfo, error) {
	info := &HealthInfo{
		Status:         "ok",
		MusicdlVersion: "unknown",
		GoVersion:      runtime.Version(),
		APIRunning:     f.runtime != nil,
	}
	if bi, ok := debug.ReadBuildInfo(); ok {
		info.MusicdlVersion = bi.Main.Version
		for _, dep := range bi.Deps {
			if dep.Path == "github.com/sv4u/spotigo/v2" {
				info.SpotigoVersion = dep.Version
				if dep.Replace != nil {
					info.SpotigoVersion = dep.Replace.Version
				}
			}
		}
	}
	return info, nil
}

func (f *FileDataProvider) GetRecoveryStatus() (*RecoveryInfo, error) {
	recovery := &RecoveryInfo{}

	resumePath := filepath.Join(f.cacheDir, "resume_state.json")
	if data, err := os.ReadFile(resumePath); err == nil {
		var raw struct {
			CompletedItems map[string]bool                 `json:"completedItems"`
			FailedItems    map[string]json.RawMessage      `json:"failedItems"`
			TotalItems     int                             `json:"totalItems"`
		}
		if json.Unmarshal(data, &raw) == nil {
			completed := len(raw.CompletedItems)
			failed := len(raw.FailedItems)
			recovery.Resume = ResumeInfo{
				HasResumeData:  completed > 0 || failed > 0,
				CompletedCount: completed,
				FailedCount:    failed,
				TotalItems:     raw.TotalItems,
				RemainingCount: raw.TotalItems - completed - failed,
			}
			for id, rawItem := range raw.FailedItems {
				var item struct {
					URL         string `json:"url"`
					Name        string `json:"name"`
					Error       string `json:"error"`
					Attempts    int    `json:"attempts"`
					LastAttempt int64  `json:"lastAttempt"`
					Retryable   bool   `json:"retryable"`
				}
				if json.Unmarshal(rawItem, &item) == nil {
					recovery.Resume.FailedItems = append(recovery.Resume.FailedItems, FailedItemEntry{
						ID:          id,
						URL:         item.URL,
						Name:        item.Name,
						Error:       item.Error,
						Attempts:    item.Attempts,
						LastAttempt: item.LastAttempt,
						Retryable:   item.Retryable,
					})
				}
			}
		}
	}

	recovery.CircuitBreaker = CircuitBreakerInfo{
		State:    "unknown",
		CanRetry: true,
	}
	return recovery, nil
}

func (f *FileDataProvider) GetCacheInfo() (*CacheInfo, error) {
	info := &CacheInfo{CacheDir: f.cacheDir}

	plans, _ := f.ListPlanFiles()
	info.PlanFiles = plans

	var totalSize int64
	_ = filepath.Walk(f.cacheDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		totalSize += fi.Size()
		return nil
	})
	info.TotalSize = totalSize

	if _, err := os.Stat(filepath.Join(f.cacheDir, "stats.json")); err == nil {
		info.StatsFile = filepath.Join(f.cacheDir, "stats.json")
	}
	if _, err := os.Stat(filepath.Join(f.cacheDir, "resume_state.json")); err == nil {
		info.ResumeFile = filepath.Join(f.cacheDir, "resume_state.json")
	}
	historyDir := filepath.Join(f.cacheDir, "history")
	if entries, err := os.ReadDir(historyDir); err == nil {
		info.HistoryDir = historyDir
		for _, e := range entries {
			if strings.HasPrefix(e.Name(), "run_") && strings.HasSuffix(e.Name(), ".json") {
				info.HistoryCount++
			}
		}
	}
	return info, nil
}

func (f *FileDataProvider) BrowseLibrary(subpath string) ([]LibraryEntry, error) {
	configPath := filepath.Join(f.workDir, "config.yaml")
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return f.browseDir(f.workDir, subpath)
	}
	baseDir := f.workDir
	outputTemplate := cfg.Download.Output
	if parts := strings.SplitN(outputTemplate, "/", 2); len(parts) > 1 && parts[0] != "" && !strings.Contains(parts[0], "{") {
		baseDir = filepath.Join(f.workDir, parts[0])
	}
	return f.browseDir(baseDir, subpath)
}

func (f *FileDataProvider) browseDir(baseDir, subpath string) ([]LibraryEntry, error) {
	dir := baseDir
	if subpath != "" {
		dir = filepath.Join(baseDir, subpath)
	}
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("invalid path")
	}
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return nil, fmt.Errorf("invalid base path")
	}
	if !strings.HasPrefix(absDir, absBase+string(filepath.Separator)) && absDir != absBase {
		return nil, fmt.Errorf("path outside library directory")
	}
	entries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}
	var result []LibraryEntry
	musicExts := map[string]bool{".mp3": true, ".flac": true, ".m4a": true, ".opus": true, ".ogg": true, ".wav": true}
	for _, entry := range entries {
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		relPath := entry.Name()
		if subpath != "" {
			relPath = filepath.Join(subpath, entry.Name())
		}
		le := LibraryEntry{
			Name:  entry.Name(),
			Path:  relPath,
			IsDir: entry.IsDir(),
		}
		if !entry.IsDir() {
			le.SizeBytes = info.Size()
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if musicExts[ext] {
				le.Format = strings.TrimPrefix(ext, ".")
			}
		}
		result = append(result, le)
	}
	return result, nil
}

func (f *FileDataProvider) SearchLibrary(query string) ([]LibraryEntry, error) {
	queryLower := strings.ToLower(query)
	musicExts := map[string]bool{".mp3": true, ".flac": true, ".m4a": true, ".opus": true, ".ogg": true, ".wav": true}
	var results []LibraryEntry
	_ = filepath.Walk(f.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if !musicExts[ext] {
			return nil
		}
		if strings.Contains(strings.ToLower(info.Name()), queryLower) ||
			strings.Contains(strings.ToLower(path), queryLower) {
			relPath, relErr := filepath.Rel(f.workDir, path)
			if relErr != nil {
				relPath = path
			}
			results = append(results, LibraryEntry{
				Name:      info.Name(),
				Path:      relPath,
				SizeBytes: info.Size(),
				Format:    strings.TrimPrefix(ext, "."),
			})
		}
		if len(results) >= 200 {
			return filepath.SkipAll
		}
		return nil
	})
	return results, nil
}

func (f *FileDataProvider) GetLibraryStats() (*LibraryStatsInfo, error) {
	stats := &LibraryStatsInfo{ByFormat: make(map[string]int)}
	musicExts := map[string]bool{".mp3": true, ".flac": true, ".m4a": true, ".opus": true, ".ogg": true, ".wav": true}
	artists := make(map[string]bool)
	albums := make(map[string]bool)

	_ = filepath.Walk(f.workDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(info.Name()))
		if !musicExts[ext] {
			return nil
		}
		stats.TotalFiles++
		stats.TotalSize += info.Size()
		stats.ByFormat[strings.TrimPrefix(ext, ".")]++

		rel, relErr := filepath.Rel(f.workDir, path)
		if relErr == nil {
			parts := strings.Split(rel, string(filepath.Separator))
			if len(parts) >= 2 {
				artists[parts[0]] = true
			}
			if len(parts) >= 3 {
				albums[parts[0]+"/"+parts[1]] = true
			}
		}
		return nil
	})

	stats.ArtistCount = len(artists)
	stats.AlbumCount = len(albums)
	return stats, nil
}

func (f *FileDataProvider) findAllLogFiles() []string {
	var files []string
	entries, err := os.ReadDir(f.logDir)
	if err != nil {
		return nil
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "run_") {
			files = append(files, collectLogFiles(filepath.Join(f.logDir, entry.Name()))...)
		}
	}
	return files
}

func collectLogFiles(dir string) []string {
	var files []string
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".log") {
			files = append(files, filepath.Join(dir, entry.Name()))
		}
	}
	return files
}

func parseLogFile(path string) ([]LogEntryInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	source := filepath.Base(filepath.Dir(path)) + "/" + filepath.Base(path)
	var entries []LogEntryInfo
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var entry LogEntryInfo
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			entry = LogEntryInfo{Message: line, Level: "INFO"}
		}
		entry.Source = source
		entries = append(entries, entry)
	}
	return entries, nil
}

func (f *FileDataProvider) GetPlexSyncStatus() (*PlexSyncStatusInfo, error) {
	if f.runtime != nil {
		if status := f.runtime.GetLivePlexSyncStatus(); status != nil {
			return status, nil
		}
	}
	return &PlexSyncStatusInfo{
		IsRunning: false,
		Results:   []PlexSyncResultInfo{},
	}, nil
}
